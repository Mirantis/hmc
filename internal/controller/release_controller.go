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

	hcv2 "github.com/fluxcd/helm-controller/api/v2"
	fluxmeta "github.com/fluxcd/pkg/apis/meta"
	fluxconditions "github.com/fluxcd/pkg/runtime/conditions"
	sourcev1 "github.com/fluxcd/source-controller/api/v1"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chartutil"
	"helm.sh/helm/v3/pkg/storage/driver"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/source"

	hmc "github.com/Mirantis/hmc/api/v1alpha1"
	"github.com/Mirantis/hmc/internal/build"
	"github.com/Mirantis/hmc/internal/helm"
	"github.com/Mirantis/hmc/internal/providers"
	"github.com/Mirantis/hmc/internal/utils"
)

// ReleaseReconciler reconciles a Template object
type ReleaseReconciler struct {
	client.Client

	Config *rest.Config

	HMCTemplatesChartName string
	SystemNamespace       string

	DefaultRegistryConfig helm.DefaultRegistryConfig

	CreateManagement bool
	CreateRelease    bool
	CreateTemplates  bool
}

func (r *ReleaseReconciler) Reconcile(ctx context.Context, req ctrl.Request) (result ctrl.Result, err error) {
	l := ctrl.LoggerFrom(ctx).WithValues("controller", "ReleaseController")
	l.Info("Reconciling Release")
	defer l.Info("Release reconcile is finished")

	release := &hmc.Release{}
	if req.Name != "" {
		if err := r.Client.Get(ctx, req.NamespacedName, release); err != nil {
			l.Error(err, "failed to get Release")
			return ctrl.Result{}, err
		}
		defer func() {
			release.Status.ObservedGeneration = release.Generation
			for _, condition := range release.Status.Conditions {
				if condition.Status != metav1.ConditionTrue {
					release.Status.Ready = false
				}
			}
			err = errors.Join(err, r.Status().Update(ctx, release))
		}()
	}

	err = r.reconcileHMCTemplates(ctx, release.Name, release.Spec.Version, release.UID)
	r.updateTemplatesCreatedCondition(release, err)
	if err != nil {
		l.Error(err, "failed to reconcile HMC Templates")
		return ctrl.Result{}, err
	}

	if release.Name == "" {
		if err := r.ensureManagement(ctx); err != nil {
			l.Error(err, "failed to create Management object")
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}
	err = r.validateProviderTemplates(ctx, release.Name, release.Templates())
	updateTemplatesValidCondition(release, err)
	if err != nil {
		l.Error(err, "failed to validate provider templates")
		return ctrl.Result{}, err
	}
	release.Status.Ready = true
	return ctrl.Result{}, nil
}

func (r *ReleaseReconciler) validateProviderTemplates(ctx context.Context, releaseName string, expectedTemplates []string) error {
	providerTemplates := &hmc.ProviderTemplateList{}
	if err := r.List(ctx, providerTemplates, client.MatchingFields{hmc.OwnerRefIndexKey: releaseName}); err != nil {
		return err
	}
	validTemplates := make(map[string]bool)
	for _, t := range providerTemplates.Items {
		validTemplates[t.Name] = t.Status.ObservedGeneration == t.Generation && t.Status.Valid
	}
	invalidTemplates := []string{}
	for _, t := range expectedTemplates {
		if !validTemplates[t] {
			invalidTemplates = append(invalidTemplates, t)
		}
	}
	if len(invalidTemplates) > 0 {
		return fmt.Errorf("missing or invalid templates: %s", strings.Join(invalidTemplates, ", "))
	}
	return nil
}

func updateTemplatesValidCondition(release *hmc.Release, err error) {
	condition := metav1.Condition{
		Type:               hmc.TemplatesValidCondition,
		Status:             metav1.ConditionTrue,
		ObservedGeneration: release.Generation,
		Reason:             hmc.SucceededReason,
		Message:            "All templates are valid",
	}
	if err != nil {
		condition.Status = metav1.ConditionFalse
		condition.Message = err.Error()
		condition.Reason = hmc.FailedReason
		release.Status.Ready = false
	}
	meta.SetStatusCondition(&release.Status.Conditions, condition)
}

func (r *ReleaseReconciler) updateTemplatesCreatedCondition(release *hmc.Release, err error) {
	condition := metav1.Condition{
		Type:               hmc.TemplatesCreatedCondition,
		Status:             metav1.ConditionTrue,
		ObservedGeneration: release.Generation,
		Reason:             hmc.SucceededReason,
		Message:            "All templates have been created",
	}
	if !r.CreateTemplates {
		condition.Message = "Templates creation is disabled"
	}
	if err != nil {
		condition.Status = metav1.ConditionFalse
		condition.Message = err.Error()
		condition.Reason = hmc.FailedReason
	}
	meta.SetStatusCondition(&release.Status.Conditions, condition)
}

func (r *ReleaseReconciler) ensureManagement(ctx context.Context) error {
	l := ctrl.LoggerFrom(ctx)
	if !r.CreateManagement {
		return nil
	}
	l.Info("Ensuring Management is created")
	mgmtObj := &hmc.Management{
		ObjectMeta: metav1.ObjectMeta{
			Name:       hmc.ManagementName,
			Finalizers: []string{hmc.ManagementFinalizer},
		},
	}
	err := r.Get(ctx, client.ObjectKey{
		Name: hmc.ManagementName,
	}, mgmtObj)
	if err == nil {
		return nil
	}
	if !apierrors.IsNotFound(err) {
		return fmt.Errorf("failed to get %s Management object: %w", hmc.AccessManagementName, err)
	}
	mgmtObj.Spec.Release, err = r.getCurrentReleaseName(ctx)
	if err != nil {
		return err
	}
	mgmtObj.Spec.Providers = providers.List()

	getter := helm.NewMemoryRESTClientGetter(r.Config, r.RESTMapper())
	actionConfig := new(action.Configuration)
	err = actionConfig.Init(getter, r.SystemNamespace, "secret", l.Info)
	if err != nil {
		return err
	}

	hmcConfig := make(chartutil.Values)
	release, err := actionConfig.Releases.Last("hmc")
	if err != nil {
		if !errors.Is(err, driver.ErrReleaseNotFound) {
			return err
		}
	} else {
		if len(release.Config) > 0 {
			chartutil.CoalesceTables(hmcConfig, release.Config)
		}
	}
	rawConfig, err := json.Marshal(hmcConfig)
	if err != nil {
		return err
	}
	mgmtObj.Spec.Core = &hmc.Core{
		HMC: hmc.Component{
			Config: &apiextensionsv1.JSON{
				Raw: rawConfig,
			},
		},
	}
	err = r.Create(ctx, mgmtObj)
	if err != nil {
		return fmt.Errorf("failed to create %s Management object: %w", hmc.AccessManagementName, err)
	}

	l.Info("Successfully created Management object with default configuration")
	return nil
}

func (r *ReleaseReconciler) reconcileHMCTemplates(ctx context.Context, releaseName, releaseVersion string, releaseUID types.UID) error {
	l := ctrl.LoggerFrom(ctx)
	if !r.CreateTemplates {
		l.Info("Templates creation is disabled")
		return nil
	}
	if releaseName == "" && !r.CreateRelease {
		l.Info("Initial creation of HMC Release is skipped")
		return nil
	}
	initialInstall := releaseName == ""
	var ownerRefs []metav1.OwnerReference
	if releaseName == "" {
		releaseName = utils.ReleaseNameFromVersion(build.Version)
		releaseVersion = build.Version
		err := helm.ReconcileHelmRepository(ctx, r.Client, hmc.DefaultRepoName, r.SystemNamespace, r.DefaultRegistryConfig.HelmRepositorySpec())
		if err != nil {
			l.Error(err, "Failed to reconcile default HelmRepository", "namespace", r.SystemNamespace)
			return err
		}
	} else {
		ownerRefs = []metav1.OwnerReference{
			{
				APIVersion: hmc.GroupVersion.String(),
				Kind:       hmc.ReleaseKind,
				Name:       releaseName,
				UID:        releaseUID,
			},
		}
	}

	hmcTemplatesName := utils.TemplatesChartFromReleaseName(releaseName)
	helmChart := &sourcev1.HelmChart{
		ObjectMeta: metav1.ObjectMeta{
			Name:      hmcTemplatesName,
			Namespace: r.SystemNamespace,
		},
	}

	operation, err := ctrl.CreateOrUpdate(ctx, r.Client, helmChart, func() error {
		helmChart.OwnerReferences = ownerRefs
		if helmChart.Labels == nil {
			helmChart.Labels = make(map[string]string)
		}
		helmChart.Labels[hmc.HMCManagedLabelKey] = hmc.HMCManagedLabelValue
		helmChart.Spec = sourcev1.HelmChartSpec{
			Chart:     r.HMCTemplatesChartName,
			Version:   releaseVersion,
			SourceRef: hmc.DefaultSourceRef,
			Interval:  metav1.Duration{Duration: helm.DefaultReconcileInterval},
		}
		return nil
	})
	if err != nil {
		return err
	}
	if operation == controllerutil.OperationResultCreated || operation == controllerutil.OperationResultUpdated {
		l.Info(fmt.Sprintf("Successfully %s %s/%s HelmChart", operation, r.SystemNamespace, hmcTemplatesName))
	}

	opts := helm.ReconcileHelmReleaseOpts{
		ChartRef: &hcv2.CrossNamespaceSourceReference{
			Kind:      helmChart.Kind,
			Name:      helmChart.Name,
			Namespace: helmChart.Namespace,
		},
	}

	if initialInstall {
		createReleaseValues := map[string]any{
			"createRelease": true,
		}
		raw, err := json.Marshal(createReleaseValues)
		if err != nil {
			return err
		}
		opts.Values = &apiextensionsv1.JSON{Raw: raw}
	}

	hr, operation, err := helm.ReconcileHelmRelease(ctx, r.Client, hmcTemplatesName, r.SystemNamespace, opts)
	if err != nil {
		return err
	}
	if operation == controllerutil.OperationResultCreated || operation == controllerutil.OperationResultUpdated {
		l.Info(fmt.Sprintf("Successfully %s %s/%s HelmRelease", operation, r.SystemNamespace, hmcTemplatesName))
	}
	hrReadyCondition := fluxconditions.Get(hr, fluxmeta.ReadyCondition)
	if hrReadyCondition == nil || hrReadyCondition.ObservedGeneration != hr.Generation {
		return fmt.Errorf("HelmRelease %s/%s is not ready yet. Waiting for reconciliation", r.SystemNamespace, hmcTemplatesName)
	}
	if hrReadyCondition.Status == metav1.ConditionFalse {
		return fmt.Errorf("HelmRelease %s/%s is not ready yet. %s", r.SystemNamespace, hmcTemplatesName, hrReadyCondition.Message)
	}
	return nil
}

func (r *ReleaseReconciler) getCurrentReleaseName(ctx context.Context) (string, error) {
	releases := &hmc.ReleaseList{}
	listOptions := client.ListOptions{
		FieldSelector: fields.SelectorFromSet(fields.Set{hmc.ReleaseVersionIndexKey: build.Version}),
	}
	if err := r.Client.List(ctx, releases, &listOptions); err != nil {
		return "", err
	}
	if len(releases.Items) != 1 {
		return "", fmt.Errorf("expected 1 Release with version %s, found %d", build.Version, len(releases.Items))
	}
	return releases.Items[0].Name, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ReleaseReconciler) SetupWithManager(mgr ctrl.Manager) error {
	c, err := ctrl.NewControllerManagedBy(mgr).
		For(&hmc.Release{}, builder.WithPredicates(predicate.Funcs{
			DeleteFunc:  func(event.DeleteEvent) bool { return false },
			GenericFunc: func(event.GenericEvent) bool { return false },
		})).
		Build(r)
	if err != nil {
		return err
	}
	//
	if !r.CreateManagement {
		return nil
	}
	// There's no Release objects created yet and we need to trigger reconcile
	initChannel := make(chan event.GenericEvent, 1)
	initChannel <- event.GenericEvent{Object: &hmc.Release{}}
	return c.Watch(source.Channel(initChannel, &handler.EnqueueRequestForObject{}))
}
