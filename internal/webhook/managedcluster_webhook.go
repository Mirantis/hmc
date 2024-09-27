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

type ManagedClusterValidator struct {
	client.Client
}

var (
	errInvalidManagedCluster = errors.New("the ManagedCluster is invalid")
	errNotReadyComponents    = errors.New("one or more required components are not ready")
)

func (v *ManagedClusterValidator) SetupWebhookWithManager(mgr ctrl.Manager) error {
	v.Client = mgr.GetClient()
	return ctrl.NewWebhookManagedBy(mgr).
		For(&v1alpha1.ManagedCluster{}).
		WithValidator(v).
		WithDefaulter(v).
		Complete()
}

var (
	_ webhook.CustomValidator = &ManagedClusterValidator{}
	_ webhook.CustomDefaulter = &ManagedClusterValidator{}
)

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type.
func (v *ManagedClusterValidator) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	managedCluster, ok := obj.(*v1alpha1.ManagedCluster)
	if !ok {
		return nil, apierrors.NewBadRequest(fmt.Sprintf("expected ManagedCluster but got a %T", obj))
	}
	template, err := v.getManagedClusterTemplate(ctx, managedCluster.Namespace, managedCluster.Spec.Template)
	if err != nil {
		return nil, fmt.Errorf("%s: %v", errInvalidManagedCluster, err)
	}
	err = v.isTemplateValid(template)
	if err != nil {
		return nil, fmt.Errorf("%s: %v", errInvalidManagedCluster, err)
	}
	warnings, err := v.checkComponentsHealth(ctx, template)
	if err != nil {
		return nil, fmt.Errorf("failed to verify components health: %v", err)
	}
	if len(warnings) > 0 {
		return warnings, errNotReadyComponents
	}
	return nil, nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type.
func (v *ManagedClusterValidator) ValidateUpdate(ctx context.Context, _ runtime.Object, newObj runtime.Object) (admission.Warnings, error) {
	newManagedCluster, ok := newObj.(*v1alpha1.ManagedCluster)
	if !ok {
		return nil, apierrors.NewBadRequest(fmt.Sprintf("expected ManagedCluster but got a %T", newObj))
	}
	template, err := v.getManagedClusterTemplate(ctx, newManagedCluster.Namespace, newManagedCluster.Spec.Template)
	if err != nil {
		return nil, fmt.Errorf("%s: %v", errInvalidManagedCluster, err)
	}
	err = v.isTemplateValid(template)
	if err != nil {
		return nil, fmt.Errorf("%s: %v", errInvalidManagedCluster, err)
	}
	return nil, nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type.
func (*ManagedClusterValidator) ValidateDelete(_ context.Context, _ runtime.Object) (admission.Warnings, error) {
	return nil, nil
}

// Default implements webhook.Defaulter so a webhook will be registered for the type.
func (v *ManagedClusterValidator) Default(ctx context.Context, obj runtime.Object) error {
	managedCluster, ok := obj.(*v1alpha1.ManagedCluster)
	if !ok {
		return apierrors.NewBadRequest(fmt.Sprintf("expected ManagedCluster but got a %T", obj))
	}

	// Only apply defaults when there's no configuration provided
	if managedCluster.Spec.Config != nil {
		return nil
	}
	template, err := v.getManagedClusterTemplate(ctx, managedCluster.Namespace, managedCluster.Spec.Template)
	if err != nil {
		return fmt.Errorf("could not get template for the managedcluster: %s", err)
	}
	err = v.isTemplateValid(template)
	if err != nil {
		return fmt.Errorf("template is invalid: %s", err)
	}
	if template.Status.Config == nil {
		return nil
	}
	managedCluster.Spec.DryRun = true
	managedCluster.Spec.Config = &apiextensionsv1.JSON{Raw: template.Status.Config.Raw}
	return nil
}

func (v *ManagedClusterValidator) getManagedClusterTemplate(ctx context.Context, templateNamespace, templateName string) (*v1alpha1.ClusterTemplate, error) {
	template := &v1alpha1.ClusterTemplate{}
	templateRef := types.NamespacedName{Name: templateName, Namespace: templateNamespace}
	if err := v.Get(ctx, templateRef, template); err != nil {
		return nil, err
	}
	return template, nil
}

func (v *ManagedClusterValidator) getProviderTemplate(ctx context.Context, templateName string) (*v1alpha1.ProviderTemplate, error) {
	template := &v1alpha1.ProviderTemplate{}
	templateRef := types.NamespacedName{Name: templateName}
	if err := v.Get(ctx, templateRef, template); err != nil {
		return nil, err
	}
	return template, nil
}

func (*ManagedClusterValidator) isTemplateValid(template *v1alpha1.ClusterTemplate) error {
	if !template.Status.Valid {
		return fmt.Errorf("the template is not valid: %s", template.Status.ValidationError)
	}
	return nil
}

func (v *ManagedClusterValidator) checkComponentsHealth(ctx context.Context, clusterTemplate *v1alpha1.ClusterTemplate) (admission.Warnings, error) {
	requiredProviders := clusterTemplate.Status.Providers
	management := &v1alpha1.Management{}
	managementRef := types.NamespacedName{Name: v1alpha1.ManagementName}
	if err := v.Get(ctx, managementRef, management); err != nil {
		return nil, err
	}

	exposedProviders := management.Status.AvailableProviders
	missingComponents := make(map[string][]string)

	var failedComponents []string
	componentsErrors := make(map[string]string)
	for component, status := range management.Status.Components {
		if !status.Success {
			template, err := v.getProviderTemplate(ctx, component)
			if err != nil {
				return nil, err
			}
			if management.Spec.GetCoreTemplates()[component] {
				missingComponents["core components"] = append(missingComponents["core components"], component)
				failedComponents = append(failedComponents, component)
				componentsErrors[component] = status.Error
			}
			if oneOrMoreProviderFailed(template.Status.Providers.BootstrapProviders, requiredProviders.BootstrapProviders) ||
				oneOrMoreProviderFailed(template.Status.Providers.ControlPlaneProviders, requiredProviders.ControlPlaneProviders) ||
				oneOrMoreProviderFailed(template.Status.Providers.InfrastructureProviders, requiredProviders.InfrastructureProviders) {
				failedComponents = append(failedComponents, component)
				componentsErrors[component] = status.Error
			}
		}
	}

	missingComponents["bootstrap providers"] = getMissingProviders(exposedProviders.BootstrapProviders, requiredProviders.BootstrapProviders)
	missingComponents["control plane providers"] = getMissingProviders(exposedProviders.ControlPlaneProviders, requiredProviders.ControlPlaneProviders)
	missingComponents["infrastructure providers"] = getMissingProviders(exposedProviders.InfrastructureProviders, requiredProviders.InfrastructureProviders)

	warnings := make([]string, 0, len(missingComponents)+len(failedComponents))
	for componentType, missing := range missingComponents {
		if len(missing) > 0 {
			sort.Strings(missing)
			warnings = append(warnings, fmt.Sprintf("not ready %s: %v", componentType, missing))
		}
	}
	sort.Strings(warnings)

	sort.Strings(failedComponents)
	for _, failedComponent := range failedComponents {
		warnings = append(warnings, fmt.Sprintf("%s installation failed: %s", failedComponent, componentsErrors[failedComponent]))
	}
	return warnings, nil
}

func getMissingProviders(exposedProviders []string, requiredProviders []string) []string {
	exposedBootstrapProviders := utils.SliceToMapKeys[[]string, map[string]struct{}](exposedProviders)
	diff, isSubset := utils.DiffSliceSubset(requiredProviders, exposedBootstrapProviders)
	if !isSubset {
		return diff
	}
	return []string{}
}

func oneOrMoreProviderFailed(failedProviders []string, requiredProviders []string) bool {
	failedProvidersMap := utils.SliceToMapKeys[[]string, map[string]struct{}](failedProviders)
	_, isSubset := utils.DiffSliceSubset[[]string, map[string]struct{}](requiredProviders, failedProvidersMap)
	return isSubset
}
