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
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	texttemplate "text/template"
	"time"

	hcv2 "github.com/fluxcd/helm-controller/api/v2"
	fluxmeta "github.com/fluxcd/pkg/apis/meta"
	fluxconditions "github.com/fluxcd/pkg/runtime/conditions"
	sourcev1 "github.com/fluxcd/source-controller/api/v1"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	capz "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	capv "sigs.k8s.io/cluster-api-provider-vsphere/apis/v1beta1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/yaml"

	hmc "github.com/Mirantis/hmc/api/v1alpha1"
	"github.com/Mirantis/hmc/internal/helm"
	"github.com/Mirantis/hmc/internal/sveltos"
	"github.com/Mirantis/hmc/internal/telemetry"
	"github.com/Mirantis/hmc/internal/utils/status"
)

const (
	DefaultRequeueInterval = 10 * time.Second
)

// ManagedClusterReconciler reconciles a ManagedCluster object
type ManagedClusterReconciler struct {
	client.Client
	Config          *rest.Config
	DynamicClient   *dynamic.DynamicClient
	SystemNamespace string
}

var (
	gvkAWSCluster = schema.GroupVersionKind{
		Group:   "infrastructure.cluster.x-k8s.io",
		Version: "v1beta2",
		Kind:    "awscluster",
	}

	gvkAzureCluster = schema.GroupVersionKind{
		Group:   "infrastructure.cluster.x-k8s.io",
		Version: "v1beta1",
		Kind:    "azurecluster",
	}

	gvkMachine = schema.GroupVersionKind{
		Group:   "cluster.x-k8s.io",
		Version: "v1beta1",
		Kind:    "machine",
	}
)

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *ManagedClusterReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	l := ctrl.LoggerFrom(ctx)
	l.Info("Reconciling ManagedCluster")

	managedCluster := &hmc.ManagedCluster{}
	if err := r.Get(ctx, req.NamespacedName, managedCluster); err != nil {
		if apierrors.IsNotFound(err) {
			l.Info("ManagedCluster not found, ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}

		l.Error(err, "Failed to get ManagedCluster")
		return ctrl.Result{}, err
	}

	if !managedCluster.DeletionTimestamp.IsZero() {
		l.Info("Deleting ManagedCluster")
		return r.Delete(ctx, managedCluster)
	}

	if managedCluster.Status.ObservedGeneration == 0 {
		mgmt := &hmc.Management{}
		mgmtRef := client.ObjectKey{Name: hmc.ManagementName}
		if err := r.Get(ctx, mgmtRef, mgmt); err != nil {
			l.Error(err, "Failed to get Management object")
			return ctrl.Result{}, err
		}
		if err := telemetry.TrackManagedClusterCreate(
			string(mgmt.UID), string(managedCluster.UID), managedCluster.Spec.Template, managedCluster.Spec.DryRun); err != nil {
			l.Error(err, "Failed to track ManagedCluster creation")
		}
	}

	return r.Update(ctx, managedCluster)
}

func (r *ManagedClusterReconciler) setStatusFromClusterStatus(
	ctx context.Context, managedCluster *hmc.ManagedCluster,
) (bool, error) {
	l := ctrl.LoggerFrom(ctx)

	resourceConditions, err := status.GetResourceConditions(ctx, managedCluster.Namespace, r.DynamicClient, schema.GroupVersionResource{
		Group:    "cluster.x-k8s.io",
		Version:  "v1beta1",
		Resource: "clusters",
	}, labels.SelectorFromSet(map[string]string{hmc.FluxHelmChartNameKey: managedCluster.Name}).String())
	if err != nil {
		notFoundErr := status.ResourceNotFoundError{}
		if errors.As(err, &notFoundErr) {
			l.Info(err.Error())
			return true, nil
		}
		return false, fmt.Errorf("failed to get conditions: %w", err)
	}

	allConditionsComplete := true
	for _, metaCondition := range resourceConditions.Conditions {
		if metaCondition.Status != "True" {
			allConditionsComplete = false
		}

		if metaCondition.Reason == "" && metaCondition.Status == "True" {
			metaCondition.Reason = "Succeeded"
		}
		apimeta.SetStatusCondition(managedCluster.GetConditions(), metaCondition)
	}

	return !allConditionsComplete, nil
}

func (r *ManagedClusterReconciler) Update(ctx context.Context, managedCluster *hmc.ManagedCluster) (result ctrl.Result, err error) {
	l := ctrl.LoggerFrom(ctx)

	finalizersUpdated := controllerutil.AddFinalizer(managedCluster, hmc.ManagedClusterFinalizer)
	if finalizersUpdated {
		if err := r.Client.Update(ctx, managedCluster); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to update managedCluster %s/%s: %w", managedCluster.Namespace, managedCluster.Name, err)
		}
		return ctrl.Result{}, nil
	}

	if len(managedCluster.Status.Conditions) == 0 {
		managedCluster.InitConditions()
	}

	defer func() {
		err = errors.Join(err, r.updateStatus(ctx, managedCluster))
	}()

	template := &hmc.ClusterTemplate{}
	templateRef := client.ObjectKey{Name: managedCluster.Spec.Template, Namespace: managedCluster.Namespace}
	if err := r.Get(ctx, templateRef, template); err != nil {
		l.Error(err, "Failed to get Template")
		errMsg := fmt.Sprintf("failed to get provided template: %s", err)
		if apierrors.IsNotFound(err) {
			errMsg = "provided template is not found"
		}
		apimeta.SetStatusCondition(managedCluster.GetConditions(), metav1.Condition{
			Type:    hmc.TemplateReadyCondition,
			Status:  metav1.ConditionFalse,
			Reason:  hmc.FailedReason,
			Message: errMsg,
		})
		return ctrl.Result{}, err
	}

	if !template.Status.Valid {
		errMsg := "provided template is not marked as valid"
		apimeta.SetStatusCondition(managedCluster.GetConditions(), metav1.Condition{
			Type:    hmc.TemplateReadyCondition,
			Status:  metav1.ConditionFalse,
			Reason:  hmc.FailedReason,
			Message: errMsg,
		})
		return ctrl.Result{}, errors.New(errMsg)
	}
	// template is ok, propagate data from it
	managedCluster.Status.KubernetesVersion = template.Status.KubernetesVersion

	apimeta.SetStatusCondition(managedCluster.GetConditions(), metav1.Condition{
		Type:    hmc.TemplateReadyCondition,
		Status:  metav1.ConditionTrue,
		Reason:  hmc.SucceededReason,
		Message: "Template is valid",
	})

	source, err := r.getSource(ctx, template.Status.ChartRef)
	if err != nil {
		apimeta.SetStatusCondition(managedCluster.GetConditions(), metav1.Condition{
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
		apimeta.SetStatusCondition(managedCluster.GetConditions(), metav1.Condition{
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
	err = actionConfig.Init(getter, managedCluster.Namespace, "secret", l.Info)
	if err != nil {
		return ctrl.Result{}, err
	}

	l.Info("Validating Helm chart with provided values")
	if err := validateReleaseWithValues(ctx, actionConfig, managedCluster, hcChart); err != nil {
		apimeta.SetStatusCondition(managedCluster.GetConditions(), metav1.Condition{
			Type:    hmc.HelmChartReadyCondition,
			Status:  metav1.ConditionFalse,
			Reason:  hmc.FailedReason,
			Message: fmt.Sprintf("failed to validate template with provided configuration: %s", err),
		})
		return ctrl.Result{}, err
	}

	apimeta.SetStatusCondition(managedCluster.GetConditions(), metav1.Condition{
		Type:    hmc.HelmChartReadyCondition,
		Status:  metav1.ConditionTrue,
		Reason:  hmc.SucceededReason,
		Message: "Helm chart is valid",
	})

	cred := &hmc.Credential{}
	err = r.Client.Get(ctx, client.ObjectKey{
		Name:      managedCluster.Spec.Credential,
		Namespace: managedCluster.Namespace,
	}, cred)
	if err != nil {
		apimeta.SetStatusCondition(managedCluster.GetConditions(), metav1.Condition{
			Type:    hmc.CredentialReadyCondition,
			Status:  metav1.ConditionFalse,
			Reason:  hmc.FailedReason,
			Message: fmt.Sprintf("Failed to get Credential: %s", err),
		})
		return ctrl.Result{}, err
	}

	if cred.Status.State != hmc.CredentialReady {
		apimeta.SetStatusCondition(managedCluster.GetConditions(), metav1.Condition{
			Type:    hmc.CredentialReadyCondition,
			Status:  metav1.ConditionFalse,
			Reason:  hmc.FailedReason,
			Message: "Credential is not in Ready state",
		})
	}

	apimeta.SetStatusCondition(managedCluster.GetConditions(), metav1.Condition{
		Type:    hmc.CredentialReadyCondition,
		Status:  metav1.ConditionTrue,
		Reason:  hmc.SucceededReason,
		Message: "Credential is Ready",
	})

	if !managedCluster.Spec.DryRun {
		helmValues, err := setIdentityHelmValues(managedCluster.Spec.Config, cred.Spec.IdentityRef)
		if err != nil {
			return ctrl.Result{},
				fmt.Errorf("error setting identity values: %s", err)
		}
		hr, _, err := helm.ReconcileHelmRelease(ctx, r.Client, managedCluster.Name, managedCluster.Namespace, helm.ReconcileHelmReleaseOpts{
			Values: helmValues,
			OwnerReference: &metav1.OwnerReference{
				APIVersion: hmc.GroupVersion.String(),
				Kind:       hmc.ManagedClusterKind,
				Name:       managedCluster.Name,
				UID:        managedCluster.UID,
			},
			ChartRef: template.Status.ChartRef,
		})
		if err != nil {
			apimeta.SetStatusCondition(managedCluster.GetConditions(), metav1.Condition{
				Type:    hmc.HelmReleaseReadyCondition,
				Status:  metav1.ConditionFalse,
				Reason:  hmc.FailedReason,
				Message: err.Error(),
			})
			return ctrl.Result{}, err
		}

		hrReadyCondition := fluxconditions.Get(hr, fluxmeta.ReadyCondition)
		if hrReadyCondition != nil {
			apimeta.SetStatusCondition(managedCluster.GetConditions(), metav1.Condition{
				Type:    hmc.HelmReleaseReadyCondition,
				Status:  hrReadyCondition.Status,
				Reason:  hrReadyCondition.Reason,
				Message: hrReadyCondition.Message,
			})
		}

		requeue, err := r.setStatusFromClusterStatus(ctx, managedCluster)
		if err != nil {
			if requeue {
				return ctrl.Result{RequeueAfter: DefaultRequeueInterval}, err
			}

			return ctrl.Result{}, err
		}

		if requeue {
			return ctrl.Result{RequeueAfter: DefaultRequeueInterval}, nil
		}

		if !fluxconditions.IsReady(hr) {
			return ctrl.Result{RequeueAfter: DefaultRequeueInterval}, nil
		}

		err = r.reconcileCredentialPropagation(ctx, managedCluster)
		if err != nil {
			return ctrl.Result{}, err
		}

		return r.updateServices(ctx, managedCluster)
	}

	return ctrl.Result{}, nil
}

// updateServices reconciles services provided in ManagedCluster.Spec.Services.
// TODO(https://github.com/Mirantis/hmc/issues/361): Set status to ManagedCluster object at appropriate places.
func (r *ManagedClusterReconciler) updateServices(ctx context.Context, mc *hmc.ManagedCluster) (ctrl.Result, error) {
	opts, err := helmChartOpts(ctx, r.Client, mc.Namespace, mc.Spec.Services)
	if err != nil {
		return ctrl.Result{}, err
	}

	if _, err := sveltos.ReconcileProfile(ctx, r.Client, mc.Namespace, mc.Name,
		sveltos.ReconcileProfileOpts{
			OwnerReference: &metav1.OwnerReference{
				APIVersion: hmc.GroupVersion.String(),
				Kind:       hmc.ManagedClusterKind,
				Name:       mc.Name,
				UID:        mc.UID,
			},
			LabelSelector: metav1.LabelSelector{
				MatchLabels: map[string]string{
					hmc.FluxHelmChartNamespaceKey: mc.Namespace,
					hmc.FluxHelmChartNameKey:      mc.Name,
				},
			},
			HelmChartOpts:  opts,
			Priority:       mc.Spec.ServicesPriority,
			StopOnConflict: mc.Spec.StopOnConflict,
		}); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to reconcile Profile: %w", err)
	}

	// We don't technically need to requeue here, but doing so because golint fails with:
	// `(*ManagedClusterReconciler).updateServices` - result `res` is always `nil` (unparam)
	//
	// This will be automatically resolved once setting status is implemented (https://github.com/Mirantis/hmc/issues/361),
	// as it is likely that some execution path in the function will have to return with a requeue to fetch latest status.
	return ctrl.Result{RequeueAfter: DefaultRequeueInterval}, nil
}

func validateReleaseWithValues(ctx context.Context, actionConfig *action.Configuration, managedCluster *hmc.ManagedCluster, hcChart *chart.Chart) error {
	install := action.NewInstall(actionConfig)
	install.DryRun = true
	install.ReleaseName = managedCluster.Name
	install.Namespace = managedCluster.Namespace
	install.ClientOnly = true

	vals, err := managedCluster.HelmValues()
	if err != nil {
		return err
	}
	_, err = install.RunWithContext(ctx, hcChart, vals)
	if err != nil {
		return err
	}
	return nil
}

func (r *ManagedClusterReconciler) updateStatus(ctx context.Context, managedCluster *hmc.ManagedCluster) error {
	managedCluster.Status.ObservedGeneration = managedCluster.Generation
	warnings := ""
	errs := ""
	for _, condition := range managedCluster.Status.Conditions {
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
		Message: "ManagedCluster is ready",
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
	apimeta.SetStatusCondition(managedCluster.GetConditions(), condition)
	if err := r.Status().Update(ctx, managedCluster); err != nil {
		return fmt.Errorf("failed to update status for managedCluster %s/%s: %w", managedCluster.Namespace, managedCluster.Name, err)
	}
	return nil
}

func (r *ManagedClusterReconciler) getSource(ctx context.Context, ref *hcv2.CrossNamespaceSourceReference) (sourcev1.Source, error) {
	if ref == nil {
		return nil, fmt.Errorf("helm chart source is not provided")
	}
	chartRef := client.ObjectKey{Namespace: ref.Namespace, Name: ref.Name}
	hc := sourcev1.HelmChart{}
	if err := r.Client.Get(ctx, chartRef, &hc); err != nil {
		return nil, err
	}
	return &hc, nil
}

func (r *ManagedClusterReconciler) Delete(ctx context.Context, managedCluster *hmc.ManagedCluster) (ctrl.Result, error) {
	l := ctrl.LoggerFrom(ctx)

	hr := &hcv2.HelmRelease{}
	err := r.Get(ctx, client.ObjectKey{
		Name:      managedCluster.Name,
		Namespace: managedCluster.Namespace,
	}, hr)
	if err != nil {
		if apierrors.IsNotFound(err) {
			l.Info("Removing Finalizer", "finalizer", hmc.ManagedClusterFinalizer)
			finalizersUpdated := controllerutil.RemoveFinalizer(managedCluster, hmc.ManagedClusterFinalizer)
			if finalizersUpdated {
				if err := r.Client.Update(ctx, managedCluster); err != nil {
					return ctrl.Result{}, fmt.Errorf("failed to update managedCluster %s/%s: %w", managedCluster.Namespace, managedCluster.Name, err)
				}
			}
			l.Info("ManagedCluster deleted")
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	err = helm.DeleteHelmRelease(ctx, r.Client, managedCluster.Name, managedCluster.Namespace)
	if err != nil {
		return ctrl.Result{}, err
	}

	// Without explicitly deleting the Profile object, we run into a race condition
	// which prevents Sveltos objects from being removed from the management cluster.
	// It is detailed in https://github.com/projectsveltos/addon-controller/issues/732.
	// We may try to remove the explicit call to Delete once a fix for it has been merged.
	// TODO(https://github.com/Mirantis/hmc/issues/526).
	err = sveltos.DeleteProfile(ctx, r.Client, managedCluster.Namespace, managedCluster.Name)
	if err != nil {
		return ctrl.Result{}, err
	}

	err = r.releaseCluster(ctx, managedCluster.Namespace, managedCluster.Name, managedCluster.Spec.Template)
	if err != nil {
		return ctrl.Result{}, err
	}

	l.Info("HelmRelease still exists, retrying")
	return ctrl.Result{RequeueAfter: DefaultRequeueInterval}, nil
}

func (r *ManagedClusterReconciler) releaseCluster(ctx context.Context, namespace, name, templateName string) error {
	providers, err := r.getProviders(ctx, namespace, templateName)
	if err != nil {
		return err
	}

	for _, provider := range providers.BootstrapProviders {
		if provider.Name == "eks" {
			// no need to do anything for EKS clusters
			return nil
		}
	}

	providerGVKs := map[string]schema.GroupVersionKind{
		"aws":   gvkAWSCluster,
		"azure": gvkAzureCluster,
	}

	// Associate the provider with it's GVK
	for _, provider := range providers.InfrastructureProviders {
		gvk, ok := providerGVKs[provider.Name]
		if !ok {
			continue
		}

		cluster, err := r.getCluster(ctx, namespace, name, gvk)
		if err != nil {
			return err
		}

		found, err := r.objectsAvailable(ctx, namespace, cluster.Name, gvkMachine)
		if err != nil {
			return err
		}

		if !found {
			return r.removeClusterFinalizer(ctx, cluster)
		}
	}

	return nil
}

func (r *ManagedClusterReconciler) getProviders(ctx context.Context, templateNamespace, templateName string) (hmc.ProvidersTupled, error) {
	template := &hmc.ClusterTemplate{}
	templateRef := client.ObjectKey{Name: templateName, Namespace: templateNamespace}
	if err := r.Get(ctx, templateRef, template); err != nil {
		ctrl.LoggerFrom(ctx).Error(err, "Failed to get ClusterTemplate", "template namespace", templateNamespace, "template name", templateName)
		return hmc.ProvidersTupled{}, err
	}

	return template.Status.Providers, nil
}

func (r *ManagedClusterReconciler) getCluster(ctx context.Context, namespace, name string, gvk schema.GroupVersionKind) (*metav1.PartialObjectMetadata, error) {
	opts := &client.ListOptions{
		LabelSelector: labels.SelectorFromSet(map[string]string{hmc.FluxHelmChartNameKey: name}),
		Namespace:     namespace,
	}
	itemsList := &metav1.PartialObjectMetadataList{}
	itemsList.SetGroupVersionKind(gvk)
	if err := r.Client.List(ctx, itemsList, opts); err != nil {
		return nil, err
	}
	if len(itemsList.Items) == 0 {
		return nil, fmt.Errorf("%s with name %s was not found", gvk.Kind, name)
	}

	return &itemsList.Items[0], nil
}

func (r *ManagedClusterReconciler) removeClusterFinalizer(ctx context.Context, cluster *metav1.PartialObjectMetadata) error {
	originalCluster := *cluster
	finalizersUpdated := controllerutil.RemoveFinalizer(cluster, hmc.BlockingFinalizer)
	if finalizersUpdated {
		ctrl.LoggerFrom(ctx).Info("Allow to stop cluster", "finalizer", hmc.BlockingFinalizer)
		if err := r.Client.Patch(ctx, cluster, client.MergeFrom(&originalCluster)); err != nil {
			return fmt.Errorf("failed to patch cluster %s/%s: %w", cluster.Namespace, cluster.Name, err)
		}
	}

	return nil
}

func (r *ManagedClusterReconciler) objectsAvailable(ctx context.Context, namespace, clusterName string, gvk schema.GroupVersionKind) (bool, error) {
	opts := &client.ListOptions{
		LabelSelector: labels.SelectorFromSet(map[string]string{hmc.ClusterNameLabelKey: clusterName}),
		Namespace:     namespace,
		Limit:         1,
	}
	itemsList := &metav1.PartialObjectMetadataList{}
	itemsList.SetGroupVersionKind(gvk)
	if err := r.Client.List(ctx, itemsList, opts); err != nil {
		return false, err
	}
	return len(itemsList.Items) != 0, nil
}

func (r *ManagedClusterReconciler) reconcileCredentialPropagation(ctx context.Context, managedCluster *hmc.ManagedCluster) error {
	l := ctrl.LoggerFrom(ctx)
	l.Info("Reconciling CCM credentials propagation")

	providers, err := r.getProviders(ctx, managedCluster.Namespace, managedCluster.Spec.Template)
	if err != nil {
		return fmt.Errorf("failed to get cluster providers for cluster %s/%s: %s", managedCluster.Namespace, managedCluster.Name, err)
	}

	kubeconfSecret := &corev1.Secret{}
	if err := r.Client.Get(ctx, client.ObjectKey{
		Name:      fmt.Sprintf("%s-kubeconfig", managedCluster.Name),
		Namespace: managedCluster.Namespace,
	}, kubeconfSecret); err != nil {
		return fmt.Errorf("failed to get kubeconfig secret for cluster %s/%s: %s", managedCluster.Namespace, managedCluster.Name, err)
	}

	for _, provider := range providers.InfrastructureProviders {
		switch provider.Name {
		case "aws":
			l.Info("Skipping creds propagation for AWS")
			continue
		case "azure":
			l.Info("Azure creds propagation start")
			err := r.propagateAzureSecrets(ctx, managedCluster, kubeconfSecret)
			if err != nil {
				errMsg := fmt.Sprintf("failed to create Azure CCM credentials: %s", err)
				apimeta.SetStatusCondition(managedCluster.GetConditions(), metav1.Condition{
					Type:    hmc.CredentialsPropagatedCondition,
					Status:  metav1.ConditionFalse,
					Reason:  hmc.FailedReason,
					Message: errMsg,
				})
				return errors.New(errMsg)
			}
			apimeta.SetStatusCondition(managedCluster.GetConditions(), metav1.Condition{
				Type:    hmc.CredentialsPropagatedCondition,
				Status:  metav1.ConditionTrue,
				Reason:  hmc.SucceededReason,
				Message: "Azure CCM credentials created",
			})
			continue
		case "vsphere":
			l.Info("vSphere creds propagation start")
			err := r.propagateVSphereSecrets(ctx, managedCluster, kubeconfSecret)
			if err != nil {
				errMsg := fmt.Sprintf("failed to create vSphere CCM credentials: %s", err)
				apimeta.SetStatusCondition(managedCluster.GetConditions(), metav1.Condition{
					Type:    hmc.CredentialsPropagatedCondition,
					Status:  metav1.ConditionFalse,
					Reason:  hmc.FailedReason,
					Message: errMsg,
				})
				return errors.New(errMsg)
			}
			apimeta.SetStatusCondition(managedCluster.GetConditions(), metav1.Condition{
				Type:    hmc.CredentialsPropagatedCondition,
				Status:  metav1.ConditionTrue,
				Reason:  hmc.SucceededReason,
				Message: "vSphere CCM credentials created",
			})
			continue
		default:
			errMsg := fmt.Sprintf("unsupported infrastructure provider %s", provider)
			apimeta.SetStatusCondition(managedCluster.GetConditions(), metav1.Condition{
				Type:    hmc.CredentialsPropagatedCondition,
				Status:  metav1.ConditionFalse,
				Reason:  hmc.FailedReason,
				Message: errMsg,
			})
			continue
		}
	}
	l.Info("CCM credentials reconcile finished")
	return nil
}

func (r *ManagedClusterReconciler) propagateAzureSecrets(ctx context.Context, managedCluster *hmc.ManagedCluster, kubeconfSecret *corev1.Secret) error {
	azureCluster := &capz.AzureCluster{}
	if err := r.Client.Get(ctx, client.ObjectKey{
		Name:      managedCluster.Name,
		Namespace: managedCluster.Namespace,
	}, azureCluster); err != nil {
		return fmt.Errorf("failed to get AzureCluster %s: %s", managedCluster.Name, err)
	}

	azureClIdty := &capz.AzureClusterIdentity{}
	if err := r.Client.Get(ctx, client.ObjectKey{
		Name:      azureCluster.Spec.IdentityRef.Name,
		Namespace: azureCluster.Spec.IdentityRef.Namespace,
	}, azureClIdty); err != nil {
		return fmt.Errorf("failed to get AzureClusterIdentity %s: %s", azureCluster.Spec.IdentityRef.Name, err)
	}

	azureSecret := &corev1.Secret{}
	if err := r.Client.Get(ctx, client.ObjectKey{
		Name:      azureClIdty.Spec.ClientSecret.Name,
		Namespace: azureClIdty.Spec.ClientSecret.Namespace,
	}, azureSecret); err != nil {
		return fmt.Errorf("failed to get azure Secret %s: %s", azureClIdty.Spec.ClientSecret.Name, err)
	}

	ccmSecret, err := generateAzureCCMSecret(azureCluster, azureClIdty, azureSecret)
	if err != nil {
		return fmt.Errorf("failed to generate Azure CCM secret: %s", err)
	}

	if err := applyCCMConfigs(ctx, kubeconfSecret, ccmSecret); err != nil {
		return fmt.Errorf("failed to apply Azure CCM secret: %s", err)
	}

	return nil
}

func generateAzureCCMSecret(azureCluster *capz.AzureCluster, azureClIdty *capz.AzureClusterIdentity, azureSecret *corev1.Secret) (*corev1.Secret, error) {
	azureJSONMap := map[string]any{
		"cloud":                        azureCluster.Spec.AzureEnvironment,
		"tenantId":                     azureClIdty.Spec.TenantID,
		"subscriptionId":               azureCluster.Spec.SubscriptionID,
		"aadClientId":                  azureClIdty.Spec.ClientID,
		"aadClientSecret":              string(azureSecret.Data["clientSecret"]),
		"resourceGroup":                azureCluster.Spec.ResourceGroup,
		"securityGroupName":            azureCluster.Spec.NetworkSpec.Subnets[0].SecurityGroup.Name,
		"securityGroupResourceGroup":   azureCluster.Spec.NetworkSpec.Vnet.ResourceGroup,
		"location":                     azureCluster.Spec.Location,
		"vmType":                       "vmss",
		"vnetName":                     azureCluster.Spec.NetworkSpec.Vnet.Name,
		"vnetResourceGroup":            azureCluster.Spec.NetworkSpec.Vnet.ResourceGroup,
		"subnetName":                   azureCluster.Spec.NetworkSpec.Subnets[0].Name,
		"loadBalancerSku":              "Standard",
		"loadBalancerName":             "",
		"maximumLoadBalancerRuleCount": 250,
		"useManagedIdentityExtension":  false,
		"useInstanceMetadata":          true,
	}
	azureJSON, err := json.Marshal(azureJSONMap)
	if err != nil {
		return nil, fmt.Errorf("error marshalling azure.json: %s", err)
	}

	secretData := map[string][]byte{
		"cloud-config": azureJSON,
	}

	return makeSecret("azure-cloud-provider", metav1.NamespaceSystem, secretData), nil
}

func (r *ManagedClusterReconciler) propagateVSphereSecrets(ctx context.Context, managedCluster *hmc.ManagedCluster, kubeconfSecret *corev1.Secret) error {
	vsphereCluster := &capv.VSphereCluster{}
	if err := r.Client.Get(ctx, client.ObjectKey{
		Name:      managedCluster.Name,
		Namespace: managedCluster.Namespace,
	}, vsphereCluster); err != nil {
		return fmt.Errorf("failed to get VSphereCluster %s: %s", managedCluster.Name, err)
	}

	vsphereClIdty := &capv.VSphereClusterIdentity{}
	if err := r.Client.Get(ctx, client.ObjectKey{
		Name: vsphereCluster.Spec.IdentityRef.Name,
	}, vsphereClIdty); err != nil {
		return fmt.Errorf("failed to get VSphereClusterIdentity %s: %s", vsphereCluster.Spec.IdentityRef.Name, err)
	}

	vsphereSecret := &corev1.Secret{}
	if err := r.Client.Get(ctx, client.ObjectKey{
		Name:      vsphereClIdty.Spec.SecretName,
		Namespace: r.SystemNamespace,
	}, vsphereSecret); err != nil {
		return fmt.Errorf("failed to get VSphere Secret %s: %s", vsphereClIdty.Spec.SecretName, err)
	}

	vsphereMachines := &capv.VSphereMachineList{}
	if err := r.Client.List(
		ctx,
		vsphereMachines,
		&client.ListOptions{
			Namespace: managedCluster.Namespace,
			LabelSelector: labels.SelectorFromSet(map[string]string{
				hmc.ClusterNameLabelKey: managedCluster.Name,
			}),
			Limit: 1,
		},
	); err != nil {
		return fmt.Errorf("failed to list VSphereMachines for cluster %s: %s", managedCluster.Name, err)
	}
	ccmSecret, ccmConfig, err := generateVSphereCCMConfigs(vsphereCluster, vsphereSecret, &vsphereMachines.Items[0])
	if err != nil {
		return fmt.Errorf("failed to generate VSphere CCM config: %s", err)
	}
	csiSecret, err := generateVSphereCSISecret(vsphereCluster, vsphereSecret, &vsphereMachines.Items[0])
	if err != nil {
		return fmt.Errorf("failed to generate VSphere CSI secret: %s", err)
	}

	if err := applyCCMConfigs(ctx, kubeconfSecret, ccmSecret, ccmConfig, csiSecret); err != nil {
		return fmt.Errorf("failed to apply VSphere CCM/CSI secrets: %s", err)
	}

	return nil
}

func generateVSphereCCMConfigs(vCl *capv.VSphereCluster, vScrt *corev1.Secret, vMa *capv.VSphereMachine) (*corev1.Secret, *corev1.ConfigMap, error) {
	secretName := "vsphere-cloud-secret"
	secretData := map[string][]byte{
		fmt.Sprintf("%s.username", vCl.Spec.Server): vScrt.Data["username"],
		fmt.Sprintf("%s.password", vCl.Spec.Server): vScrt.Data["password"],
	}
	ccmCfg := map[string]any{
		"global": map[string]any{
			"port":            443,
			"insecureFlag":    true,
			"secretName":      secretName,
			"secretNamespace": metav1.NamespaceSystem,
		},
		"vcenter": map[string]any{
			vCl.Spec.Server: map[string]any{
				"server": vCl.Spec.Server,
				"datacenters": []string{
					vMa.Spec.Datacenter,
				},
			},
		},
		"labels": map[string]any{
			"region": "k8s-region",
			"zone":   "k8s-zone",
		},
	}

	ccmCfgYaml, err := yaml.Marshal(ccmCfg)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to marshal CCM config: %s", err)
	}

	cmData := map[string]string{
		"vsphere.conf": string(ccmCfgYaml),
	}
	return makeSecret(secretName, metav1.NamespaceSystem, secretData),
		makeConfigMap("cloud-config", metav1.NamespaceSystem, cmData),
		nil
}

func generateVSphereCSISecret(vCl *capv.VSphereCluster, vScrt *corev1.Secret, vMa *capv.VSphereMachine) (*corev1.Secret, error) {
	csiCfg := `
[Global]
cluster-id = "{{ .ClusterID }}"

[VirtualCenter "{{ .Vcenter }}"]
insecure-flag = "true"
user = "{{ .Username }}"
password = "{{ .Password }}"
port = "443"
datacenters = "{{ .Datacenter }}"
`
	type CSIFields struct {
		ClusterID, Vcenter, Username, Password, Datacenter string
	}

	fields := CSIFields{
		ClusterID:  vCl.Name,
		Vcenter:    vCl.Spec.Server,
		Username:   string(vScrt.Data["username"]),
		Password:   string(vScrt.Data["password"]),
		Datacenter: vMa.Spec.Datacenter,
	}

	tmpl, err := texttemplate.New("csiCfg").Parse(csiCfg)
	if err != nil {
		return nil, fmt.Errorf("failed to generate CSI secret (tmpl parse): %s", err)
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, fields); err != nil {
		return nil, fmt.Errorf("failed to generate CSI secret (tmpl execute): %s", err)
	}

	secretData := map[string][]byte{
		"csi-vsphere.conf": buf.Bytes(),
	}

	return makeSecret("vcenter-config-secret", metav1.NamespaceSystem, secretData), nil
}

func applyCCMConfigs(ctx context.Context, kubeconfSecret *corev1.Secret, objects ...client.Object) error {
	clnt, err := makeClientFromSecret(kubeconfSecret)
	if err != nil {
		return fmt.Errorf("failed to create k8s client: %s", err)
	}
	for _, object := range objects {
		if err := clnt.Patch(
			ctx,
			object,
			client.Apply,
			client.FieldOwner("hmc-controller"),
		); err != nil {
			return fmt.Errorf("failed to apply CCM config object %s: %s", object.GetName(), err)
		}
	}
	return nil
}

func makeSecret(name, namespace string, data map[string][]byte) *corev1.Secret {
	s := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Data: data,
	}
	s.SetGroupVersionKind(corev1.SchemeGroupVersion.WithKind("Secret"))
	return s
}

func makeConfigMap(name, namespace string, data map[string]string) *corev1.ConfigMap {
	c := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Data: data,
	}
	c.SetGroupVersionKind(corev1.SchemeGroupVersion.WithKind("ConfigMap"))
	return c
}

func makeClientFromSecret(kubeconfSecret *corev1.Secret) (client.Client, error) {
	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		return nil, err
	}
	restConfig, err := clientcmd.RESTConfigFromKubeConfig(kubeconfSecret.Data["value"])
	if err != nil {
		return nil, err
	}
	cl, err := client.New(restConfig, client.Options{
		Scheme: scheme,
	})
	if err != nil {
		return nil, err
	}
	return cl, nil
}

func setIdentityHelmValues(values *apiextensionsv1.JSON, idRef *corev1.ObjectReference) (*apiextensionsv1.JSON, error) {
	var valuesJSON map[string]any
	err := json.Unmarshal(values.Raw, &valuesJSON)
	if err != nil {
		return nil, fmt.Errorf("error unmarshalling values: %s", err)
	}

	valuesJSON["clusterIdentity"] = idRef
	valuesRaw, err := json.Marshal(valuesJSON)
	if err != nil {
		return nil, fmt.Errorf("error marshalling values: %s", err)
	}

	return &apiextensionsv1.JSON{
		Raw: valuesRaw,
	}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ManagedClusterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&hmc.ManagedCluster{}).
		Watches(&hcv2.HelmRelease{},
			handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, o client.Object) []ctrl.Request {
				managedCluster := hmc.ManagedCluster{}
				managedClusterRef := client.ObjectKey{
					Namespace: o.GetNamespace(),
					Name:      o.GetName(),
				}
				err := r.Client.Get(ctx, managedClusterRef, &managedCluster)
				if err != nil {
					return []ctrl.Request{}
				}
				return []reconcile.Request{
					{
						NamespacedName: managedClusterRef,
					},
				}
			}),
		).
		Complete(r)
}
