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

package webhook

import (
	"context"
	"errors"
	"fmt"
	"os"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/Mirantis/hmc/api/v1alpha1"
)

var errTemplateDeletionForbidden = errors.New("template deletion is forbidden")

type TemplateValidator struct {
	client.Client

	SystemNamespace string
	InjectUserInfo  func(*admission.Request)
}

type ClusterTemplateValidator struct {
	TemplateValidator
}

func (v *ClusterTemplateValidator) SetupWebhookWithManager(mgr ctrl.Manager) error {
	v.Client = mgr.GetClient()
	return ctrl.NewWebhookManagedBy(mgr).
		For(&v1alpha1.ClusterTemplate{}).
		WithValidator(v).
		WithDefaulter(v).
		Complete()
}

var (
	_ webhook.CustomValidator = &ClusterTemplateValidator{}
	_ webhook.CustomDefaulter = &ClusterTemplateValidator{}
)

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type.
func (*ClusterTemplateValidator) ValidateCreate(_ context.Context, _ runtime.Object) (admission.Warnings, error) {
	return nil, nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type.
func (*ClusterTemplateValidator) ValidateUpdate(_ context.Context, _ runtime.Object, _ runtime.Object) (admission.Warnings, error) {
	return nil, nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type.
func (v *ClusterTemplateValidator) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	template, ok := obj.(*v1alpha1.ClusterTemplate)
	if !ok {
		return admission.Warnings{"Wrong object"}, apierrors.NewBadRequest(fmt.Sprintf("expected ClusterTemplate but got a %T", obj))
	}
	deletionAllowed, warnings, err := v.isTemplateDeletionAllowed(ctx, template)
	if err != nil {
		return nil, fmt.Errorf("failed to check if the ClusterTemplate %s/%s is allowed to be deleted: %v", template.Namespace, template.Name, err)
	}
	if !deletionAllowed {
		return warnings, errTemplateDeletionForbidden
	}

	return nil, nil
}

// Default implements webhook.Defaulter so a webhook will be registered for the type.
func (*ClusterTemplateValidator) Default(context.Context, runtime.Object) error {
	return nil
}

type ServiceTemplateValidator struct {
	TemplateValidator
}

func (v *ServiceTemplateValidator) SetupWebhookWithManager(mgr ctrl.Manager) error {
	v.Client = mgr.GetClient()
	return ctrl.NewWebhookManagedBy(mgr).
		For(&v1alpha1.ServiceTemplate{}).
		WithValidator(v).
		WithDefaulter(v).
		Complete()
}

var (
	_ webhook.CustomValidator = &ServiceTemplateValidator{}
	_ webhook.CustomDefaulter = &ServiceTemplateValidator{}
)

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type.
func (*ServiceTemplateValidator) ValidateCreate(_ context.Context, _ runtime.Object) (admission.Warnings, error) {
	return nil, nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type.
func (*ServiceTemplateValidator) ValidateUpdate(_ context.Context, _ runtime.Object, _ runtime.Object) (admission.Warnings, error) {
	return nil, nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type.
func (v *ServiceTemplateValidator) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	template, ok := obj.(*v1alpha1.ServiceTemplate)
	if !ok {
		return admission.Warnings{"Wrong object"}, apierrors.NewBadRequest(fmt.Sprintf("expected ServiceTemplate but got a %T", obj))
	}

	deletionAllowed, warnings, err := v.isTemplateDeletionAllowed(ctx, template)
	if err != nil {
		return nil, fmt.Errorf("failed to check if the ServiceTemplate %s/%s is allowed to be deleted: %v", template.Namespace, template.Name, err)
	}
	if !deletionAllowed {
		return warnings, errTemplateDeletionForbidden
	}

	return nil, nil
}

// Default implements webhook.Defaulter so a webhook will be registered for the type.
func (*ServiceTemplateValidator) Default(_ context.Context, _ runtime.Object) error {
	return nil
}

type ProviderTemplateValidator struct {
	TemplateValidator
}

func (v *ProviderTemplateValidator) SetupWebhookWithManager(mgr ctrl.Manager) error {
	v.Client = mgr.GetClient()
	return ctrl.NewWebhookManagedBy(mgr).
		For(&v1alpha1.ProviderTemplate{}).
		WithValidator(v).
		WithDefaulter(v).
		Complete()
}

var (
	_ webhook.CustomValidator = &ProviderTemplateValidator{}
	_ webhook.CustomDefaulter = &ProviderTemplateValidator{}
)

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type.
func (*ProviderTemplateValidator) ValidateCreate(_ context.Context, _ runtime.Object) (admission.Warnings, error) {
	return nil, nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type.
func (*ProviderTemplateValidator) ValidateUpdate(_ context.Context, _ runtime.Object, _ runtime.Object) (admission.Warnings, error) {
	return nil, nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type.
func (v *ProviderTemplateValidator) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	template, ok := obj.(*v1alpha1.ProviderTemplate)
	if !ok {
		return admission.Warnings{"Wrong object"}, apierrors.NewBadRequest(fmt.Sprintf("expected ProviderTemplate but got a %T", obj))
	}
	deletionAllowed, warnings, err := v.isTemplateDeletionAllowed(ctx, template)
	if err != nil {
		return nil, fmt.Errorf("failed to check if the ProviderTemplate %s is allowed to be deleted: %v", template.Name, err)
	}
	if !deletionAllowed {
		return warnings, errTemplateDeletionForbidden
	}
	return nil, nil
}

// Default implements webhook.Defaulter so a webhook will be registered for the type.
func (*ProviderTemplateValidator) Default(_ context.Context, _ runtime.Object) error {
	return nil
}

func (v TemplateValidator) isTemplateDeletionAllowed(ctx context.Context, template client.Object) (bool, admission.Warnings, error) {
	req, err := admission.RequestFromContext(ctx)
	if err != nil {
		return false, nil, err
	}
	if v.InjectUserInfo != nil {
		v.InjectUserInfo(&req)
	}

	triggeredByController := false
	if serviceAccountIsEqual(req, os.Getenv(ServiceAccountEnvName)) {
		triggeredByController = true
	}

	// Forbid template deletion if the template is managed by the TemplateManagement
	if !triggeredByController && templateManagedByHMC(template) {
		return false, nil, nil
	}

	// Forbid template deletion if it's in use by cluster or by chain
	kind := template.GetObjectKind().GroupVersionKind().Kind
	if kind == v1alpha1.ClusterTemplateKind || kind == v1alpha1.ServiceTemplateKind {
		inUseByCluster, err := v.templateInUseByCluster(ctx, template)
		if err != nil {
			return false, nil, err
		}
		if inUseByCluster {
			return false, admission.Warnings{fmt.Sprintf("The %s object can't be removed if ManagedCluster objects referencing it still exist", kind)}, nil
		}
		inUseByChain, err := v.templateInUseByTemplateChain(ctx, template)
		if err != nil {
			return false, nil, err
		}
		if inUseByChain {
			return false, admission.Warnings{fmt.Sprintf("The %s object can't be removed if %s object referencing it exists", kind, getTemplateChainKind(template))}, nil
		}
	}
	return true, nil, nil
}

func (v TemplateValidator) templateInUseByCluster(ctx context.Context, template client.Object) (bool, error) {
	var key string
	kind := template.GetObjectKind().GroupVersionKind().Kind

	if kind == v1alpha1.ClusterTemplateKind {
		key = v1alpha1.TemplateKey
	}
	if kind == v1alpha1.ServiceTemplateKind {
		key = v1alpha1.ServicesTemplateKey
	}

	managedClusters := &v1alpha1.ManagedClusterList{}
	if err := v.Client.List(ctx, managedClusters,
		client.InNamespace(template.GetNamespace()),
		client.MatchingFields{key: template.GetName()},
		client.Limit(1)); err != nil {
		return false, err
	}
	if len(managedClusters.Items) > 0 {
		return true, nil
	}
	return false, nil
}

func (v TemplateValidator) templateInUseByTemplateChain(ctx context.Context, template client.Object) (bool, error) {
	listOpts := []client.ListOption{
		client.InNamespace(template.GetNamespace()),
		client.MatchingFields{v1alpha1.SupportedTemplateKey: template.GetName()},
		client.Limit(1),
	}
	templateChainKind := getTemplateChainKind(template)
	if templateChainKind == v1alpha1.ClusterTemplateChainKind {
		chainList := &v1alpha1.ClusterTemplateChainList{}
		if err := v.Client.List(ctx, chainList, listOpts...); err != nil {
			return false, err
		}
		if len(chainList.Items) > 0 {
			return true, nil
		}
	}
	if templateChainKind == v1alpha1.ServiceTemplateChainKind {
		chainList := &v1alpha1.ServiceTemplateChainList{}
		if err := v.Client.List(ctx, chainList, listOpts...); err != nil {
			return false, err
		}
		if len(chainList.Items) > 0 {
			return true, nil
		}
	}
	return false, nil
}

func templateManagedByHMC(template client.Object) bool {
	return template.GetLabels()[v1alpha1.HMCManagedLabelKey] == v1alpha1.HMCManagedLabelValue
}

func getTemplateChainKind(template client.Object) string {
	kind := template.GetObjectKind().GroupVersionKind().Kind
	if kind == v1alpha1.ClusterTemplateKind {
		return v1alpha1.ClusterTemplateChainKind
	}
	if kind == v1alpha1.ServiceTemplateKind {
		return v1alpha1.ServiceTemplateChainKind
	}
	return ""
}
