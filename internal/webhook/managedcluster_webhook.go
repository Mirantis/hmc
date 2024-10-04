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
	"sort"

	"github.com/Masterminds/semver/v3"
	admissionv1 "k8s.io/api/admission/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	hmcv1alpha1 "github.com/Mirantis/hmc/api/v1alpha1"
)

type ManagedClusterValidator struct {
	client.Client
}

const invalidManagedClusterMsg = "the ManagedCluster is invalid"

func (v *ManagedClusterValidator) SetupWebhookWithManager(mgr ctrl.Manager) error {
	v.Client = mgr.GetClient()
	return ctrl.NewWebhookManagedBy(mgr).
		For(&hmcv1alpha1.ManagedCluster{}).
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
	managedCluster, ok := obj.(*hmcv1alpha1.ManagedCluster)
	if !ok {
		return nil, apierrors.NewBadRequest(fmt.Sprintf("expected ManagedCluster but got a %T", obj))
	}

	template, err := v.getManagedClusterTemplate(ctx, managedCluster.Namespace, managedCluster.Spec.Template)
	if err != nil {
		return nil, fmt.Errorf("%s: %v", invalidManagedClusterMsg, err)
	}

	if err := v.isTemplateValid(ctx, template); err != nil {
		return nil, fmt.Errorf("%s: %v", invalidManagedClusterMsg, err)
	}

	return nil, nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type.
func (v *ManagedClusterValidator) ValidateUpdate(ctx context.Context, _ runtime.Object, newObj runtime.Object) (admission.Warnings, error) {
	newManagedCluster, ok := newObj.(*hmcv1alpha1.ManagedCluster)
	if !ok {
		return nil, apierrors.NewBadRequest(fmt.Sprintf("expected ManagedCluster but got a %T", newObj))
	}

	template, err := v.getManagedClusterTemplate(ctx, newManagedCluster.Namespace, newManagedCluster.Spec.Template)
	if err != nil {
		return nil, fmt.Errorf("%s: %v", invalidManagedClusterMsg, err)
	}

	if err := v.isTemplateValid(ctx, template); err != nil {
		return nil, fmt.Errorf("%s: %v", invalidManagedClusterMsg, err)
	}

	if err := validateK8sCompatibility(ctx, v.Client, newManagedCluster); err != nil {
		return admission.Warnings{"Failed to validate k8s version compatibility with ServiceTemplates"}, fmt.Errorf("failed to validate k8s compatibility: %v", err)
	}

	return nil, nil
}

func validateK8sCompatibility(ctx context.Context, cl client.Client, mc *hmcv1alpha1.ManagedCluster) error {
	if len(mc.Spec.Services) == 0 || mc.Status.KubertenesVersion == "" {
		return nil
	}

	svcTpls := new(hmcv1alpha1.ServiceTemplateList)
	if err := cl.List(ctx, svcTpls, client.InNamespace(mc.Namespace)); err != nil {
		return fmt.Errorf("failed to list ServiceTemplates in %s namespace: %w", mc.Namespace, err)
	}

	svcTplName2KConstraint := make(map[string]string, len(svcTpls.Items))
	for _, v := range svcTpls.Items {
		svcTplName2KConstraint[v.Name] = v.Status.KubertenesConstraint
	}

	mcVersion, err := semver.NewVersion(mc.Status.KubertenesVersion)
	if err != nil { // should never happen
		return fmt.Errorf("failed to parse k8s version %s of the ManagedCluster %s/%s: %w", mc.Status.KubertenesVersion, mc.Namespace, mc.Name, err)
	}

	for _, v := range mc.Spec.Services {
		if v.Disable {
			continue
		}

		kc, ok := svcTplName2KConstraint[v.Template]
		if !ok {
			return fmt.Errorf("specified ServiceTemplate %s/%s is missing in the cluster", mc.Namespace, v.Template)
		}

		if kc == "" {
			continue
		}

		tplConstraint, err := semver.NewConstraint(kc)
		if err != nil { // should never happen
			return fmt.Errorf("failed to parse k8s constrainted version %s of the ServiceTemplate %s/%s: %w", kc, mc.Namespace, v.Template, err)
		}

		if !tplConstraint.Check(mcVersion) {
			return fmt.Errorf("k8s version %s of the ManagedCluster %s/%s does not satisfy constrainted version %s from the ServiceTemplate %s/%s",
				mc.Status.KubertenesVersion, mc.Namespace, mc.Name,
				kc, mc.Namespace, v.Template)
		}
	}

	return nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type.
func (*ManagedClusterValidator) ValidateDelete(_ context.Context, _ runtime.Object) (admission.Warnings, error) {
	return nil, nil
}

// Default implements webhook.Defaulter so a webhook will be registered for the type.
func (v *ManagedClusterValidator) Default(ctx context.Context, obj runtime.Object) error {
	managedCluster, ok := obj.(*hmcv1alpha1.ManagedCluster)
	if !ok {
		return apierrors.NewBadRequest(fmt.Sprintf("expected ManagedCluster but got a %T", obj))
	}

	// Only apply defaults when there's no configuration provided;
	// if template ref is empty, then nothing to default
	if managedCluster.Spec.Config != nil || managedCluster.Spec.Template == "" {
		return nil
	}

	template, err := v.getManagedClusterTemplate(ctx, managedCluster.Namespace, managedCluster.Spec.Template)
	if err != nil {
		return fmt.Errorf("could not get template for the managedcluster: %v", err)
	}

	if err := v.isTemplateValid(ctx, template); err != nil {
		return fmt.Errorf("template is invalid: %v", err)
	}

	if template.Status.Config == nil {
		return nil
	}

	managedCluster.Spec.DryRun = true
	managedCluster.Spec.Config = &apiextensionsv1.JSON{Raw: template.Status.Config.Raw}

	return nil
}

func (v *ManagedClusterValidator) getManagedClusterTemplate(ctx context.Context, templateNamespace, templateName string) (tpl *hmcv1alpha1.ClusterTemplate, err error) {
	tpl = new(hmcv1alpha1.ClusterTemplate)
	return tpl, v.Get(ctx, client.ObjectKey{Namespace: templateNamespace, Name: templateName}, tpl)
}

func (v *ManagedClusterValidator) isTemplateValid(ctx context.Context, template *hmcv1alpha1.ClusterTemplate) error {
	if !template.Status.Valid {
		return fmt.Errorf("the template is not valid: %s", template.Status.ValidationError)
	}

	if err := v.verifyProviders(ctx, template); err != nil {
		return fmt.Errorf("failed to verify providers: %v", err)
	}

	return nil
}

func (v *ManagedClusterValidator) verifyProviders(ctx context.Context, template *hmcv1alpha1.ClusterTemplate) error {
	management := new(hmcv1alpha1.Management)
	if err := v.Get(ctx, client.ObjectKey{Name: hmcv1alpha1.ManagementName}, management); err != nil {
		return err
	}

	const (
		bootstrapProviderType    = "bootstrap"
		controlPlateProviderType = "control plane"
		infraProviderType        = "infrastructure"
	)

	var (
		exposedProviders  = management.Status.AvailableProviders
		requiredProviders = template.Status.Providers

		missingBootstrap, missingCP, missingInfra []string
		wrongVersionProviders                     map[string][]string
	)

	// on update we have to validate versions between exact the provider tpl and constraints from the cluster tpl
	if req, _ := admission.RequestFromContext(ctx); req.Operation == admissionv1.Update {
		wrongVersionProviders = make(map[string][]string, 3)
		missing, wrongVers, err := getMissingProvidersWithWrongVersions(exposedProviders.BootstrapProviders, requiredProviders.BootstrapProviders)
		if err != nil {
			return err
		}
		wrongVersionProviders[bootstrapProviderType], missingBootstrap = wrongVers, missing

		missing, wrongVers, err = getMissingProvidersWithWrongVersions(exposedProviders.ControlPlaneProviders, requiredProviders.ControlPlaneProviders)
		if err != nil {
			return err
		}
		wrongVersionProviders[controlPlateProviderType], missingCP = wrongVers, missing

		missing, wrongVers, err = getMissingProvidersWithWrongVersions(exposedProviders.InfrastructureProviders, requiredProviders.InfrastructureProviders)
		if err != nil {
			return err
		}
		wrongVersionProviders[infraProviderType], missingInfra = wrongVers, missing
	} else {
		missingBootstrap = getMissingProviders(exposedProviders.BootstrapProviders, requiredProviders.BootstrapProviders)
		missingCP = getMissingProviders(exposedProviders.ControlPlaneProviders, requiredProviders.ControlPlaneProviders)
		missingInfra = getMissingProviders(exposedProviders.InfrastructureProviders, requiredProviders.InfrastructureProviders)
	}

	missingProviders := map[string][]string{
		bootstrapProviderType:    missingBootstrap,
		controlPlateProviderType: missingCP,
		infraProviderType:        missingInfra,
	}

	errs := collectErrors(missingProviders, "one or more required %s providers are not deployed yet: %v")
	errs = append(errs, collectErrors(wrongVersionProviders, "one or more required %s providers does not satisfy constraints: %v")...)
	if len(errs) > 0 {
		sort.Slice(errs, func(i, j int) bool {
			return errs[i].Error() < errs[j].Error()
		})

		return errors.Join(errs...)
	}

	return nil
}

func collectErrors(m map[string][]string, msgFormat string) (errs []error) {
	for providerType, missing := range m {
		if len(missing) > 0 {
			slices.Sort(missing)
			errs = append(errs, fmt.Errorf(msgFormat, providerType, missing))
		}
	}

	return errs
}

func getMissingProviders(exposed, required []hmcv1alpha1.ProviderTuple) (missing []string) {
	missing, _, _ = getMissingProvidersWithWrongVersions(exposed, required)
	return missing
}

func getMissingProvidersWithWrongVersions(exposed, required []hmcv1alpha1.ProviderTuple) (missing, nonSatisfying []string, _ error) {
	exposedSet := make(map[string]hmcv1alpha1.ProviderTuple, len(exposed))
	for _, v := range exposed {
		exposedSet[v.Name] = v
	}

	var merr error
	for _, reqWithConstraint := range required {
		exposedWithExactVer, ok := exposedSet[reqWithConstraint.Name]
		if !ok {
			missing = append(missing, reqWithConstraint.Name)
			continue
		}

		if exposedWithExactVer.VersionOrConstraint == "" || reqWithConstraint.VersionOrConstraint == "" {
			continue
		}

		exactVer, err := semver.NewVersion(exposedWithExactVer.VersionOrConstraint)
		if err != nil {
			merr = errors.Join(merr, fmt.Errorf("failed to parse version %s of the provider %s: %w", exposedWithExactVer.VersionOrConstraint, exposedWithExactVer.Name, err))
			continue
		}

		requiredC, err := semver.NewConstraint(reqWithConstraint.VersionOrConstraint)
		if err != nil {
			merr = errors.Join(merr, fmt.Errorf("failed to parse constraint %s of the provider %s: %w", exposedWithExactVer.VersionOrConstraint, exposedWithExactVer.Name, err))
			continue
		}

		if !requiredC.Check(exactVer) {
			nonSatisfying = append(nonSatisfying, fmt.Sprintf("%s %s !~ %s", reqWithConstraint.Name, exposedWithExactVer.VersionOrConstraint, reqWithConstraint.VersionOrConstraint))
		}
	}

	return missing, nonSatisfying, merr
}
