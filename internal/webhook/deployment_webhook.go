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
	"errors"
	"fmt"
	"sort"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/Mirantis/hmc/api/v1alpha1"
	"github.com/Mirantis/hmc/internal/utils"
)

type DeploymentValidator struct {
	client.Client
}

var (
	InvalidDeploymentErr = errors.New("the deployment is invalid")
)

func (v *DeploymentValidator) SetupWebhookWithManager(mgr ctrl.Manager) error {
	v.Client = mgr.GetClient()
	return ctrl.NewWebhookManagedBy(mgr).
		For(&v1alpha1.Deployment{}).
		WithValidator(v).
		WithDefaulter(v).
		Complete()
}

var (
	_ webhook.CustomValidator = &DeploymentValidator{}
	_ webhook.CustomDefaulter = &DeploymentValidator{}
)

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type.
func (v *DeploymentValidator) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	deployment, ok := obj.(*v1alpha1.Deployment)
	if !ok {
		return nil, apierrors.NewBadRequest(fmt.Sprintf("expected Deployment but got a %T", obj))
	}
	template, err := v.getDeploymentTemplate(ctx, deployment.Spec.Template)
	if err != nil {
		return nil, fmt.Errorf("%s: %v", InvalidDeploymentErr, err)
	}
	err = v.isTemplateValid(ctx, template)
	if err != nil {
		return nil, fmt.Errorf("%s: %v", InvalidDeploymentErr, err)
	}
	return nil, nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type.
func (v *DeploymentValidator) ValidateUpdate(ctx context.Context, _ runtime.Object, newObj runtime.Object) (admission.Warnings, error) {
	newDeployment, ok := newObj.(*v1alpha1.Deployment)
	if !ok {
		return nil, apierrors.NewBadRequest(fmt.Sprintf("expected Deployment but got a %T", newObj))
	}
	template, err := v.getDeploymentTemplate(ctx, newDeployment.Spec.Template)
	if err != nil {
		return nil, fmt.Errorf("%s: %v", InvalidDeploymentErr, err)
	}
	err = v.isTemplateValid(ctx, template)
	if err != nil {
		return nil, fmt.Errorf("%s: %v", InvalidDeploymentErr, err)
	}
	return nil, nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type.
func (*DeploymentValidator) ValidateDelete(_ context.Context, _ runtime.Object) (admission.Warnings, error) {
	return nil, nil
}

// Default implements webhook.Defaulter so a webhook will be registered for the type.
func (v *DeploymentValidator) Default(ctx context.Context, obj runtime.Object) error {
	deployment, ok := obj.(*v1alpha1.Deployment)
	if !ok {
		return apierrors.NewBadRequest(fmt.Sprintf("expected Deployment but got a %T", obj))
	}

	// Only apply defaults when there's no configuration provided
	if deployment.Spec.Config != nil {
		return nil
	}
	template, err := v.getDeploymentTemplate(ctx, deployment.Spec.Template)
	if err != nil {
		return fmt.Errorf("could not get template for the deployment: %s", err)
	}
	err = v.isTemplateValid(ctx, template)
	if err != nil {
		return fmt.Errorf("template is invalid: %s", err)
	}
	if template.Status.Config == nil {
		return nil
	}
	deployment.Spec.DryRun = true
	deployment.Spec.Config = &apiextensionsv1.JSON{Raw: template.Status.Config.Raw}
	return nil
}

func (v *DeploymentValidator) getDeploymentTemplate(ctx context.Context, templateName string) (*v1alpha1.Template, error) {
	template := &v1alpha1.Template{}
	templateRef := types.NamespacedName{Name: templateName, Namespace: v1alpha1.TemplatesNamespace}
	if err := v.Get(ctx, templateRef, template); err != nil {
		return nil, err
	}
	return template, nil
}

func (v *DeploymentValidator) isTemplateValid(ctx context.Context, template *v1alpha1.Template) error {
	if template.Status.Type != v1alpha1.TemplateTypeDeployment {
		return fmt.Errorf("the template should be of the deployment type. Current: %s", template.Status.Type)
	}
	if !template.Status.Valid {
		return fmt.Errorf("the template is not valid: %s", template.Status.ValidationError)
	}
	err := v.verifyProviders(ctx, template)
	if err != nil {
		return fmt.Errorf("providers verification failed: %v", err)
	}
	return nil
}

func (v *DeploymentValidator) verifyProviders(ctx context.Context, template *v1alpha1.Template) error {
	requiredProviders := template.Status.Providers
	management := &v1alpha1.Management{}
	managementRef := types.NamespacedName{Name: v1alpha1.ManagementName, Namespace: v1alpha1.ManagementNamespace}
	if err := v.Get(ctx, managementRef, management); err != nil {
		return err
	}

	exposedProviders := management.Status.AvailableProviders
	missingProviders := make(map[string][]string)
	missingProviders["bootstrap"] = getMissingProviders(exposedProviders.BootstrapProviders, requiredProviders.BootstrapProviders)
	missingProviders["control plane"] = getMissingProviders(exposedProviders.ControlPlaneProviders, requiredProviders.ControlPlaneProviders)
	missingProviders["infrastructure"] = getMissingProviders(exposedProviders.InfrastructureProviders, requiredProviders.InfrastructureProviders)

	var errs []error
	for providerType, missing := range missingProviders {
		if len(missing) > 0 {
			sort.Slice(missing, func(i, j int) bool {
				return missing[i] < missing[j]
			})
			errs = append(errs, fmt.Errorf("one or more required %s providers are not deployed yet: %v", providerType, missing))
		}
	}
	if len(errs) > 0 {
		sort.Slice(errs, func(i, j int) bool {
			return errs[i].Error() < errs[j].Error()
		})
		return errors.Join(errs...)
	}
	return nil
}

func getMissingProviders(exposedProviders []string, requiredProviders []string) []string {
	exposedBootstrapProviders := utils.SliceToMapKeys[[]string, map[string]struct{}](exposedProviders)
	diff, isSubset := utils.DiffSliceSubset[[]string, map[string]struct{}](requiredProviders, exposedBootstrapProviders)
	if !isSubset {
		return diff
	}
	return []string{}
}
