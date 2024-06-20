/*
Copyright 2024.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"context"
	"fmt"

	hcv2 "github.com/fluxcd/helm-controller/api/v2"
	sourcev1 "github.com/fluxcd/source-controller/api/v1"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	hmc "github.com/Mirantis/hmc/api/v1alpha1"
	"github.com/Mirantis/hmc/internal/helm"
	"github.com/Mirantis/hmc/internal/telemetry"
)

// DeploymentReconciler reconciles a Deployment object
type DeploymentReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	Config *rest.Config
}

//+kubebuilder:rbac:groups=hmc.mirantis.com,resources=deployments,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=hmc.mirantis.com,resources=deployments/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=hmc.mirantis.com,resources=deployments/finalizers,verbs=update
//+kubebuilder:rbac:groups=helm.toolkit.fluxcd.io,resources=helmreleases,verbs=get;list;watch;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *DeploymentReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	l := log.FromContext(ctx).WithValues("DeploymentController", req.NamespacedName)
	l.Info("Reconciling Deployment")

	deployment := &hmc.Deployment{}
	if err := r.Get(ctx, req.NamespacedName, deployment); err != nil {
		if errors.IsNotFound(err) {
			l.Info("Deployment not found, ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		l.Error(err, "Failed to get Deployment")
		return ctrl.Result{}, err
	}

	if deployment.Status.ObservedGeneration == 0 {
		if err := telemetry.TrackDeploymentCreate(string(deployment.UID), deployment.Spec.Template, deployment.Spec.DryRun); err != nil {
			l.Error(err, "Failed to track Deployment creation")
		}
	}
	template := &hmc.Template{}
	templateRef := types.NamespacedName{Name: deployment.Spec.Template, Namespace: deployment.Namespace}
	if err := r.Get(ctx, templateRef, template); err != nil {
		l.Error(err, "Failed to get Template")
		errMsg := fmt.Sprintf("failed to get provided template: %s", err)
		if errors.IsNotFound(err) {
			errMsg = "provided template is not found"
		}
		_ = r.updateStatus(ctx, deployment, errMsg)
		return ctrl.Result{}, err
	}
	if !template.Status.Valid {
		errMsg := "provided template is not marked as valid"
		_ = r.updateStatus(ctx, deployment, errMsg)
		return ctrl.Result{}, fmt.Errorf(errMsg)
	}

	// TODO: this should be implemented in admission controller instead
	if changed := applyDefaultDeploymentConfiguration(deployment, template); changed {
		l.Info("Applying default configuration")
		return ctrl.Result{}, r.Client.Update(ctx, deployment)
	}
	source, err := r.getSource(ctx, template.Status.ChartRef)
	if err != nil {
		_ = r.updateStatus(ctx, deployment, fmt.Sprintf("failed to get helm chart source: %s", err))
		return ctrl.Result{}, err
	}
	l.Info("Downloading Helm chart")
	hcChart, err := helm.DownloadChartFromArtifact(ctx, source.GetArtifact())
	if err != nil {
		_ = r.updateStatus(ctx, deployment, fmt.Sprintf("failed to download helm chart: %s", err))
		return ctrl.Result{}, err
	}

	l.Info("Initializing Helm client")
	getter := helm.NewMemoryRESTClientGetter(r.Config, r.RESTMapper())
	actionConfig := new(action.Configuration)
	err = actionConfig.Init(getter, deployment.Namespace, "secret", l.Info)
	if err != nil {
		_ = r.updateStatus(ctx, deployment, fmt.Sprintf("failed to initialize helm client: %s", err))
		return ctrl.Result{}, err
	}

	l.Info("Validating Helm chart with provided values")
	if err := r.validateReleaseWithValues(ctx, actionConfig, deployment, hcChart); err != nil {
		_ = r.updateStatus(ctx, deployment, fmt.Sprintf("failed to validate template with provided configuration: %s", err))
		return ctrl.Result{}, err
	}
	if !deployment.Spec.DryRun {
		ownerRef := metav1.OwnerReference{
			APIVersion: hmc.GroupVersion.String(),
			Kind:       hmc.DeploymentKind,
			Name:       deployment.Name,
			UID:        deployment.UID,
		}
		_, err := helm.ReconcileHelmRelease(ctx, r.Client, deployment.Name, deployment.Namespace, deployment.Spec.Config,
			ownerRef, template.Status.ChartRef, defaultReconcileInterval, nil)
		if err != nil {
			return ctrl.Result{}, err
		}
	}
	return ctrl.Result{}, r.updateStatus(ctx, deployment, "")
}

func (r *DeploymentReconciler) validateReleaseWithValues(ctx context.Context, actionConfig *action.Configuration, deployment *hmc.Deployment, hcChart *chart.Chart) error {
	install := action.NewInstall(actionConfig)
	install.DryRun = true
	install.ReleaseName = deployment.Name
	install.Namespace = deployment.Namespace
	install.ClientOnly = true

	vals, err := deployment.HelmValues()
	if err != nil {
		return err
	}
	_, err = install.RunWithContext(ctx, hcChart, vals)
	if err != nil {
		return err
	}
	return nil
}

func (r *DeploymentReconciler) updateStatus(ctx context.Context, deployment *hmc.Deployment, validationError string) error {
	deployment.Status.ObservedGeneration = deployment.Generation
	deployment.Status.ValidationError = validationError
	deployment.Status.Valid = validationError == ""
	if err := r.Status().Update(ctx, deployment); err != nil {
		return fmt.Errorf("failed to update status for deployment %s/%s: %w", deployment.Namespace, deployment.Name, err)
	}
	return nil
}

func (r *DeploymentReconciler) getSource(ctx context.Context, ref *hcv2.CrossNamespaceSourceReference) (sourcev1.Source, error) {
	if ref == nil {
		return nil, fmt.Errorf("helm chart source is not provided")
	}
	chartRef := types.NamespacedName{Namespace: ref.Namespace, Name: ref.Name}
	hc := sourcev1.HelmChart{}
	if err := r.Client.Get(ctx, chartRef, &hc); err != nil {
		return nil, err
	}
	return &hc, nil
}

func applyDefaultDeploymentConfiguration(deployment *hmc.Deployment, template *hmc.Template) (changed bool) {
	if deployment.Spec.Config != nil || template.Status.Config == nil {
		// Only apply defaults when there's no configuration provided
		return false
	}
	deployment.Spec.DryRun = true
	deployment.Spec.Config = &apiextensionsv1.JSON{Raw: template.Status.Config.Raw}
	return true
}

// SetupWithManager sets up the controller with the Manager.
func (r *DeploymentReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&hmc.Deployment{}).
		Complete(r)
}
