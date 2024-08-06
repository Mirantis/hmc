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

package webhook // nolint:dupl

import (
	"context"
	"fmt"

	"github.com/Mirantis/hmc/api/v1alpha1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

type DeploymentValidator struct {
	client.Client
}

func (in *DeploymentValidator) SetupWebhookWithManager(mgr ctrl.Manager) error {
	in.Client = mgr.GetClient()
	return ctrl.NewWebhookManagedBy(mgr).
		For(&v1alpha1.Deployment{}).
		WithValidator(in).
		WithDefaulter(in).
		Complete()
}

var (
	_ webhook.CustomValidator = &DeploymentValidator{}
	_ webhook.CustomDefaulter = &DeploymentValidator{}
)

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type.
func (*DeploymentValidator) ValidateCreate(_ context.Context, _ runtime.Object) (admission.Warnings, error) {
	return nil, nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type.
func (*DeploymentValidator) ValidateUpdate(_ context.Context, _ runtime.Object, _ runtime.Object) (admission.Warnings, error) {
	return nil, nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type.
func (*DeploymentValidator) ValidateDelete(_ context.Context, _ runtime.Object) (admission.Warnings, error) {
	return nil, nil
}

// Default implements webhook.Defaulter so a webhook will be registered for the type.
func (in *DeploymentValidator) Default(ctx context.Context, obj runtime.Object) error {
	deployment, ok := obj.(*v1alpha1.Deployment)
	if !ok {
		return apierrors.NewBadRequest(fmt.Sprintf("expected Deployment but got a %T", obj))
	}
	template := &v1alpha1.Template{}
	templateRef := types.NamespacedName{Name: deployment.Spec.Template, Namespace: v1alpha1.TemplatesNamespace}
	if err := in.Get(ctx, templateRef, template); err != nil {
		return err
	}
	applyDefaultDeploymentConfiguration(deployment, template)
	return nil
}

func applyDefaultDeploymentConfiguration(deployment *v1alpha1.Deployment, template *v1alpha1.Template) {
	if deployment.Spec.Config != nil || template.Status.Config == nil {
		// Only apply defaults when there's no configuration provided
		return
	}
	deployment.Spec.DryRun = true
	deployment.Spec.Config = &apiextensionsv1.JSON{Raw: template.Status.Config.Raw}
}
