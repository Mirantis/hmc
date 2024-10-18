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
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	hmc "github.com/Mirantis/hmc/api/v1alpha1"
	"github.com/Mirantis/hmc/internal/sveltos"
	"github.com/Mirantis/hmc/internal/utils"
	sourcev1 "github.com/fluxcd/source-controller/api/v1"
)

// MultiClusterServiceReconciler reconciles a MultiClusterService object
type MultiClusterServiceReconciler struct {
	client.Client
}

// Reconcile reconciles a MultiClusterService object.
func (r *MultiClusterServiceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	l := ctrl.LoggerFrom(ctx)
	l.Info("Reconciling MultiClusterService")

	mcsvc := &hmc.MultiClusterService{}
	err := r.Get(ctx, req.NamespacedName, mcsvc)
	if apierrors.IsNotFound(err) {
		l.Info("MultiClusterService not found, ignoring since object must be deleted")
		return ctrl.Result{}, nil
	}
	if err != nil {
		l.Error(err, "Failed to get MultiClusterService")
		return ctrl.Result{}, err
	}

	if !mcsvc.DeletionTimestamp.IsZero() {
		l.Info("Deleting MultiClusterService")
		return r.reconcileDelete(ctx, mcsvc)
	}

	if controllerutil.AddFinalizer(mcsvc, hmc.MultiClusterServiceFinalizer) {
		if err := r.Client.Update(ctx, mcsvc); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to update MultiClusterService %s with finalizer %s: %w", mcsvc.Name, hmc.MultiClusterServiceFinalizer, err)
		}
		return ctrl.Result{}, nil
	}

	// By using DefaultSystemNamespace we are enforcing that MultiClusterService
	// may only use ServiceTemplates that are present in the hmc-system namespace.
	opts, err := helmChartOpts(ctx, r.Client, utils.DefaultSystemNamespace, mcsvc.Spec.Services)
	if err != nil {
		return ctrl.Result{}, err
	}

	if _, err := sveltos.ReconcileClusterProfile(ctx, r.Client, mcsvc.Name,
		sveltos.ReconcileProfileOpts{
			OwnerReference: &metav1.OwnerReference{
				APIVersion: hmc.GroupVersion.String(),
				Kind:       hmc.MultiClusterServiceKind,
				Name:       mcsvc.Name,
				UID:        mcsvc.UID,
			},
			LabelSelector:  mcsvc.Spec.ClusterSelector,
			HelmChartOpts:  opts,
			Priority:       mcsvc.Spec.ServicesPriority,
			StopOnConflict: mcsvc.Spec.StopOnConflict,
		}); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to reconcile ClusterProfile: %w", err)
	}

	return ctrl.Result{}, nil
}

// helmChartOpts returns slice of helm chart options to use with Sveltos.
// Namespace is the namespace of the referred templates in services slice.
func helmChartOpts(ctx context.Context, c client.Client, namespace string, services []hmc.ServiceSpec) ([]sveltos.HelmChartOpts, error) {
	l := ctrl.LoggerFrom(ctx)
	opts := []sveltos.HelmChartOpts{}

	// NOTE: The Profile/ClusterProfile object will be updated with
	// no helm charts if len(mc.Spec.Services) == 0. This will result
	// in the helm charts being uninstalled on matching clusters if
	// Profile/ClusterProfile originally had len(m.Spec.Sevices) > 0.
	for _, svc := range services {
		if svc.Disable {
			l.Info(fmt.Sprintf("Skip adding ServiceTemplate %s because Disable=true", svc.Template))
			continue
		}

		tmpl := &hmc.ServiceTemplate{}
		// Here we can use the same namespace for all services
		// because if the services slice is part of:
		// 1. ManagedCluster: Then the referred template must be in its own namespace.
		// 2. MultiClusterService: Then the referred template must be in hmc-system namespace.
		tmplRef := types.NamespacedName{Name: svc.Template, Namespace: namespace}
		if err := c.Get(ctx, tmplRef, tmpl); err != nil {
			return nil, fmt.Errorf("failed to get ServiceTemplate %s: %w", tmplRef.String(), err)
		}

		if tmpl.GetCommonStatus() == nil || tmpl.GetCommonStatus().ChartRef == nil {
			return nil, fmt.Errorf("status for ServiceTemplate %s/%s has not been updated yet", tmpl.Namespace, tmpl.Name)
		}

		chart := &sourcev1.HelmChart{}
		chartRef := types.NamespacedName{
			Namespace: tmpl.GetCommonStatus().ChartRef.Namespace,
			Name:      tmpl.GetCommonStatus().ChartRef.Name,
		}
		if err := c.Get(ctx, chartRef, chart); err != nil {
			return nil, fmt.Errorf("failed to get HelmChart %s referenced by ServiceTemplate %s: %w", chartRef.String(), tmplRef.String(), err)
		}

		repo := &sourcev1.HelmRepository{}
		repoRef := types.NamespacedName{
			// Using chart's namespace because it's source
			// should be within the same namespace.
			Namespace: chart.Namespace,
			Name:      chart.Spec.SourceRef.Name,
		}
		if err := c.Get(ctx, repoRef, repo); err != nil {
			return nil, fmt.Errorf("failed to get HelmRepository %s: %w", repoRef.String(), err)
		}

		chartName := tmpl.Spec.Helm.ChartName
		if chartName == "" {
			chartName = tmpl.Spec.Helm.ChartRef.Name
		}

		opts = append(opts, sveltos.HelmChartOpts{
			Values:        svc.Values,
			RepositoryURL: repo.Spec.URL,
			// We don't have repository name so chart name becomes repository name.
			RepositoryName: chartName,
			ChartName: func() string {
				if repo.Spec.Type == utils.RegistryTypeOCI {
					return chartName
				}
				// Sveltos accepts ChartName in <repository>/<chart> format for non-OCI.
				// We don't have a repository name, so we can use <chart>/<chart> instead.
				// See: https://projectsveltos.github.io/sveltos/addons/helm_charts/.
				return fmt.Sprintf("%s/%s", chartName, chartName)
			}(),
			ChartVersion: tmpl.Spec.Helm.ChartVersion,
			ReleaseName:  svc.Name,
			ReleaseNamespace: func() string {
				if svc.Namespace != "" {
					return svc.Namespace
				}
				return svc.Name
			}(),
			// The reason it is passed to PlainHTTP instead of InsecureSkipTLSVerify is because
			// the source.Spec.Insecure field is meant to be used for connecting to repositories
			// over plain HTTP, which is different than what InsecureSkipTLSVerify is meant for.
			// See: https://github.com/fluxcd/source-controller/pull/1288
			PlainHTTP: repo.Spec.Insecure,
		})
	}

	return opts, nil
}

func (r *MultiClusterServiceReconciler) reconcileDelete(ctx context.Context, mcsvc *hmc.MultiClusterService) (ctrl.Result, error) {
	if err := sveltos.DeleteClusterProfile(ctx, r.Client, mcsvc.Name); err != nil {
		return ctrl.Result{}, err
	}

	if controllerutil.RemoveFinalizer(mcsvc, hmc.MultiClusterServiceFinalizer) {
		if err := r.Client.Update(ctx, mcsvc); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to remove finalizer %s from MultiClusterService %s: %w", hmc.MultiClusterServiceFinalizer, mcsvc.Name, err)
		}
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *MultiClusterServiceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&hmc.MultiClusterService{}).
		Complete(r)
}
