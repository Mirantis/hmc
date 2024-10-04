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
	"slices"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/Mirantis/hmc/api/v1alpha1"
)

type ClusterTemplateValidator struct {
	client.Client
}

var errTemplateDeletionForbidden = errors.New("template deletion is forbidden")

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

	managedClusters := &v1alpha1.ManagedClusterList{}
	listOptions := client.ListOptions{
		FieldSelector: fields.SelectorFromSet(fields.Set{v1alpha1.TemplateKey: template.Name}),
		Limit:         1,
		Namespace:     template.Namespace,
	}
	err := v.Client.List(ctx, managedClusters, &listOptions)
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
	client.Client
}

func (in *ServiceTemplateValidator) SetupWebhookWithManager(mgr ctrl.Manager) error {
	in.Client = mgr.GetClient()
	return ctrl.NewWebhookManagedBy(mgr).
		For(&v1alpha1.ServiceTemplate{}).
		WithValidator(in).
		WithDefaulter(in).
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
func (in *ServiceTemplateValidator) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	tmpl, ok := obj.(*v1alpha1.ServiceTemplate)
	if !ok {
		return admission.Warnings{"Wrong object"}, apierrors.NewBadRequest(fmt.Sprintf("expected ServiceTemplate but got a %T", obj))
	}

	// TODO: Find a way to only select the ManagedClusters that have
	// .Spec.Services[].Template == tmpl.Name using ListOptions if possible.
	// This way we can get rid of the pagination as well.
	var token string
	for {
		managedClusters := &v1alpha1.ManagedClusterList{}
		if err := in.Client.List(ctx, managedClusters, &client.ListOptions{
			Namespace: tmpl.Namespace,
			Limit:     100, // arbitrary page size.
			Raw: &metav1.ListOptions{
				Continue: token,
			},
		}); err != nil {
			return nil, err
		}

		for _, m := range managedClusters.Items {
			if slices.ContainsFunc(m.Spec.Services, func(s v1alpha1.ManagedClusterServiceSpec) bool {
				return s.Template == tmpl.Name
			}) {
				return admission.Warnings{"The ServiceTemplate object can't be removed if ManagedCluster objects referencing it still exist"}, errTemplateDeletionForbidden
			}
		}

		if token = managedClusters.Continue; token == "" {
			break
		}
	}

	return nil, nil
}

// Default implements webhook.Defaulter so a webhook will be registered for the type.
func (*ServiceTemplateValidator) Default(_ context.Context, _ runtime.Object) error {
	return nil
}

type ProviderTemplateValidator struct {
	client.Client
}

func (in *ProviderTemplateValidator) SetupWebhookWithManager(mgr ctrl.Manager) error {
	in.Client = mgr.GetClient()
	return ctrl.NewWebhookManagedBy(mgr).
		For(&v1alpha1.ProviderTemplate{}).
		WithValidator(in).
		WithDefaulter(in).
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
func (*ProviderTemplateValidator) ValidateDelete(_ context.Context, _ runtime.Object) (admission.Warnings, error) {
	return nil, nil
}

// Default implements webhook.Defaulter so a webhook will be registered for the type.
func (*ProviderTemplateValidator) Default(_ context.Context, _ runtime.Object) error {
	return nil
}
