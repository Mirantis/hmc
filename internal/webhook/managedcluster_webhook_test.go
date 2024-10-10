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
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/Mirantis/hmc/api/v1alpha1"
	"github.com/Mirantis/hmc/test/objects/managedcluster"
	"github.com/Mirantis/hmc/test/objects/management"
	"github.com/Mirantis/hmc/test/objects/template"
	"github.com/Mirantis/hmc/test/scheme"
)

type testCase struct {
	name              string
	oldManagedCluster *v1alpha1.ManagedCluster
	newManagedCluster *v1alpha1.ManagedCluster
	existingObjects   []runtime.Object
	err               string
	warnings          admission.Warnings
}

const (
	testNamespace = "test"

	testTemplateName      = "template-test"
	templateUpgradeSource = "template-1-0-0"

	hmcTemplateName  = "hmc"
	capiTemplateName = "cluster-api"
	capzTemplateName = "cluster-api-provider-azure"
)

var (
	coreConfiguration = v1alpha1.Core{
		HMC:  v1alpha1.Component{Template: hmcTemplateName},
		CAPI: v1alpha1.Component{Template: capiTemplateName},
	}

	mgmt = management.NewManagement(
		management.WithCoreComponents(&coreConfiguration),
		management.WithAvailableProviders(v1alpha1.Providers{
			InfrastructureProviders: []string{"aws"},
			BootstrapProviders:      []string{"k0s"},
			ControlPlaneProviders:   []string{"k0s"},
		}),
		management.WithComponentsStatus(map[string]v1alpha1.ComponentStatus{
			hmcTemplateName:  {Success: true},
			capiTemplateName: {Success: true},
		}),
	)

	createAndUpdateTests = []testCase{
		{
			name:              "should fail if the template is unset",
			oldManagedCluster: managedcluster.NewManagedCluster(managedcluster.WithTemplate(templateUpgradeSource)),
			newManagedCluster: managedcluster.NewManagedCluster(),
			err:               "the ManagedCluster is invalid: clustertemplates.hmc.mirantis.com \"\" not found",
		},
		{
			name:              "should fail if the ClusterTemplate is not found in the ManagedCluster's namespace",
			oldManagedCluster: managedcluster.NewManagedCluster(managedcluster.WithTemplate(templateUpgradeSource)),
			newManagedCluster: managedcluster.NewManagedCluster(managedcluster.WithTemplate(testTemplateName)),
			existingObjects: []runtime.Object{
				mgmt,
				template.NewClusterTemplate(
					template.WithName(testTemplateName),
					template.WithNamespace(testNamespace),
				),
			},
			err: fmt.Sprintf("the ManagedCluster is invalid: clustertemplates.hmc.mirantis.com \"%s\" not found", testTemplateName),
		},
		{
			name:              "should fail if the cluster template was found but is invalid (some validation error)",
			oldManagedCluster: managedcluster.NewManagedCluster(managedcluster.WithTemplate(templateUpgradeSource)),
			newManagedCluster: managedcluster.NewManagedCluster(managedcluster.WithTemplate(testTemplateName)),
			existingObjects: []runtime.Object{
				mgmt,
				template.NewClusterTemplate(
					template.WithName(testTemplateName),
					template.WithValidationStatus(v1alpha1.TemplateValidationStatus{
						Valid:           false,
						ValidationError: "validation error example",
					}),
				),
			},
			err: "the ManagedCluster is invalid: the template is not valid: validation error example",
		},
		{
			name:              "should succeed",
			oldManagedCluster: managedcluster.NewManagedCluster(managedcluster.WithTemplate(templateUpgradeSource)),
			newManagedCluster: managedcluster.NewManagedCluster(managedcluster.WithTemplate(testTemplateName)),
			existingObjects: []runtime.Object{
				mgmt,
				template.NewClusterTemplate(
					template.WithName(testTemplateName),
					template.WithProvidersStatus(v1alpha1.Providers{
						InfrastructureProviders: []string{"aws"},
						BootstrapProviders:      []string{"k0s"},
						ControlPlaneProviders:   []string{"k0s"},
					}),
					template.WithValidationStatus(v1alpha1.TemplateValidationStatus{Valid: true}),
				),
			},
		},
	}
)

func TestManagedClusterValidateCreate(t *testing.T) {
	g := NewWithT(t)

	ctx := context.Background()

	tests := createAndUpdateTests
	tests = append(tests, []testCase{
		{
			name:              "should fail if one or more requested providers are not available yet",
			oldManagedCluster: managedcluster.NewManagedCluster(managedcluster.WithTemplate(templateUpgradeSource)),
			newManagedCluster: managedcluster.NewManagedCluster(managedcluster.WithTemplate(testTemplateName)),
			existingObjects: []runtime.Object{
				management.NewManagement(
					management.WithCoreComponents(&coreConfiguration),
					management.WithAvailableProviders(v1alpha1.Providers{
						InfrastructureProviders: []string{"aws"},
						BootstrapProviders:      []string{"k0s"},
					}),
					management.WithComponentsStatus(map[string]v1alpha1.ComponentStatus{
						hmcTemplateName:  {Success: true},
						capiTemplateName: {Success: true},
					}),
				),
				template.NewClusterTemplate(
					template.WithName(testTemplateName),
					template.WithProvidersStatus(v1alpha1.Providers{
						InfrastructureProviders: []string{"azure"},
						BootstrapProviders:      []string{"k0s"},
						ControlPlaneProviders:   []string{"k0s"},
					}),
					template.WithValidationStatus(v1alpha1.TemplateValidationStatus{Valid: true}),
				),
			},
			warnings: admission.Warnings{
				"not ready control plane providers: [k0s]",
				"not ready infrastructure providers: [azure]",
			},
			err: "one or more required components are not ready",
		},
		{
			name:              "should fail if one or more providers and core components are not ready yet",
			newManagedCluster: managedcluster.NewManagedCluster(managedcluster.WithTemplate(testTemplateName)),
			existingObjects: []runtime.Object{
				management.NewManagement(
					management.WithCoreComponents(&coreConfiguration),
					management.WithAvailableProviders(v1alpha1.Providers{
						InfrastructureProviders: []string{"aws"},
						BootstrapProviders:      []string{"k0s"},
					}),
					management.WithComponentsStatus(map[string]v1alpha1.ComponentStatus{
						hmcTemplateName: {Success: true},
						capiTemplateName: {
							Success: false,
							Error:   "cluster-api template is invalid",
						},
					}),
				),
				template.NewProviderTemplate(template.WithName(hmcTemplateName)),
				template.NewProviderTemplate(
					template.WithName(capiTemplateName),
					template.WithValidationStatus(v1alpha1.TemplateValidationStatus{Valid: false}),
				),
				template.NewClusterTemplate(
					template.WithName(testTemplateName),
					template.WithProvidersStatus(v1alpha1.Providers{
						InfrastructureProviders: []string{"azure"},
						BootstrapProviders:      []string{"k0s"},
						ControlPlaneProviders:   []string{"k0s"},
					}),
					template.WithValidationStatus(v1alpha1.TemplateValidationStatus{Valid: true}),
				),
			},
			warnings: admission.Warnings{
				"not ready control plane providers: [k0s]",
				"not ready core components: [cluster-api]",
				"not ready infrastructure providers: [azure]",
				"cluster-api installation failed: cluster-api template is invalid",
			},
			err: "one or more required components are not ready",
		},
		{
			name:              "should succeed if one or more providers failed but it is not required",
			newManagedCluster: managedcluster.NewManagedCluster(managedcluster.WithTemplate(testTemplateName)),
			existingObjects: []runtime.Object{
				management.NewManagement(
					management.WithCoreComponents(&coreConfiguration),
					management.WithAvailableProviders(v1alpha1.Providers{
						InfrastructureProviders: []string{"aws", "azure"},
						BootstrapProviders:      []string{"k0s"},
						ControlPlaneProviders:   []string{"k0s"},
					}),
					management.WithComponentsStatus(map[string]v1alpha1.ComponentStatus{
						hmcTemplateName:  {Success: true},
						capiTemplateName: {Success: true},
						capzTemplateName: {
							Success: false,
							Error:   "cluster API provider Azure deployment failed",
						},
					}),
				),
				template.NewProviderTemplate(
					template.WithName(capzTemplateName),
					template.WithValidationStatus(v1alpha1.TemplateValidationStatus{Valid: false}),
				),
				template.NewClusterTemplate(
					template.WithName(testTemplateName),
					template.WithProvidersStatus(v1alpha1.Providers{
						InfrastructureProviders: []string{"aws"},
						BootstrapProviders:      []string{"k0s"},
						ControlPlaneProviders:   []string{"k0s"},
					}),
					template.WithValidationStatus(v1alpha1.TemplateValidationStatus{Valid: true}),
				),
			},
		},
	}...)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := fake.NewClientBuilder().WithScheme(scheme.Scheme).WithRuntimeObjects(tt.existingObjects...).Build()
			validator := &ManagedClusterValidator{Client: c}
			warn, err := validator.ValidateCreate(ctx, tt.newManagedCluster)
			if tt.err != "" {
				g.Expect(err).To(HaveOccurred())
				if err.Error() != tt.err {
					t.Fatalf("expected error '%s', got error: %s", tt.err, err.Error())
				}
			} else {
				g.Expect(err).To(Succeed())
			}
			if len(tt.warnings) > 0 {
				g.Expect(warn).To(Equal(tt.warnings))
			} else {
				g.Expect(warn).To(BeEmpty())
			}
		})
	}
}

func TestManagedClusterValidateUpdate(t *testing.T) {
	g := NewWithT(t)

	ctx := context.Background()
	for _, tt := range createAndUpdateTests {
		t.Run(tt.name, func(t *testing.T) {
			c := fake.NewClientBuilder().WithScheme(scheme.Scheme).WithRuntimeObjects(tt.existingObjects...).Build()
			validator := &ManagedClusterValidator{Client: c}
			warn, err := validator.ValidateUpdate(ctx, tt.oldManagedCluster, tt.newManagedCluster)
			if tt.err != "" {
				g.Expect(err).To(HaveOccurred())
				if err.Error() != tt.err {
					t.Fatalf("expected error '%s', got error: %s", tt.err, err.Error())
				}
			} else {
				g.Expect(err).To(Succeed())
			}
			if len(tt.warnings) > 0 {
				g.Expect(warn).To(Equal(tt.warnings))
			} else {
				g.Expect(warn).To(BeEmpty())
			}
		})
	}
}

func TestManagedClusterDefault(t *testing.T) {
	g := NewWithT(t)

	ctx := context.Background()

	managedClusterConfig := `{"foo":"bar"}`

	tests := []struct {
		name            string
		input           *v1alpha1.ManagedCluster
		output          *v1alpha1.ManagedCluster
		existingObjects []runtime.Object
		err             string
	}{
		{
			name:   "should not set defaults if the config is provided",
			input:  managedcluster.NewManagedCluster(managedcluster.WithConfig(managedClusterConfig)),
			output: managedcluster.NewManagedCluster(managedcluster.WithConfig(managedClusterConfig)),
		},
		{
			name:   "should not set defaults: template is invalid",
			input:  managedcluster.NewManagedCluster(managedcluster.WithTemplate(testTemplateName)),
			output: managedcluster.NewManagedCluster(managedcluster.WithTemplate(testTemplateName)),
			existingObjects: []runtime.Object{
				mgmt,
				template.NewClusterTemplate(
					template.WithName(testTemplateName),
					template.WithValidationStatus(v1alpha1.TemplateValidationStatus{
						Valid:           false,
						ValidationError: "validation error example",
					}),
				),
			},
			err: "template is invalid: the template is not valid: validation error example",
		},
		{
			name:   "should not set defaults: config in template status is unset",
			input:  managedcluster.NewManagedCluster(managedcluster.WithTemplate(testTemplateName)),
			output: managedcluster.NewManagedCluster(managedcluster.WithTemplate(testTemplateName)),
			existingObjects: []runtime.Object{
				mgmt,
				template.NewClusterTemplate(
					template.WithName(testTemplateName),
					template.WithValidationStatus(v1alpha1.TemplateValidationStatus{Valid: true}),
				),
			},
		},
		{
			name:  "should set defaults",
			input: managedcluster.NewManagedCluster(managedcluster.WithTemplate(testTemplateName)),
			output: managedcluster.NewManagedCluster(
				managedcluster.WithTemplate(testTemplateName),
				managedcluster.WithConfig(managedClusterConfig),
				managedcluster.WithDryRun(true),
			),
			existingObjects: []runtime.Object{
				mgmt,
				template.NewClusterTemplate(
					template.WithName(testTemplateName),
					template.WithValidationStatus(v1alpha1.TemplateValidationStatus{Valid: true}),
					template.WithConfigStatus(managedClusterConfig),
				),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := fake.NewClientBuilder().WithScheme(scheme.Scheme).WithRuntimeObjects(tt.existingObjects...).Build()
			validator := &ManagedClusterValidator{Client: c}
			err := validator.Default(ctx, tt.input)
			if tt.err != "" {
				g.Expect(err).To(HaveOccurred())
				if err.Error() != tt.err {
					t.Fatalf("expected error '%s', got error: %s", tt.err, err.Error())
				}
			} else {
				g.Expect(err).To(Succeed())
			}
			g.Expect(tt.input).To(Equal(tt.output))
		})
	}
}
