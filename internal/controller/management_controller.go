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
	"errors"
	"fmt"

	"github.com/fluxcd/pkg/apis/meta"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	hmc "github.com/Mirantis/hmc/api/v1alpha1"
	"github.com/Mirantis/hmc/internal/helm"
)

// ManagementReconciler reconciles a Management object
type ManagementReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	Config *rest.Config
}

//+kubebuilder:rbac:groups=hmc.mirantis.com,resources=managements,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=hmc.mirantis.com,resources=managements/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=hmc.mirantis.com,resources=managements/finalizers,verbs=update

func (r *ManagementReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	l := log.FromContext(ctx).WithValues("ManagementController", req.NamespacedName)
	l.Info("Reconciling Management")

	management := &hmc.Management{}
	if err := r.Get(ctx, req.NamespacedName, management); err != nil {
		if apierrors.IsNotFound(err) {
			l.Info("Management config not found, ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		l.Error(err, "Failed to get Management")
		return ctrl.Result{}, err
	}

	// TODO: this should be implemented in admission controller instead
	if changed := applyDefaultCoreConfiguration(management); changed {
		l.Info("Applying default core configuration")
		return ctrl.Result{}, r.Client.Update(ctx, management)
	}

	ownerRef := metav1.OwnerReference{
		APIVersion: hmc.GroupVersion.String(),
		Kind:       hmc.ManagementKind,
		Name:       management.Name,
		UID:        management.UID,
	}

	var errs error
	detectedProviders := hmc.Providers{}
	detectedComponents := make(map[string]hmc.ComponentStatus)

	components := wrappedComponents(management)
	for _, component := range components {
		template := &hmc.Template{}
		err := r.Get(ctx, types.NamespacedName{
			Namespace: hmc.TemplatesNamespace,
			Name:      component.Template,
		}, template)
		if err != nil {
			errMsg := fmt.Sprintf("Failed to get Template %s/%s: %s", hmc.TemplatesNamespace, component.Template, err)
			updateComponentsStatus(detectedComponents, &detectedProviders, component.Template, template.Status, errMsg)
			errs = errors.Join(fmt.Errorf(errMsg))
			continue
		}
		if !template.Status.Valid {
			errMsg := fmt.Sprintf("Template %s/%s is not marked as valid", hmc.TemplatesNamespace, component.Template)
			updateComponentsStatus(detectedComponents, &detectedProviders, component.Template, template.Status, errMsg)
			errs = errors.Join(fmt.Errorf(errMsg))
			continue
		}

		_, _, err = helm.ReconcileHelmRelease(ctx, r.Client, component.Template, management.Namespace, component.Config,
			ownerRef, template.Status.ChartRef, defaultReconcileInterval, component.dependsOn)
		if err != nil {
			errMsg := fmt.Sprintf("error reconciling HelmRelease %s/%s: %s", management.Namespace, component.Template, err)
			updateComponentsStatus(detectedComponents, &detectedProviders, component.Template, template.Status, errMsg)
			errs = errors.Join(fmt.Errorf(errMsg))
			continue
		}
		updateComponentsStatus(detectedComponents, &detectedProviders, component.Template, template.Status, "")
	}

	management.Status.ObservedGeneration = management.Generation
	management.Status.AvailableProviders = detectedProviders
	management.Status.Components = detectedComponents
	if err := r.Status().Update(ctx, management); err != nil {
		errs = errors.Join(fmt.Errorf("failed to update status for Management %s/%s: %w", management.Namespace, management.Name, err))
	}
	if errs != nil {
		l.Error(errs, "Multiple errors during Management reconciliation")
		return ctrl.Result{}, errs
	}
	return ctrl.Result{}, nil
}

type component struct {
	hmc.Component
	// helm release dependencies
	dependsOn []meta.NamespacedObjectReference
}

func wrappedComponents(mgmt *hmc.Management) (components []component) {
	if mgmt.Spec.Core == nil {
		return
	}
	components = append(components, component{Component: mgmt.Spec.Core.CertManager})
	components = append(components, component{Component: mgmt.Spec.Core.HMC, dependsOn: []meta.NamespacedObjectReference{{Name: mgmt.Spec.Core.CertManager.Template}}})
	components = append(components, component{Component: mgmt.Spec.Core.CAPI, dependsOn: []meta.NamespacedObjectReference{{Name: mgmt.Spec.Core.CertManager.Template}}})
	for provider := range mgmt.Spec.Providers {
		components = append(components, component{Component: mgmt.Spec.Providers[provider], dependsOn: []meta.NamespacedObjectReference{{Name: mgmt.Spec.Core.CAPI.Template}}})
	}
	return
}

func applyDefaultCoreConfiguration(mgmt *hmc.Management) (changed bool) {
	if mgmt.Spec.Core != nil {
		// Only apply defaults when there's no configuration provided
		return false
	}
	mgmt.Spec.Core = &hmc.Core{
		HMC: hmc.Component{
			Template: hmc.DefaultCoreHMCTemplate,
		},
		CAPI: hmc.Component{
			Template: hmc.DefaultCoreCAPITemplate,
		},
		CertManager: hmc.Component{
			Template: hmc.DefaultCoreCertManagerTemplate,
		},
	}

	return true
}

func updateComponentsStatus(
	components map[string]hmc.ComponentStatus,
	providers *hmc.Providers,
	componentName string,
	templateStatus hmc.TemplateStatus,
	err string) {

	components[componentName] = hmc.ComponentStatus{
		Error:   err,
		Success: err == "",
	}

	if err == "" {
		providers.InfrastructureProviders = append(providers.InfrastructureProviders, templateStatus.Providers.InfrastructureProviders...)
		providers.BootstrapProviders = append(providers.BootstrapProviders, templateStatus.Providers.BootstrapProviders...)
		providers.ControlPlaneProviders = append(providers.ControlPlaneProviders, templateStatus.Providers.ControlPlaneProviders...)
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *ManagementReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&hmc.Management{}).
		Complete(r)
}
