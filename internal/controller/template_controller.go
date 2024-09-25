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

	helmcontrollerv2 "github.com/fluxcd/helm-controller/api/v2"
	"github.com/fluxcd/pkg/apis/meta"
	sourcev1 "github.com/fluxcd/source-controller/api/v1"
	"helm.sh/helm/v3/pkg/chart"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	hmc "github.com/Mirantis/hmc/api/v1alpha1"
	"github.com/Mirantis/hmc/internal/helm"
)

const (
	defaultRepoName = "hmc-templates"
)

// TemplateReconciler reconciles a Template object
type TemplateReconciler struct {
	client.Client
	Scheme          *runtime.Scheme
	SystemNamespace string

	// DefaultRepoType is the type specified by default in HelmRepository
	// objects.  Valid types are 'default' for http/https repositories, and
	// 'oci' for OCI repositories.  The RepositoryType is set in main based on
	// the URI scheme of the DefaultRegistryURL.
	DefaultRepoType           string
	DefaultRegistryURL        string
	RegistryCredentialsSecret string
	InsecureRegistry          bool

	downloadHelmChartFunc func(context.Context, *sourcev1.Artifact) (*chart.Chart, error)
}

type ClusterTemplateReconciler struct {
	TemplateReconciler
}

type ServiceTemplateReconciler struct {
	TemplateReconciler
}

type ProviderTemplateReconciler struct {
	TemplateReconciler
}

func (r *ClusterTemplateReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	l := log.FromContext(ctx).WithValues("ClusterTemplateController", req.NamespacedName)
	l.Info("Reconciling ClusterTemplate")

	clusterTemplate := &hmc.ClusterTemplate{}
	err := r.Get(ctx, req.NamespacedName, clusterTemplate)
	if err != nil {
		if apierrors.IsNotFound(err) {
			l.Info("ClusterTemplate not found, ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		l.Error(err, "Failed to get ClusterTemplate")
		return ctrl.Result{}, err
	}
	return r.ReconcileTemplate(ctx, clusterTemplate)
}

func (r *ServiceTemplateReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	l := log.FromContext(ctx).WithValues("ServiceTemplateReconciler", req.NamespacedName)
	l.Info("Reconciling ServiceTemplate")

	serviceTemplate := &hmc.ServiceTemplate{}
	err := r.Get(ctx, req.NamespacedName, serviceTemplate)
	if err != nil {
		if apierrors.IsNotFound(err) {
			l.Info("ServiceTemplate not found, ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		l.Error(err, "Failed to get ServiceTemplate")
		return ctrl.Result{}, err
	}
	return r.ReconcileTemplate(ctx, serviceTemplate)
}

func (r *ProviderTemplateReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	l := log.FromContext(ctx).WithValues("ProviderTemplateReconciler", req.NamespacedName)
	l.Info("Reconciling ProviderTemplate")

	providerTemplate := &hmc.ProviderTemplate{}
	err := r.Get(ctx, req.NamespacedName, providerTemplate)
	if err != nil {
		if apierrors.IsNotFound(err) {
			l.Info("ProviderTemplate not found, ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		l.Error(err, "Failed to get ProviderTemplate")
		return ctrl.Result{}, err
	}
	return r.ReconcileTemplate(ctx, providerTemplate)
}

// Template is the interface defining a list of methods to interact with templates
type Template interface {
	client.Object
	GetSpec() *hmc.TemplateSpecCommon
	GetStatus() *hmc.TemplateStatusCommon
}

func (r *TemplateReconciler) ReconcileTemplate(ctx context.Context, template Template) (ctrl.Result, error) {
	l := log.FromContext(ctx)

	spec := template.GetSpec()
	status := template.GetStatus()
	var err error
	var hcChart *sourcev1.HelmChart
	if spec.Helm.ChartRef != nil {
		hcChart, err = r.getHelmChartFromChartRef(ctx, spec.Helm.ChartRef)
		if err != nil {
			l.Error(err, "failed to get artifact from chartRef", "kind", spec.Helm.ChartRef.Kind, "namespace", spec.Helm.ChartRef.Namespace, "name", spec.Helm.ChartRef.Name)
			return ctrl.Result{}, err
		}
	} else {
		if spec.Helm.ChartName == "" {
			err := fmt.Errorf("neither chartName nor chartRef is set")
			l.Error(err, "invalid helm chart reference")
			return ctrl.Result{}, err
		}
		if template.GetNamespace() == r.SystemNamespace || !templateManagedByHMC(template) {
			err := r.reconcileDefaultHelmRepository(ctx, template.GetNamespace())
			if err != nil {
				l.Error(err, "Failed to reconcile default HelmRepository", "namespace", template.GetNamespace())
				return ctrl.Result{}, err
			}
		}
		l.Info("Reconciling helm-controller objects ")
		hcChart, err = r.reconcileHelmChart(ctx, template)
		if err != nil {
			l.Error(err, "Failed to reconcile HelmChart")
			return ctrl.Result{}, err
		}
	}
	if hcChart == nil {
		err := fmt.Errorf("HelmChart is nil")
		l.Error(err, "could not get the helm chart")
		return ctrl.Result{}, err
	}

	status.ChartRef = &helmcontrollerv2.CrossNamespaceSourceReference{
		Kind:      sourcev1.HelmChartKind,
		Name:      hcChart.Name,
		Namespace: hcChart.Namespace,
	}
	if reportStatus, err := helm.ArtifactReady(hcChart); err != nil {
		l.Info("HelmChart Artifact is not ready")
		if reportStatus {
			_ = r.updateStatus(ctx, template, err.Error())
		}
		return ctrl.Result{}, err
	}

	artifact := hcChart.Status.Artifact

	if r.downloadHelmChartFunc == nil {
		r.downloadHelmChartFunc = helm.DownloadChartFromArtifact
	}

	l.Info("Downloading Helm chart")
	helmChart, err := r.downloadHelmChartFunc(ctx, artifact)
	if err != nil {
		l.Error(err, "Failed to download Helm chart")
		err = fmt.Errorf("failed to download chart: %s", err)
		_ = r.updateStatus(ctx, template, err.Error())
		return ctrl.Result{}, err
	}
	l.Info("Validating Helm chart")
	if err := parseChartMetadata(template, helmChart); err != nil {
		l.Error(err, "Failed to parse Helm chart metadata")
		_ = r.updateStatus(ctx, template, err.Error())
		return ctrl.Result{}, err
	}
	if err = helmChart.Validate(); err != nil {
		l.Error(err, "Helm chart validation failed")
		_ = r.updateStatus(ctx, template, err.Error())
		return ctrl.Result{}, err
	}

	status.Description = helmChart.Metadata.Description
	rawValues, err := json.Marshal(helmChart.Values)
	if err != nil {
		l.Error(err, "Failed to parse Helm chart values")
		err = fmt.Errorf("failed to parse Helm chart values: %s", err)
		_ = r.updateStatus(ctx, template, err.Error())
		return ctrl.Result{}, err
	}
	status.Config = &apiextensionsv1.JSON{Raw: rawValues}
	l.Info("Chart validation completed successfully")

	return ctrl.Result{}, r.updateStatus(ctx, template, "")
}

func templateManagedByHMC(template Template) bool {
	return template.GetLabels()[hmc.HMCManagedLabelKey] == hmc.HMCManagedLabelValue
}

func parseChartMetadata(template Template, inChart *chart.Chart) error {
	if inChart.Metadata == nil {
		return fmt.Errorf("chart metadata is empty")
	}
	spec := template.GetSpec()
	status := template.GetStatus()

	// the value in spec has higher priority
	if len(spec.Providers.InfrastructureProviders) > 0 {
		status.Providers.InfrastructureProviders = spec.Providers.InfrastructureProviders
	} else {
		infraProviders := inChart.Metadata.Annotations[hmc.ChartAnnotationInfraProviders]
		if infraProviders != "" {
			status.Providers.InfrastructureProviders = strings.Split(infraProviders, ",")
		}
	}
	if len(spec.Providers.BootstrapProviders) > 0 {
		status.Providers.BootstrapProviders = spec.Providers.BootstrapProviders
	} else {
		bootstrapProviders := inChart.Metadata.Annotations[hmc.ChartAnnotationBootstrapProviders]
		if bootstrapProviders != "" {
			status.Providers.BootstrapProviders = strings.Split(bootstrapProviders, ",")
		}
	}
	if len(spec.Providers.ControlPlaneProviders) > 0 {
		status.Providers.ControlPlaneProviders = spec.Providers.ControlPlaneProviders
	} else {
		cpProviders := inChart.Metadata.Annotations[hmc.ChartAnnotationControlPlaneProviders]
		if cpProviders != "" {
			status.Providers.ControlPlaneProviders = strings.Split(cpProviders, ",")
		}
	}
	return nil
}

func (r *TemplateReconciler) updateStatus(ctx context.Context, template Template, validationError string) error {
	status := template.GetStatus()
	status.ObservedGeneration = template.GetGeneration()
	status.ValidationError = validationError
	status.Valid = validationError == ""
	err := r.Status().Update(ctx, template)
	if err != nil {
		return fmt.Errorf("failed to update status for template %s/%s: %w", template.GetNamespace(), template.GetName(), err)
	}
	return nil
}

func (r *TemplateReconciler) reconcileDefaultHelmRepository(ctx context.Context, namespace string) error {
	l := log.FromContext(ctx)
	if namespace == "" {
		namespace = r.SystemNamespace
	}
	helmRepo := &sourcev1.HelmRepository{
		ObjectMeta: metav1.ObjectMeta{
			Name:      defaultRepoName,
			Namespace: namespace,
		},
	}
	operation, err := ctrl.CreateOrUpdate(ctx, r.Client, helmRepo, func() error {
		if helmRepo.Labels == nil {
			helmRepo.Labels = make(map[string]string)
		}

		helmRepo.Labels[hmc.HMCManagedLabelKey] = hmc.HMCManagedLabelValue
		helmRepo.Spec = sourcev1.HelmRepositorySpec{
			Type:     r.DefaultRepoType,
			URL:      r.DefaultRegistryURL,
			Interval: metav1.Duration{Duration: helm.DefaultReconcileInterval},
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
	if operation == controllerutil.OperationResultCreated || operation == controllerutil.OperationResultUpdated {
		l.Info(fmt.Sprintf("Successfully %s %s/%s HelmRepository", operation, namespace, defaultRepoName))
	}
	return nil
}

func (r *TemplateReconciler) reconcileHelmChart(ctx context.Context, template Template) (*sourcev1.HelmChart, error) {
	spec := template.GetSpec()
	namespace := template.GetNamespace()
	if namespace == "" {
		namespace = r.SystemNamespace
	}
	helmChart := &sourcev1.HelmChart{
		ObjectMeta: metav1.ObjectMeta{
			Name:      template.GetName(),
			Namespace: namespace,
		},
	}

	_, err := ctrl.CreateOrUpdate(ctx, r.Client, helmChart, func() error {
		if helmChart.Labels == nil {
			helmChart.Labels = make(map[string]string)
		}
		helmChart.Labels[hmc.HMCManagedLabelKey] = hmc.HMCManagedLabelValue
		helmChart.OwnerReferences = []metav1.OwnerReference{
			{
				APIVersion: hmc.GroupVersion.String(),
				Kind:       template.GetObjectKind().GroupVersionKind().Kind,
				Name:       template.GetName(),
				UID:        template.GetUID(),
			},
		}
		helmChart.Spec = sourcev1.HelmChartSpec{
			Chart:   spec.Helm.ChartName,
			Version: spec.Helm.ChartVersion,
			SourceRef: sourcev1.LocalHelmChartSourceReference{
				Kind: sourcev1.HelmRepositoryKind,
				Name: defaultRepoName,
			},
			Interval: metav1.Duration{Duration: helm.DefaultReconcileInterval},
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return helmChart, nil
}

func (r *TemplateReconciler) getHelmChartFromChartRef(ctx context.Context, chartRef *helmcontrollerv2.CrossNamespaceSourceReference) (*sourcev1.HelmChart, error) {
	if chartRef.Kind != sourcev1.HelmChartKind {
		return nil, fmt.Errorf("invalid chartRef.Kind: %s. Only HelmChart kind is supported", chartRef.Kind)
	}
	helmChart := &sourcev1.HelmChart{}
	err := r.Get(ctx, types.NamespacedName{
		Namespace: chartRef.Namespace,
		Name:      chartRef.Name,
	}, helmChart)
	if err != nil {
		return nil, err
	}
	return helmChart, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ClusterTemplateReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&hmc.ClusterTemplate{}).
		Complete(r)
}

// SetupWithManager sets up the controller with the Manager.
func (r *ServiceTemplateReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&hmc.ServiceTemplate{}).
		Complete(r)
}

// SetupWithManager sets up the controller with the Manager.
func (r *ProviderTemplateReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&hmc.ProviderTemplate{}).
		Complete(r)
}
