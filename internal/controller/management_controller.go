// Copyright 2024
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package controller

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	fluxv2 "github.com/fluxcd/helm-controller/api/v2"
	"github.com/fluxcd/pkg/apis/meta"
	sourcev1 "github.com/fluxcd/source-controller/api/v1"
	"helm.sh/helm/v3/pkg/chartutil"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	hmc "github.com/Mirantis/hmc/api/v1alpha1"
	"github.com/Mirantis/hmc/internal/certmanager"
	"github.com/Mirantis/hmc/internal/helm"
	"github.com/Mirantis/hmc/internal/utils"
	"github.com/Mirantis/hmc/internal/utils/status"
)

// Those are only needed for the initial installation
var enforcedValues = map[string]any{
	"controller": map[string]any{
		"createManagement":         false,
		"createTemplateManagement": false,
		"createRelease":            false,
	},
}

// ManagementReconciler reconciles a Management object
type ManagementReconciler struct {
	client.Client
	Scheme                   *runtime.Scheme
	Config                   *rest.Config
	SystemNamespace          string
	DynamicClient            *dynamic.DynamicClient
	CreateTemplateManagement bool
}

func (r *ManagementReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	l := ctrl.LoggerFrom(ctx)
	l.Info("Reconciling Management")

	management := &hmc.Management{}
	if err := r.Get(ctx, req.NamespacedName, management); err != nil {
		if apierrors.IsNotFound(err) {
			l.Info("Management not found, ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}

		l.Error(err, "Failed to get Management")
		return ctrl.Result{}, err
	}

	if !management.DeletionTimestamp.IsZero() {
		l.Info("Deleting Management")
		return r.Delete(ctx, management)
	}

	return r.Update(ctx, management)
}

func (r *ManagementReconciler) Update(ctx context.Context, management *hmc.Management) (ctrl.Result, error) {
	l := ctrl.LoggerFrom(ctx)

	finalizersUpdated := controllerutil.AddFinalizer(management, hmc.ManagementFinalizer)
	if finalizersUpdated {
		if err := r.Client.Update(ctx, management); err != nil {
			l.Error(err, "Failed to update Management finalizers")
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	if err := r.ensureTemplateManagement(ctx, management); err != nil {
		l.Error(err, "Failed to ensure TemplateManagement is created")
		return ctrl.Result{}, err
	}

	release := &hmc.Release{}
	if err := r.Client.Get(ctx, client.ObjectKey{Name: management.Spec.Release}, release); err != nil {
		l.Error(err, "failed to get Release object")
		return ctrl.Result{}, err
	}

	var errs error
	detectedProviders := hmc.ProvidersTupled{}
	detectedComponents := make(map[string]hmc.ComponentStatus)

	err := r.enableAdditionalComponents(ctx, management)
	if err != nil {
		l.Error(err, "failed to enable additional HMC components")
		return ctrl.Result{}, err
	}

	components, err := wrappedComponents(management, release)
	if err != nil {
		l.Error(err, "failed to wrap HMC components")
		return ctrl.Result{}, err
	}
	for _, component := range components {
		template := &hmc.ProviderTemplate{}
		err := r.Get(ctx, client.ObjectKey{
			Name: component.Template,
		}, template)
		if err != nil {
			errMsg := fmt.Sprintf("Failed to get ProviderTemplate %s: %s", component.Template, err)
			updateComponentsStatus(detectedComponents, &detectedProviders, component.helmReleaseName, component.Template, template.Status.Providers, errMsg)
			errs = errors.Join(errs, errors.New(errMsg))
			continue
		}
		if !template.Status.Valid {
			errMsg := fmt.Sprintf("Template %s is not marked as valid", component.Template)
			updateComponentsStatus(detectedComponents, &detectedProviders, component.helmReleaseName, component.Template, template.Status.Providers, errMsg)
			errs = errors.Join(errs, errors.New(errMsg))
			continue
		}

		_, _, err = helm.ReconcileHelmRelease(ctx, r.Client, component.helmReleaseName, r.SystemNamespace, helm.ReconcileHelmReleaseOpts{
			Values:          component.Config,
			ChartRef:        template.Status.ChartRef,
			DependsOn:       component.dependsOn,
			TargetNamespace: component.targetNamespace,
			CreateNamespace: component.createNamespace,
		})
		if err != nil {
			errMsg := fmt.Sprintf("error reconciling HelmRelease %s/%s: %s", r.SystemNamespace, component.Template, err)
			updateComponentsStatus(detectedComponents, &detectedProviders, component.helmReleaseName, component.Template, template.Status.Providers, errMsg)
			errs = errors.Join(errs, errors.New(errMsg))
			continue
		}

		if component.Template != hmc.CoreHMCName {
			if err := r.checkProviderStatus(ctx, component.Template); err != nil {
				updateComponentsStatus(detectedComponents, &detectedProviders, component.helmReleaseName, component.Template, template.Status.Providers, err.Error())
				errs = errors.Join(errs, err)
				continue
			}
		}

		updateComponentsStatus(detectedComponents, &detectedProviders, component.helmReleaseName, component.Template, template.Status.Providers, "")
	}

	management.Status.ObservedGeneration = management.Generation
	management.Status.AvailableProviders = detectedProviders
	management.Status.Components = detectedComponents
	management.Status.Release = management.Spec.Release
	if err := r.Status().Update(ctx, management); err != nil {
		errs = errors.Join(errs, fmt.Errorf("failed to update status for Management %s: %w",
			management.Name, err))
	}
	if errs != nil {
		l.Error(errs, "Multiple errors during Management reconciliation")
		return ctrl.Result{}, errs
	}
	return ctrl.Result{}, nil
}

func (r *ManagementReconciler) ensureTemplateManagement(ctx context.Context, mgmt *hmc.Management) error {
	l := ctrl.LoggerFrom(ctx)
	if !r.CreateTemplateManagement {
		return nil
	}
	l.Info("Ensuring TemplateManagement is created")
	tmObj := &hmc.TemplateManagement{
		ObjectMeta: metav1.ObjectMeta{
			Name: hmc.TemplateManagementName,
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: hmc.GroupVersion.String(),
					Kind:       mgmt.Kind,
					Name:       mgmt.Name,
					UID:        mgmt.UID,
				},
			},
		},
	}
	err := r.Get(ctx, client.ObjectKey{
		Name: hmc.TemplateManagementName,
	}, tmObj)
	if err == nil {
		return nil
	}
	if !apierrors.IsNotFound(err) {
		return fmt.Errorf("failed to get %s TemplateManagement object: %w", hmc.TemplateManagementName, err)
	}
	err = r.Create(ctx, tmObj)
	if err != nil {
		return fmt.Errorf("failed to create %s TemplateManagement object: %w", hmc.TemplateManagementName, err)
	}
	l.Info("Successfully created TemplateManagement object")

	return nil
}

// checkProviderStatus checks the status of a provider associated with a given
// ProviderTemplate name.  Since there's no way to determine resource Kind from
// the given template iterate over all possible provider types.
func (r *ManagementReconciler) checkProviderStatus(ctx context.Context, providerTemplateName string) error {
	var errs error

	for _, resourceType := range []string{
		"coreproviders",
		"infrastructureproviders",
		"controlplaneproviders",
		"bootstrapproviders",
	} {
		gvr := schema.GroupVersionResource{
			Group:    "operator.cluster.x-k8s.io",
			Version:  "v1alpha2",
			Resource: resourceType,
		}

		resourceConditions, err := status.GetResourceConditions(ctx, r.SystemNamespace, r.DynamicClient, gvr,
			labels.SelectorFromSet(map[string]string{hmc.FluxHelmChartNameKey: providerTemplateName}).String(),
		)
		if err != nil {
			notFoundErr := status.ResourceNotFoundError{}
			if errors.As(err, &notFoundErr) {
				// Check the next resource type.
				continue
			}

			return fmt.Errorf("failed to get status for template: %s: %w", providerTemplateName, err)
		}

		var falseConditionTypes []string
		for _, condition := range resourceConditions.Conditions {
			if condition.Status != metav1.ConditionTrue {
				falseConditionTypes = append(falseConditionTypes, condition.Type)
			}
		}

		if len(falseConditionTypes) > 0 {
			errs = errors.Join(fmt.Errorf("%s: %s is not yet ready, has false conditions: %s",
				resourceConditions.Name, resourceConditions.Kind, strings.Join(falseConditionTypes, ", ")))
		}
	}

	if errs != nil {
		return errs
	}

	return nil
}

func (r *ManagementReconciler) Delete(ctx context.Context, management *hmc.Management) (ctrl.Result, error) {
	l := ctrl.LoggerFrom(ctx)
	listOpts := &client.ListOptions{
		LabelSelector: labels.SelectorFromSet(map[string]string{hmc.HMCManagedLabelKey: hmc.HMCManagedLabelValue}),
	}
	if err := r.removeHelmReleases(ctx, hmc.CoreHMCName, listOpts); err != nil {
		return ctrl.Result{}, err
	}
	if err := r.removeHelmCharts(ctx, listOpts); err != nil {
		return ctrl.Result{}, err
	}
	if err := r.removeHelmRepositories(ctx, listOpts); err != nil {
		return ctrl.Result{}, err
	}

	// Removing finalizer in the end of cleanup
	l.Info("Removing Management finalizer")
	if controllerutil.RemoveFinalizer(management, hmc.ManagementFinalizer) {
		return ctrl.Result{}, r.Client.Update(ctx, management)
	}
	return ctrl.Result{}, nil
}

func (r *ManagementReconciler) removeHelmReleases(ctx context.Context, hmcReleaseName string, opts *client.ListOptions) error {
	l := ctrl.LoggerFrom(ctx)
	l.Info("Suspending HMC Helm Release reconciles")
	hmcRelease := &fluxv2.HelmRelease{}
	err := r.Client.Get(ctx, client.ObjectKey{Namespace: r.SystemNamespace, Name: hmcReleaseName}, hmcRelease)
	if err != nil && !apierrors.IsNotFound(err) {
		return err
	}
	if err == nil && !hmcRelease.Spec.Suspend {
		hmcRelease.Spec.Suspend = true
		if err := r.Client.Update(ctx, hmcRelease); err != nil {
			return err
		}
	}
	l.Info("Ensuring all HelmReleases owned by HMC are removed")
	gvk := fluxv2.GroupVersion.WithKind(fluxv2.HelmReleaseKind)
	if err := utils.EnsureDeleteAllOf(ctx, r.Client, gvk, opts); err != nil {
		l.Error(err, "Not all HelmReleases owned by HMC are removed")
		return err
	}
	return nil
}

func (r *ManagementReconciler) removeHelmCharts(ctx context.Context, opts *client.ListOptions) error {
	l := ctrl.LoggerFrom(ctx)
	l.Info("Ensuring all HelmCharts owned by HMC are removed")
	gvk := sourcev1.GroupVersion.WithKind(sourcev1.HelmChartKind)
	if err := utils.EnsureDeleteAllOf(ctx, r.Client, gvk, opts); err != nil {
		l.Error(err, "Not all HelmCharts owned by HMC are removed")
		return err
	}
	return nil
}

func (r *ManagementReconciler) removeHelmRepositories(ctx context.Context, opts *client.ListOptions) error {
	l := ctrl.LoggerFrom(ctx)
	l.Info("Ensuring all HelmRepositories owned by HMC are removed")
	gvk := sourcev1.GroupVersion.WithKind(sourcev1.HelmRepositoryKind)
	if err := utils.EnsureDeleteAllOf(ctx, r.Client, gvk, opts); err != nil {
		l.Error(err, "Not all HelmRepositories owned by HMC are removed")
		return err
	}
	return nil
}

type component struct {
	hmc.Component

	helmReleaseName string
	targetNamespace string
	// helm release dependencies
	dependsOn       []meta.NamespacedObjectReference
	createNamespace bool
}

func applyHMCDefaults(config *apiextensionsv1.JSON) (*apiextensionsv1.JSON, error) {
	values := chartutil.Values{}
	if config != nil && config.Raw != nil {
		err := json.Unmarshal(config.Raw, &values)
		if err != nil {
			return nil, err
		}
	}
	chartutil.CoalesceTables(values, enforcedValues)
	raw, err := json.Marshal(values)
	if err != nil {
		return nil, err
	}
	return &apiextensionsv1.JSON{Raw: raw}, nil
}

func wrappedComponents(mgmt *hmc.Management, release *hmc.Release) ([]component, error) {
	if mgmt.Spec.Core == nil {
		return nil, nil
	}
	components := make([]component, 0, len(mgmt.Spec.Providers)+2)
	hmcComp := component{Component: mgmt.Spec.Core.HMC, helmReleaseName: hmc.CoreHMCName}
	if hmcComp.Template == "" {
		hmcComp.Template = release.Spec.HMC.Template
	}
	hmcConfig, err := applyHMCDefaults(hmcComp.Config)
	if err != nil {
		return nil, err
	}
	hmcComp.Config = hmcConfig
	components = append(components, hmcComp)

	capiComp := component{
		Component: mgmt.Spec.Core.CAPI, helmReleaseName: hmc.CoreCAPIName,
		dependsOn: []meta.NamespacedObjectReference{{Name: hmc.CoreHMCName}},
	}
	if capiComp.Template == "" {
		capiComp.Template = release.Spec.CAPI.Template
	}
	components = append(components, capiComp)

	for _, p := range mgmt.Spec.Providers {
		c := component{
			Component: p.Component, helmReleaseName: p.Name,
			dependsOn: []meta.NamespacedObjectReference{{Name: hmc.CoreCAPIName}},
		}
		// Try to find corresponding provider in the Release object
		if c.Template == "" {
			c.Template = release.ProviderTemplate(p.Name)
		}

		if p.Name == hmc.ProviderSveltosName {
			c.targetNamespace = hmc.ProviderSveltosTargetNamespace
			c.createNamespace = hmc.ProviderSveltosCreateNamespace
		}

		components = append(components, c)
	}

	return components, nil
}

// enableAdditionalComponents enables the admission controller and cluster api operator
// once the cert manager is ready
func (r *ManagementReconciler) enableAdditionalComponents(ctx context.Context, mgmt *hmc.Management) error {
	l := ctrl.LoggerFrom(ctx)

	hmcComponent := &mgmt.Spec.Core.HMC
	config := make(map[string]any)

	if hmcComponent.Config != nil {
		err := json.Unmarshal(hmcComponent.Config.Raw, &config)
		if err != nil {
			return fmt.Errorf("failed to unmarshal HMC config into map[string]any: %v", err)
		}
	}

	admissionWebhookValues := make(map[string]any)
	if config["admissionWebhook"] != nil {
		v, ok := config["admissionWebhook"].(map[string]any)
		if !ok {
			return fmt.Errorf("failed to cast 'admissionWebhook' (type %T) to map[string]any", config["admissionWebhook"])
		}

		admissionWebhookValues = v
	}

	capiOperatorValues := make(map[string]any)
	if config["cluster-api-operator"] != nil {
		v, ok := config["cluster-api-operator"].(map[string]any)
		if !ok {
			return fmt.Errorf("failed to cast 'cluster-api-operator' (type %T) to map[string]any", config["cluster-api-operator"])
		}

		capiOperatorValues = v
	}

	err := certmanager.VerifyAPI(ctx, r.Config, r.Scheme, r.SystemNamespace)
	if err != nil {
		return fmt.Errorf("failed to check in the cert-manager API is installed: %v", err)
	}
	l.Info("Cert manager is installed, enabling the HMC admission webhook")

	admissionWebhookValues["enabled"] = true
	config["admissionWebhook"] = admissionWebhookValues

	// Enable HMC capi operator only if it was not explicitly disabled in the config to
	// support installation with existing cluster api operator
	{
		enabledV, enabledExists := capiOperatorValues["enabled"]
		enabledValue, castedOk := enabledV.(bool)
		if !enabledExists || !castedOk || enabledValue {
			l.Info("Enabling cluster API operator")
			capiOperatorValues["enabled"] = true
		}
	}
	config["cluster-api-operator"] = capiOperatorValues

	updatedConfig, err := json.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal HMC config: %v", err)
	}
	hmcComponent.Config = &apiextensionsv1.JSON{
		Raw: updatedConfig,
	}
	return nil
}

func updateComponentsStatus(
	components map[string]hmc.ComponentStatus,
	providers *hmc.ProvidersTupled,
	componentName string,
	templateName string,
	templateProviders hmc.ProvidersTupled,
	err string,
) {
	components[componentName] = hmc.ComponentStatus{
		Error:    err,
		Success:  err == "",
		Template: templateName,
	}

	if err == "" {
		providers.InfrastructureProviders = append(providers.InfrastructureProviders, templateProviders.InfrastructureProviders...)
		providers.BootstrapProviders = append(providers.BootstrapProviders, templateProviders.BootstrapProviders...)
		providers.ControlPlaneProviders = append(providers.ControlPlaneProviders, templateProviders.ControlPlaneProviders...)
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *ManagementReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&hmc.Management{}).
		Complete(r)
}
