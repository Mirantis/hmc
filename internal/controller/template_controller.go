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
	"fmt"
	"strings"
	"time"

	v2 "github.com/fluxcd/helm-controller/api/v2"
	"github.com/fluxcd/pkg/apis/meta"
	sourcev1 "github.com/fluxcd/source-controller/api/v1"
	"helm.sh/helm/v3/pkg/chart"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	hmc "github.com/Mirantis/hmc/api/v1alpha1"
	"github.com/Mirantis/hmc/internal/helm"
)

const (
	defaultRepoName = "hmc-templates"
	defaultRepoType = "oci"

	defaultReconcileInterval = 10 * time.Minute
)

var (
	errNoProviderType = fmt.Errorf("template type is not supported: %s chart annotation must be one of [%s/%s/%s]",
		hmc.ChartAnnotationType, hmc.TemplateTypeDeployment, hmc.TemplateTypeProvider, hmc.TemplateTypeCore)
)

// TemplateReconciler reconciles a Template object
type TemplateReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	DefaultOCIRegistry        string
	RegistryCredentialsSecret string
	InsecureRegistry          bool
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
		if apierrors.IsNotFound(err) {
			l.Info("Template not found, ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		l.Error(err, "Failed to get Template")
		return ctrl.Result{}, err
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

	template.Status.ChartRef = &v2.CrossNamespaceSourceReference{
		Kind:      hcChart.Kind,
		Name:      hcChart.Name,
		Namespace: hcChart.Namespace,
	}
	if err, reportStatus := helmArtifactReady(hcChart); err != nil {
		l.Info("HelmChart Artifact is not ready")
		if reportStatus {
			_ = r.updateStatus(ctx, template, err.Error())
		}
		return ctrl.Result{}, err
	}

	l.Info("Downloading Helm chart")
	helmChart, err := helm.DownloadChartFromArtifact(ctx, hcChart.Status.Artifact)
	if err != nil {
		l.Error(err, "Failed to download Helm chart")
		err = fmt.Errorf("failed to download chart: %s", err)
		_ = r.updateStatus(ctx, template, err.Error())
		return ctrl.Result{}, err
	}
	l.Info("Validating Helm chart")
	if err := r.parseChartMetadata(template, helmChart); err != nil {
		l.Error(err, "Failed to parse Helm chart metadata")
		_ = r.updateStatus(ctx, template, err.Error())
		return ctrl.Result{}, err
	}
	if err = helmChart.Validate(); err != nil {
		l.Error(err, "Helm chart validation failed")
		_ = r.updateStatus(ctx, template, err.Error())
		return ctrl.Result{}, err
	}

	template.Status.Description = helmChart.Metadata.Description
	rawValues, err := json.Marshal(helmChart.Values)
	if err != nil {
		l.Error(err, "Failed to parse Helm chart values")
		err = fmt.Errorf("failed to parse Helm chart values: %s", err)
		_ = r.updateStatus(ctx, template, err.Error())
		return ctrl.Result{}, err
	}
	template.Status.Config = &apiextensionsv1.JSON{Raw: rawValues}
	l.Info("Chart validation completed successfully")

	return ctrl.Result{}, r.updateStatus(ctx, template, "")
}

func (r *TemplateReconciler) parseChartMetadata(template *hmc.Template, chart *chart.Chart) error {
	if chart.Metadata == nil {
		return fmt.Errorf("chart metadata is empty")
	}
	templateType := chart.Metadata.Annotations[hmc.ChartAnnotationType]
	switch hmc.TemplateType(templateType) {
	case hmc.TemplateTypeDeployment, hmc.TemplateTypeProvider, hmc.TemplateTypeCore:
	default:
		return errNoProviderType
	}
	template.Status.Type = hmc.TemplateType(templateType)

	// the value in spec has higher priority
	if len(template.Spec.Providers.InfrastructureProviders) > 0 {
		template.Status.Providers.InfrastructureProviders = template.Spec.Providers.InfrastructureProviders
	} else {
		infraProviders := chart.Metadata.Annotations[hmc.ChartAnnotationInfraProviders]
		if infraProviders != "" {
			template.Status.Providers.InfrastructureProviders = strings.Split(infraProviders, ",")
		}
	}
	if len(template.Spec.Providers.BootstrapProviders) > 0 {
		template.Status.Providers.BootstrapProviders = template.Spec.Providers.BootstrapProviders
	} else {
		bootstrapProviders := chart.Metadata.Annotations[hmc.ChartAnnotationBootstrapProviders]
		if bootstrapProviders != "" {
			template.Status.Providers.BootstrapProviders = strings.Split(bootstrapProviders, ",")
		}
	}
	if len(template.Spec.Providers.ControlPlaneProviders) > 0 {
		template.Status.Providers.ControlPlaneProviders = template.Spec.Providers.ControlPlaneProviders
	} else {
		cpProviders := chart.Metadata.Annotations[hmc.ChartAnnotationControlPlaneProviders]
		if cpProviders != "" {
			template.Status.Providers.ControlPlaneProviders = strings.Split(cpProviders, ",")
		}
	}
	return nil
}

func (r *TemplateReconciler) updateStatus(ctx context.Context, template *hmc.Template, validationError string) error {
	template.Status.ObservedGeneration = template.Generation
	template.Status.ValidationError = validationError
	template.Status.Valid = validationError == ""
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
		if helmRepo.Labels == nil {
			helmRepo.Labels = make(map[string]string)
		}
		helmRepo.Labels[hmc.HMCManagedLabelKey] = "true"
		helmRepo.Spec = sourcev1.HelmRepositorySpec{
			Type:     defaultRepoType,
			URL:      r.DefaultOCIRegistry,
			Interval: metav1.Duration{Duration: defaultReconcileInterval},
			Insecure: r.InsecureRegistry,
		}
		if r.RegistryCredentialsSecret != "" {
			helmRepo.Spec.SecretRef = &meta.LocalObjectReference{
				Name: r.RegistryCredentialsSecret,
			}
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
		if helmChart.Labels == nil {
			helmChart.Labels = make(map[string]string)
		}
		helmChart.Labels[hmc.HMCManagedLabelKey] = "true"
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
