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
	"fmt"
	"testing"

	. "github.com/onsi/gomega"
	admissionv1 "k8s.io/api/admission/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/Mirantis/hmc/api/v1alpha1"
	"github.com/Mirantis/hmc/test/objects/managedcluster"
	"github.com/Mirantis/hmc/test/objects/management"
	"github.com/Mirantis/hmc/test/objects/release"
	"github.com/Mirantis/hmc/test/objects/template"
	"github.com/Mirantis/hmc/test/scheme"
)

func TestManagementValidateUpdate(t *testing.T) {
	g := NewWithT(t)

	ctx := admission.NewContextWithRequest(context.Background(), admission.Request{AdmissionRequest: admissionv1.AdmissionRequest{Operation: admissionv1.Update}})

	const (
		someContractVersion = "v1alpha4_v1beta1"
		capiVersion         = "v1beta1"
		capiVersionOther    = "v1alpha3"

		infraAWSProvider   = "infrastructure-aws"
		infraOtherProvider = "infrastructure-other-provider"
	)

	validStatus := v1alpha1.TemplateValidationStatus{Valid: true}

	componentAwsDefaultTpl := v1alpha1.Provider{
		Name: v1alpha1.ProviderCAPAName,
		Component: v1alpha1.Component{
			Template: template.DefaultName,
		},
	}

	tests := []struct {
		name            string
		oldMgmt         *v1alpha1.Management
		management      *v1alpha1.Management
		existingObjects []runtime.Object
		err             string
		warnings        admission.Warnings
	}{
		{
			name:       "no providers removed, should succeed",
			oldMgmt:    management.NewManagement(management.WithProviders(componentAwsDefaultTpl)),
			management: management.NewManagement(management.WithProviders(componentAwsDefaultTpl)),
		},
		{
			name: "release does not exist, should fail",
			oldMgmt: management.NewManagement(
				management.WithProviders(componentAwsDefaultTpl),
			),
			management: management.NewManagement(
				management.WithProviders(),
				management.WithRelease(release.DefaultName),
			),
			warnings: admission.Warnings{"Some of the providers cannot be removed"},
			err:      fmt.Sprintf(`Management "%s" is invalid: spec.providers: Forbidden: failed to get Release %s: releases.hmc.mirantis.com "%s" not found`, management.DefaultName, release.DefaultName, release.DefaultName),
		},
		{
			name: "removed provider does not have related providertemplate, should fail",
			oldMgmt: management.NewManagement(
				management.WithProviders(componentAwsDefaultTpl),
			),
			management: management.NewManagement(
				management.WithProviders(),
				management.WithRelease(release.DefaultName),
			),
			existingObjects: []runtime.Object{
				release.New(),
			},
			warnings: admission.Warnings{"Some of the providers cannot be removed"},
			err:      fmt.Sprintf(`Management "%s" is invalid: spec.providers: Forbidden: failed to get ProviderTemplate %s: providertemplates.hmc.mirantis.com "%s" not found`, management.DefaultName, template.DefaultName, template.DefaultName),
		},
		{
			name: "no cluster templates, should succeed",
			oldMgmt: management.NewManagement(
				management.WithProviders(componentAwsDefaultTpl),
			),
			management: management.NewManagement(
				management.WithProviders(),
				management.WithRelease(release.DefaultName),
			),
			existingObjects: []runtime.Object{
				release.New(),
				template.NewProviderTemplate(template.WithName(release.DefaultCAPITemplateName)),
				template.NewProviderTemplate(template.WithProvidersStatus(infraAWSProvider)),
			},
		},
		{
			name: "cluster template from removed provider exists but no managed clusters, should succeed",
			oldMgmt: management.NewManagement(
				management.WithProviders(componentAwsDefaultTpl),
			),
			management: management.NewManagement(
				management.WithProviders(),
				management.WithRelease(release.DefaultName),
			),
			existingObjects: []runtime.Object{
				release.New(),
				template.NewProviderTemplate(template.WithName(release.DefaultCAPITemplateName)),
				template.NewProviderTemplate(template.WithProvidersStatus(infraAWSProvider)),
				template.NewClusterTemplate(template.WithProvidersStatus(infraAWSProvider)),
			},
		},
		{
			name: "managed cluster uses the removed provider, should fail",
			oldMgmt: management.NewManagement(
				management.WithProviders(componentAwsDefaultTpl),
			),
			management: management.NewManagement(
				management.WithProviders(),
				management.WithRelease(release.DefaultName),
			),
			existingObjects: []runtime.Object{
				release.New(),
				template.NewProviderTemplate(template.WithProvidersStatus(infraAWSProvider)),
				template.NewProviderTemplate(template.WithName(release.DefaultCAPITemplateName)),
				template.NewClusterTemplate(template.WithProvidersStatus(infraAWSProvider)),
				managedcluster.NewManagedCluster(managedcluster.WithClusterTemplate(template.DefaultName)),
			},
			warnings: admission.Warnings{"Some of the providers cannot be removed"},
			err:      fmt.Sprintf(`Management "%s" is invalid: spec.providers: Forbidden: provider %s is required by at least one ManagedCluster (%s/%s) and cannot be removed from the Management %s`, management.DefaultName, infraAWSProvider, managedcluster.DefaultNamespace, managedcluster.DefaultName, management.DefaultName),
		},
		{
			name: "managed cluster does not use the removed provider, should succeed",
			oldMgmt: management.NewManagement(
				management.WithProviders(componentAwsDefaultTpl),
			),
			management: management.NewManagement(
				management.WithProviders(),
				management.WithRelease(release.DefaultName),
			),
			existingObjects: []runtime.Object{
				release.New(),
				template.NewProviderTemplate(template.WithProvidersStatus(infraAWSProvider)),
				template.NewProviderTemplate(template.WithName(release.DefaultCAPITemplateName)),
				template.NewClusterTemplate(template.WithProvidersStatus(infraOtherProvider)),
				managedcluster.NewManagedCluster(managedcluster.WithClusterTemplate(template.DefaultName)),
			},
		},
		{
			name:       "no release and no core capi tpl set, should succeed",
			oldMgmt:    management.NewManagement(),
			management: management.NewManagement(),
		},
		{
			name:            "no capi providertemplate, should fail",
			oldMgmt:         management.NewManagement(),
			management:      management.NewManagement(management.WithRelease(release.DefaultName)),
			existingObjects: []runtime.Object{release.New()},
			err:             fmt.Sprintf(`the Management is invalid: failed to get ProviderTemplate %s: providertemplates.hmc.mirantis.com "%s" not found`, release.DefaultCAPITemplateName, release.DefaultCAPITemplateName),
		},
		{
			name:       "capi providertemplate without capi version set, should succeed",
			oldMgmt:    management.NewManagement(),
			management: management.NewManagement(management.WithRelease(release.DefaultName)),
			existingObjects: []runtime.Object{
				release.New(),
				template.NewProviderTemplate(template.WithName(release.DefaultCAPITemplateName)),
			},
		},
		{
			name:       "capi providertemplate is not valid, should fail",
			oldMgmt:    management.NewManagement(),
			management: management.NewManagement(management.WithRelease(release.DefaultName)),
			existingObjects: []runtime.Object{
				release.New(),
				template.NewProviderTemplate(
					template.WithName(release.DefaultCAPITemplateName),
					template.WithProviderStatusCAPIContracts(capiVersion, ""),
				),
			},
			err: "the Management is invalid: not valid ProviderTemplate " + release.DefaultCAPITemplateName,
		},
		{
			name:    "no providertemplates that declared in mgmt spec.providers, should fail",
			oldMgmt: management.NewManagement(),
			management: management.NewManagement(
				management.WithRelease(release.DefaultName),
				management.WithProviders(componentAwsDefaultTpl),
			),
			existingObjects: []runtime.Object{
				release.New(),
				template.NewProviderTemplate(
					template.WithName(release.DefaultCAPITemplateName),
					template.WithProviderStatusCAPIContracts(capiVersion, ""),
					template.WithValidationStatus(validStatus),
				),
			},
			err: fmt.Sprintf(`the Management is invalid: failed to get ProviderTemplate %s: providertemplates.hmc.mirantis.com "%s" not found`, template.DefaultName, template.DefaultName),
		},
		{
			name:    "providertemplates without specified capi contracts, should succeed",
			oldMgmt: management.NewManagement(),
			management: management.NewManagement(
				management.WithRelease(release.DefaultName),
				management.WithProviders(componentAwsDefaultTpl),
			),
			existingObjects: []runtime.Object{
				release.New(),
				template.NewProviderTemplate(
					template.WithName(release.DefaultCAPITemplateName),
					template.WithProviderStatusCAPIContracts(capiVersion, ""),
					template.WithValidationStatus(validStatus),
				),
				template.NewProviderTemplate(),
			},
		},
		{
			name:    "providertemplates is not ready, should succeed",
			oldMgmt: management.NewManagement(),
			management: management.NewManagement(
				management.WithRelease(release.DefaultName),
				management.WithProviders(componentAwsDefaultTpl),
			),
			existingObjects: []runtime.Object{
				release.New(),
				template.NewProviderTemplate(
					template.WithName(release.DefaultCAPITemplateName),
					template.WithProviderStatusCAPIContracts(capiVersion, ""),
					template.WithValidationStatus(validStatus),
				),
				template.NewProviderTemplate(
					template.WithProviderStatusCAPIContracts(capiVersionOther, someContractVersion),
				),
			},
			err: "the Management is invalid: not valid ProviderTemplate " + template.DefaultName,
		},
		{
			name:    "providertemplates do not match capi contracts, should fail",
			oldMgmt: management.NewManagement(),
			management: management.NewManagement(
				management.WithRelease(release.DefaultName),
				management.WithProviders(componentAwsDefaultTpl),
			),
			existingObjects: []runtime.Object{
				release.New(),
				template.NewProviderTemplate(
					template.WithName(release.DefaultCAPITemplateName),
					template.WithProviderStatusCAPIContracts(capiVersion, ""),
					template.WithValidationStatus(validStatus),
				),
				template.NewProviderTemplate(
					template.WithProviderStatusCAPIContracts(capiVersionOther, someContractVersion),
					template.WithValidationStatus(validStatus),
				),
			},
			warnings: admission.Warnings{"The Management object has incompatible CAPI contract versions in ProviderTemplates"},
			err:      fmt.Sprintf("the Management is invalid: core CAPI contract versions does not support %s version in the ProviderTemplate %s", capiVersionOther, template.DefaultName),
		},
		{
			name:    "providertemplates match capi contracts, should succeed",
			oldMgmt: management.NewManagement(),
			management: management.NewManagement(
				management.WithRelease(release.DefaultName),
				management.WithProviders(componentAwsDefaultTpl),
			),
			existingObjects: []runtime.Object{
				release.New(),
				template.NewProviderTemplate(
					template.WithName(release.DefaultCAPITemplateName),
					template.WithProviderStatusCAPIContracts(capiVersion, ""),
					template.WithValidationStatus(validStatus),
				),
				template.NewProviderTemplate(
					template.WithProviderStatusCAPIContracts(capiVersion, someContractVersion),
					template.WithValidationStatus(validStatus),
				),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(_ *testing.T) {
			c := fake.NewClientBuilder().
				WithScheme(scheme.Scheme).
				WithRuntimeObjects(tt.existingObjects...).
				WithIndex(&v1alpha1.ClusterTemplate{}, v1alpha1.ClusterTemplateProvidersIndexKey, v1alpha1.ExtractProvidersFromClusterTemplate).
				WithIndex(&v1alpha1.ManagedCluster{}, v1alpha1.ManagedClusterTemplateIndexKey, v1alpha1.ExtractTemplateNameFromManagedCluster).
				Build()
			validator := &ManagementValidator{Client: c}

			warnings, err := validator.ValidateUpdate(ctx, tt.oldMgmt, tt.management)
			if tt.err != "" {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err).To(MatchError(tt.err))
			} else {
				g.Expect(err).To(Succeed())
			}

			g.Expect(warnings).To(Equal(tt.warnings))
		})
	}
}

func TestManagementValidateDelete(t *testing.T) {
	g := NewWithT(t)

	ctx := admission.NewContextWithRequest(context.Background(), admission.Request{AdmissionRequest: admissionv1.AdmissionRequest{Operation: admissionv1.Delete}})

	tests := []struct {
		name            string
		management      *v1alpha1.Management
		existingObjects []runtime.Object
		err             string
		warnings        admission.Warnings
	}{
		{
			name:            "should fail if ManagedCluster objects exist",
			management:      management.NewManagement(),
			existingObjects: []runtime.Object{managedcluster.NewManagedCluster()},
			warnings:        admission.Warnings{"The Management object can't be removed if ManagedCluster objects still exist"},
			err:             "management deletion is forbidden",
		},
		{
			name:       "should succeed",
			management: management.NewManagement(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(_ *testing.T) {
			c := fake.NewClientBuilder().WithScheme(scheme.Scheme).WithRuntimeObjects(tt.existingObjects...).Build()
			validator := &ManagementValidator{Client: c}

			warn, err := validator.ValidateDelete(ctx, tt.management)
			if tt.err != "" {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err).To(MatchError(tt.err))
			} else {
				g.Expect(err).To(Succeed())
			}

			g.Expect(warn).To(Equal(tt.warnings))
		})
	}
}
