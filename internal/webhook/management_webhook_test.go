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
		contractVersionValid   = "v1alpha4_v1beta1"
		contractVersionInvalid = "v1alpha1_v1alpha4_v1beta1"
	)

	providerAwsDefaultTpl := v1alpha1.Provider{
		Name: "infrastructure-aws",
		Component: v1alpha1.Component{
			Template: template.DefaultName,
		},
	}

	supportedCAPIContractVersions := []string{"v1alpha3", "v1alpha4", "v1beta1"}

	tests := []struct {
		name            string
		management      *v1alpha1.Management
		existingObjects []runtime.Object
		err             string
		warnings        admission.Warnings
	}{
		{
			name:       "no release and no core capi tpl set, should succeed",
			management: management.NewManagement(),
		},
		{
			name: "no providertemplates having providers in mgmt spec, should fail",
			management: management.NewManagement(
				management.WithRelease(release.DefaultName),
				management.WithProviders([]v1alpha1.Provider{providerAwsDefaultTpl}),
			),
			existingObjects: []runtime.Object{release.New()},
			err:             fmt.Sprintf(`failed to get ProviderTemplate %s: providertemplates.hmc.mirantis.com "%s" not found`, template.DefaultName, template.DefaultName),
		},
		{
			name: "providertemplates without specified capi contracts, should succeed",
			management: management.NewManagement(
				management.WithRelease(release.DefaultName),
				management.WithProviders([]v1alpha1.Provider{providerAwsDefaultTpl}),
			),
			existingObjects: []runtime.Object{
				release.New(),
				template.NewProviderTemplate(
					template.WithName(release.DefaultCAPITemplateName),
				),
				template.NewProviderTemplate(),
			},
		},
		{
			name: "providertemplates do not match capi contracts, should fail",
			management: management.NewManagement(
				management.WithRelease(release.DefaultName),
				management.WithProviders([]v1alpha1.Provider{providerAwsDefaultTpl}),
			),
			existingObjects: []runtime.Object{
				release.New(),
				template.NewProviderTemplate(
					template.WithName(release.DefaultCAPITemplateName),
				),
				template.NewProviderTemplate(
					template.WithProviderStatusCAPIContract(contractVersionInvalid),
				),
			},
			warnings: admission.Warnings{"The Management object has incompatible CAPI contract versions in ProviderTemplates"},
			err:      fmt.Sprintf("the Management is invalid: core CAPI contract versions %v does not support ProviderTemplate %s contract %s", supportedCAPIContractVersions, template.DefaultName, "v1alpha1"),
		},
		{
			name: "providertemplates match capi contracts, should succeed",
			management: management.NewManagement(
				management.WithRelease(release.DefaultName),
				management.WithProviders([]v1alpha1.Provider{providerAwsDefaultTpl}),
			),
			existingObjects: []runtime.Object{
				release.New(),
				template.NewProviderTemplate(
					template.WithName(release.DefaultCAPITemplateName),
				),
				template.NewProviderTemplate(
					template.WithProviderStatusCAPIContract(contractVersionValid),
				),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(_ *testing.T) {
			c := fake.NewClientBuilder().WithScheme(scheme.Scheme).WithRuntimeObjects(tt.existingObjects...).Build()
			validator := &ManagementValidator{Client: c}

			warnings, err := validator.ValidateUpdate(ctx, nil, tt.management)
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
