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
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/Mirantis/hmc/api/v1alpha1"
	"github.com/Mirantis/hmc/internal/templateutil"
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
	deletionAllowed, err := v.isTemplateDeletionAllowed(ctx, template)
	if err != nil {
		return nil, fmt.Errorf("failed to check if the ClusterTemplate %s/%s is allowed to be deleted: %v", template.Namespace, template.Name, err)
	}
	if !deletionAllowed {
		return nil, errTemplateDeletionForbidden
	}

	managedClusters := &v1alpha1.ManagedClusterList{}
	listOptions := client.ListOptions{
		FieldSelector: fields.SelectorFromSet(fields.Set{v1alpha1.TemplateKey: template.Name}),
		Limit:         1,
		Namespace:     template.Namespace,
	}
	err = v.Client.List(ctx, managedClusters, &listOptions)
	if err != nil {
		return nil, err
	}

	if len(managedClusters.Items) > 0 {
		return admission.Warnings{"The ClusterTemplate object can't be removed if ManagedCluster objects referencing it still exist"}, errTemplateDeletionForbidden
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
	deletionAllowed, err := v.isTemplateDeletionAllowed(ctx, template)
	if err != nil {
		return nil, fmt.Errorf("failed to check if the ServiceTemplate %s/%s is allowed to be deleted: %v", template.Namespace, template.Name, err)
	}
	if !deletionAllowed {
		return nil, errTemplateDeletionForbidden
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
	deletionAllowed, err := v.isTemplateDeletionAllowed(ctx, template)
	if err != nil {
		return nil, fmt.Errorf("failed to check if the ProviderTemplate %s is allowed to be deleted: %v", template.Name, err)
	}
	if !deletionAllowed {
		return nil, errTemplateDeletionForbidden
	}
	return nil, nil
}

// Default implements webhook.Defaulter so a webhook will be registered for the type.
func (*ProviderTemplateValidator) Default(_ context.Context, _ runtime.Object) error {
	return nil
}

func (v TemplateValidator) isTemplateDeletionAllowed(ctx context.Context, template templateutil.Template) (bool, error) {
	req, err := admission.RequestFromContext(ctx)
	if err != nil {
		return false, err
	}
	if v.InjectUserInfo != nil {
		v.InjectUserInfo(&req)
	}
	// Allow all templates' deletion for the HMC controller
	if serviceAccountIsEqual(req, os.Getenv(ServiceAccountEnvName)) {
		return true, nil
	}
	// Cluster-scoped ProviderTemplates and Templates from the system namespace are not allowed to be deleted
	if template.GetNamespace() == "" || template.GetNamespace() == v.SystemNamespace {
		return false, nil
	}
	// Forbid template deletion if the template is managed by the TemplateManagement
	if templateutil.IsManagedByHMC(template) {
		return false, nil
	}
	return true, nil
}
