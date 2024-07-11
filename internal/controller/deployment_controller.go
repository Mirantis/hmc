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
	"time"

	hcv2 "github.com/fluxcd/helm-controller/api/v2"
	fluxmeta "github.com/fluxcd/pkg/apis/meta"
	fluxconditions "github.com/fluxcd/pkg/runtime/conditions"
	sourcev1 "github.com/fluxcd/source-controller/api/v1"
	"github.com/go-logr/logr"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

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
		if apierrors.IsNotFound(err) {
			l.Info("Deployment not found, ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		l.Error(err, "Failed to get Deployment")
		return ctrl.Result{}, err
	}

	if !deployment.DeletionTimestamp.IsZero() {
		l.Info("Deleting Deployment")
		return r.Delete(ctx, l, deployment)
	}

	if deployment.Status.ObservedGeneration == 0 {
		if err := telemetry.TrackDeploymentCreate(string(deployment.UID), deployment.Spec.Template, deployment.Spec.DryRun); err != nil {
			l.Error(err, "Failed to track Deployment creation")
		}
	}
	return r.Update(ctx, l, deployment)
}

func (r *DeploymentReconciler) Update(ctx context.Context, l logr.Logger, deployment *hmc.Deployment) (result ctrl.Result, err error) {
	finalizersUpdated := controllerutil.AddFinalizer(deployment, hmc.DeploymentFinalizer)
	if finalizersUpdated {
		if err := r.Client.Update(ctx, deployment); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to update deployment %s/%s: %w", deployment.Namespace, deployment.Name, err)
		}
		return ctrl.Result{}, nil
	}

	if len(deployment.Status.Conditions) == 0 {
		deployment.InitConditions()
	}

	defer func() {
		err = errors.Join(err, r.updateStatus(ctx, deployment))
	}()

	template := &hmc.Template{}
	templateRef := types.NamespacedName{Name: deployment.Spec.Template, Namespace: hmc.TemplatesNamespace}
	if err := r.Get(ctx, templateRef, template); err != nil {
		l.Error(err, "Failed to get Template")
		errMsg := fmt.Sprintf("failed to get provided template: %s", err)
		if apierrors.IsNotFound(err) {
			errMsg = "provided template is not found"
		}
		apimeta.SetStatusCondition(deployment.GetConditions(), metav1.Condition{
			Type:    hmc.TemplateReadyCondition,
			Status:  metav1.ConditionFalse,
			Reason:  hmc.FailedReason,
			Message: errMsg,
		})
		return ctrl.Result{}, err
	}
	if !template.Status.Valid {
		errMsg := "provided template is not marked as valid"
		apimeta.SetStatusCondition(deployment.GetConditions(), metav1.Condition{
			Type:    hmc.TemplateReadyCondition,
			Status:  metav1.ConditionFalse,
			Reason:  hmc.FailedReason,
			Message: errMsg,
		})
		return ctrl.Result{}, fmt.Errorf(errMsg)
	}
	apimeta.SetStatusCondition(deployment.GetConditions(), metav1.Condition{
		Type:    hmc.TemplateReadyCondition,
		Status:  metav1.ConditionTrue,
		Reason:  hmc.SucceededReason,
		Message: "Template is valid",
	})
	// TODO: this should be implemented in admission controller instead
	if changed := applyDefaultDeploymentConfiguration(deployment, template); changed {
		l.Info("Applying default configuration")
		return ctrl.Result{}, r.Client.Update(ctx, deployment)
	}
	source, err := r.getSource(ctx, template.Status.ChartRef)
	if err != nil {
		apimeta.SetStatusCondition(deployment.GetConditions(), metav1.Condition{
			Type:    hmc.HelmChartReadyCondition,
			Status:  metav1.ConditionFalse,
			Reason:  hmc.FailedReason,
			Message: fmt.Sprintf("failed to get helm chart source: %s", err),
		})
		return ctrl.Result{}, err
	}
	l.Info("Downloading Helm chart")
	hcChart, err := helm.DownloadChartFromArtifact(ctx, source.GetArtifact())
	if err != nil {
		apimeta.SetStatusCondition(deployment.GetConditions(), metav1.Condition{
			Type:    hmc.HelmChartReadyCondition,
			Status:  metav1.ConditionFalse,
			Reason:  hmc.FailedReason,
			Message: fmt.Sprintf("failed to download helm chart: %s", err),
		})
		return ctrl.Result{}, err
	}

	l.Info("Initializing Helm client")
	getter := helm.NewMemoryRESTClientGetter(r.Config, r.RESTMapper())
	actionConfig := new(action.Configuration)
	err = actionConfig.Init(getter, deployment.Namespace, "secret", l.Info)
	if err != nil {
		return ctrl.Result{}, err
	}

	l.Info("Validating Helm chart with provided values")
	if err := r.validateReleaseWithValues(ctx, actionConfig, deployment, hcChart); err != nil {
		apimeta.SetStatusCondition(deployment.GetConditions(), metav1.Condition{
			Type:    hmc.HelmChartReadyCondition,
			Status:  metav1.ConditionFalse,
			Reason:  hmc.FailedReason,
			Message: fmt.Sprintf("failed to validate template with provided configuration: %s", err),
		})
		return ctrl.Result{}, err
	}

	apimeta.SetStatusCondition(deployment.GetConditions(), metav1.Condition{
		Type:    hmc.HelmChartReadyCondition,
		Status:  metav1.ConditionTrue,
		Reason:  hmc.SucceededReason,
		Message: "Helm chart is valid",
	})

	if !deployment.Spec.DryRun {
		ownerRef := metav1.OwnerReference{
			APIVersion: hmc.GroupVersion.String(),
			Kind:       hmc.DeploymentKind,
			Name:       deployment.Name,
			UID:        deployment.UID,
		}

		hr, err := helm.ReconcileHelmRelease(ctx, r.Client, deployment.Name, deployment.Namespace, deployment.Spec.Config,
			ownerRef, template.Status.ChartRef, defaultReconcileInterval, nil)
		if err != nil {
			apimeta.SetStatusCondition(deployment.GetConditions(), metav1.Condition{
				Type:    hmc.HelmReleaseReadyCondition,
				Status:  metav1.ConditionFalse,
				Reason:  hmc.FailedReason,
				Message: err.Error(),
			})
			return ctrl.Result{}, err
		}

		hrReadyCondition := fluxconditions.Get(hr, fluxmeta.ReadyCondition)
		if hrReadyCondition != nil {
			apimeta.SetStatusCondition(deployment.GetConditions(), metav1.Condition{
				Type:    hmc.HelmReleaseReadyCondition,
				Status:  hrReadyCondition.Status,
				Reason:  hrReadyCondition.Reason,
				Message: hrReadyCondition.Message,
			})
		}
		if !fluxconditions.IsReady(hr) {
			return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
		}
	}
	return ctrl.Result{}, nil
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

func (r *DeploymentReconciler) updateStatus(ctx context.Context, deployment *hmc.Deployment) error {
	deployment.Status.ObservedGeneration = deployment.Generation
	warnings := ""
	errs := ""
	for _, condition := range deployment.Status.Conditions {
		if condition.Type == hmc.ReadyCondition {
			continue
		}
		if condition.Status == metav1.ConditionUnknown {
			warnings += condition.Message + ". "
		}
		if condition.Status == metav1.ConditionFalse {
			errs += condition.Message + ". "
		}
	}
	condition := metav1.Condition{
		Type:    hmc.ReadyCondition,
		Status:  metav1.ConditionTrue,
		Reason:  hmc.SucceededReason,
		Message: "Deployment is ready",
	}
	if warnings != "" {
		condition.Status = metav1.ConditionUnknown
		condition.Reason = hmc.ProgressingReason
		condition.Message = warnings
	}
	if errs != "" {
		condition.Status = metav1.ConditionFalse
		condition.Reason = hmc.FailedReason
		condition.Message = errs
	}
	apimeta.SetStatusCondition(deployment.GetConditions(), condition)
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

func (r *DeploymentReconciler) Delete(ctx context.Context, l logr.Logger, deployment *hmc.Deployment) (ctrl.Result, error) {
	hr := &hcv2.HelmRelease{}
	err := r.Get(ctx, types.NamespacedName{
		Name:      deployment.Name,
		Namespace: deployment.Namespace,
	}, hr)
	if err != nil {
		if apierrors.IsNotFound(err) {
			l.Info("Removing Finalizer", "finalizer", hmc.DeploymentFinalizer)
			finalizersUpdated := controllerutil.RemoveFinalizer(deployment, hmc.DeploymentFinalizer)
			if finalizersUpdated {
				if err := r.Client.Update(ctx, deployment); err != nil {
					return ctrl.Result{}, fmt.Errorf("failed to update deployment %s/%s: %w", deployment.Namespace, deployment.Name, err)
				}
			}
			l.Info("Deployment deleted")
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}
	err = helm.DeleteHelmRelease(ctx, r.Client, deployment.Name, deployment.Namespace)
	if err != nil {
		return ctrl.Result{}, err
	}
	l.Info("HelmRelease still exists, retrying")
	return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *DeploymentReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&hmc.Deployment{}).
		Watches(&hcv2.HelmRelease{},
			handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, o client.Object) []ctrl.Request {
				deployment := hmc.Deployment{}
				deploymentRef := types.NamespacedName{
					Namespace: o.GetNamespace(),
					Name:      o.GetName(),
				}
				err := r.Client.Get(ctx, deploymentRef, &deployment)
				if err != nil {
					return []ctrl.Request{}
				}
				return []reconcile.Request{
					{
						NamespacedName: deploymentRef,
					},
				}
			}),
		).
		Complete(r)
}
