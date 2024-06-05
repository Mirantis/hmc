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
	"time"

	sourcev1 "github.com/fluxcd/source-controller/api/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/yaml"

	hmc "github.com/Mirantis/hmc/api/v1alpha1"
	"github.com/Mirantis/hmc/internal/utils"
)

const (
	defaultRepoName = "hmc-templates"
	defaultRepoType = "oci"

	defaultReconcileInterval = 10 * time.Minute
)

// TemplateReconciler reconciles a Template object
type TemplateReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	DefaultOCIRegistry string
	InsecureRegistry   bool
}

// +kubebuilder:rbac:groups=hmc.mirantis.com,resources=templates,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=hmc.mirantis.com,resources=templates/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=hmc.mirantis.com,resources=templates/finalizers,verbs=update
// +kubebuilder:rbac:groups=source.toolkit.fluxcd.io,resources=helmrepositories;helmcharts,verbs=get;list;watch;create;update;patch;delete

func (r *TemplateReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	l := log.FromContext(ctx).WithValues("TemplateController", req.NamespacedName)
	l.Info("Reconciling Template")

	template := &hmc.Template{}
	if err := r.Get(ctx, req.NamespacedName, template); err != nil {
		if errors.IsNotFound(err) {
			l.Info("Template not found, ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		l.Error(err, "Failed to get Template")
		return ctrl.Result{}, err
	}
	if template.Status.Valid {
		// We consider Template objects immutable, so we validate only once.
		// The chart will be validated later, when reconciling Deployment objects.
		l.Info("Template has already been validated, skipping validation")
		return ctrl.Result{}, nil
	}
	l.Info("Reconciling helm-controller objects ")
	err := r.reconcileHelmRepo(ctx, template)
	if err != nil {
		l.Error(err, "Failed to reconcile HelmRepo")
		return ctrl.Result{}, err
	}
	hcChart, err := r.reconcileHelmChart(ctx, template)
	if err != nil {
		l.Error(err, "Failed to reconcile HelmChart")
		return ctrl.Result{}, err
	}
	if hcChart == nil {
		// TODO: add externally referenced source verification
		return ctrl.Result{}, err
	}

	if err, reportStatus := helmArtifactReady(hcChart); err != nil {
		l.Info("HelmChart Artifact is not ready")
		if reportStatus {
			template.Status.ValidationError = err.Error()
			_ = r.updateStatus(ctx, template)
		}
		return ctrl.Result{}, err
	}

	l.Info("Downloading Helm chart")
	helmChart, err := utils.DownloadChartFromArtifact(ctx, hcChart.Status.Artifact)
	if err != nil {
		l.Error(err, "Failed to download Helm chart")
		err = fmt.Errorf("failed to download chart: %s", err)
		template.Status.ValidationError = err.Error()
		_ = r.updateStatus(ctx, template)
		return ctrl.Result{}, err
	}
	l.Info("Validating Helm chart")
	if err = helmChart.Validate(); err != nil {
		l.Error(err, "Helm chart validation failed")
		template.Status.ValidationError = err.Error()
		_ = r.updateStatus(ctx, template)
		return ctrl.Result{}, err
	}

	template.Status.Description = helmChart.Metadata.Description
	rawValues, err := yaml.Marshal(helmChart.Values)
	if err != nil {
		l.Error(err, "Failed to parse Helm chart values")
		err = fmt.Errorf("failed to parse Helm chart values: %s", err)
		template.Status.ValidationError = err.Error()
		_ = r.updateStatus(ctx, template)
		return ctrl.Result{}, err
	}
	template.Status.Configuration.Raw = rawValues
	l.Info("Chart validation completed successfully")
	template.Status.Valid = true
	template.Status.ValidationError = ""

	return ctrl.Result{}, r.updateStatus(ctx, template)
}

func (r *TemplateReconciler) updateStatus(ctx context.Context, template *hmc.Template) error {
	if err := r.Status().Update(ctx, template); err != nil {
		return fmt.Errorf("failed to update status for template %s/%s: %w", template.Namespace, template.Name, err)
	}
	return nil
}

func (r *TemplateReconciler) reconcileHelmRepo(ctx context.Context, template *hmc.Template) error {
	if template.Spec.Helm.ChartRef != nil {
		// HelmRepository is not managed by the controller
		return nil
	}
	helmRepo := &sourcev1.HelmRepository{
		ObjectMeta: metav1.ObjectMeta{
			Name:      defaultRepoName,
			Namespace: template.Namespace,
		},
	}
	_, err := ctrl.CreateOrUpdate(ctx, r.Client, helmRepo, func() error {
		helmRepo.Spec = sourcev1.HelmRepositorySpec{
			Type:     defaultRepoType,
			URL:      r.DefaultOCIRegistry,
			Interval: metav1.Duration{Duration: defaultReconcileInterval},
			Insecure: r.InsecureRegistry,
		}
		return nil
	})
	if err != nil {
		return err
	}
	return nil
}

func (r *TemplateReconciler) reconcileHelmChart(ctx context.Context, template *hmc.Template) (*sourcev1.HelmChart, error) {
	if template.Spec.Helm.ChartRef != nil {
		// HelmChart is not managed by the controller
		return nil, nil
	}
	helmChart := &sourcev1.HelmChart{
		ObjectMeta: metav1.ObjectMeta{
			Name:      template.Name,
			Namespace: template.Namespace,
		},
	}

	_, err := ctrl.CreateOrUpdate(ctx, r.Client, helmChart, func() error {
		helmChart.OwnerReferences = []metav1.OwnerReference{
			{
				APIVersion: hmc.GroupVersion.String(),
				Kind:       hmc.TemplateKind,
				Name:       template.Name,
				UID:        template.UID,
			},
		}
		helmChart.Spec = sourcev1.HelmChartSpec{
			Chart:   template.Spec.Helm.ChartName,
			Version: template.Spec.Helm.ChartVersion,
			SourceRef: sourcev1.LocalHelmChartSourceReference{
				Kind: sourcev1.HelmRepositoryKind,
				Name: defaultRepoName,
			},
			Interval: metav1.Duration{Duration: defaultReconcileInterval},
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return helmChart, nil
}

func helmArtifactReady(chart *sourcev1.HelmChart) (err error, reportStatus bool) {
	for _, c := range chart.Status.Conditions {
		if c.Type == "Ready" {
			if chart.Generation != c.ObservedGeneration {
				return fmt.Errorf("HelmChart was not reconciled yet, retrying"), false
			}
			if c.Status != metav1.ConditionTrue {
				return fmt.Errorf("failed to download helm chart artifact: %s", c.Message), true
			}
		}
	}
	if chart.Status.Artifact == nil || chart.Status.URL == "" {
		return fmt.Errorf("helm chart artifact is not ready yet"), false
	}
	return nil, false
}

// SetupWithManager sets up the controller with the Manager.
func (r *TemplateReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&hmc.Template{}).
		Complete(r)
}
