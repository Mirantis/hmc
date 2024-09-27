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
	chain "github.com/Mirantis/hmc/test/objects/templatechain"
	tm "github.com/Mirantis/hmc/test/objects/templatemanagement"
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
	templateUpgradeTarget = "template-1-0-1"

	ctChainName = "test"
)

var (
	mgmt = management.NewManagement(
		management.WithAvailableProviders(v1alpha1.Providers{
			InfrastructureProviders: []string{"aws"},
			BootstrapProviders:      []string{"k0s"},
			ControlPlaneProviders:   []string{"k0s"},
		}),
	)

	accessRules = []v1alpha1.AccessRule{
		{
			ClusterTemplateChains: []string{ctChainName},
		},
	}

	supportedTemplates = []v1alpha1.SupportedTemplate{
		{
			Name: testTemplateName,
		},
		{
			Name: templateUpgradeSource,
			AvailableUpgrades: []v1alpha1.AvailableUpgrade{
				{
					Name: templateUpgradeTarget,
				},
			},
		},
	}

	withAWSK0sProviders = template.WithProvidersStatus(v1alpha1.Providers{
		InfrastructureProviders: []string{"aws"},
		BootstrapProviders:      []string{"k0s"},
		ControlPlaneProviders:   []string{"k0s"},
	})
	withValidStatus = template.WithValidationStatus(v1alpha1.TemplateValidationStatus{Valid: true})
	managedByHMC    = template.ManagedByHMC()

	tmObj   = tm.NewTemplateManagement(tm.WithAccessRules(accessRules))
	ctChain = chain.NewClusterTemplateChain(chain.WithName(ctChainName), chain.WithSupportedTemplates(supportedTemplates))

	createAndUpdateTests = []testCase{
		{
			name:              "should fail if the ClusterTemplate is not found in the ManagedCluster's namespace",
			oldManagedCluster: managedcluster.NewManagedCluster(managedcluster.WithTemplate(templateUpgradeSource)),
			newManagedCluster: managedcluster.NewManagedCluster(managedcluster.WithTemplate(templateUpgradeTarget)),
			existingObjects: []runtime.Object{
				mgmt,
				template.NewClusterTemplate(template.WithName(templateUpgradeSource), template.WithNamespace(testNamespace)),
				template.NewClusterTemplate(template.WithName(templateUpgradeTarget), template.WithNamespace(testNamespace)),
			},
			err: fmt.Sprintf("the ManagedCluster is invalid: clustertemplates.hmc.mirantis.com \"%s\" not found", templateUpgradeTarget),
		},
		{
			name:              "should fail if the cluster template was found but is invalid (some validation error)",
			oldManagedCluster: managedcluster.NewManagedCluster(managedcluster.WithTemplate(templateUpgradeSource)),
			newManagedCluster: managedcluster.NewManagedCluster(managedcluster.WithTemplate(templateUpgradeTarget)),
			existingObjects: []runtime.Object{
				mgmt,
				template.NewClusterTemplate(
					template.WithName(templateUpgradeTarget),
					template.WithValidationStatus(v1alpha1.TemplateValidationStatus{
						Valid:           false,
						ValidationError: "validation error example",
					}),
				),
			},
			err: "the ManagedCluster is invalid: the template is not valid: validation error example",
		},
		{
			name:              "should fail if one or more requested providers are not available yet",
			oldManagedCluster: managedcluster.NewManagedCluster(managedcluster.WithTemplate(templateUpgradeSource)),
			newManagedCluster: managedcluster.NewManagedCluster(managedcluster.WithTemplate(templateUpgradeTarget)),
			existingObjects: []runtime.Object{
				management.NewManagement(
					management.WithAvailableProviders(v1alpha1.Providers{
						InfrastructureProviders: []string{"aws"},
						BootstrapProviders:      []string{"k0s"},
					}),
				),
				template.NewClusterTemplate(
					template.WithName(templateUpgradeTarget),
					template.WithProvidersStatus(v1alpha1.Providers{
						InfrastructureProviders: []string{"azure"},
						BootstrapProviders:      []string{"k0s"},
						ControlPlaneProviders:   []string{"k0s"},
					}),
					template.WithValidationStatus(v1alpha1.TemplateValidationStatus{Valid: true}),
				),
			},
			err: "the ManagedCluster is invalid: providers verification failed: one or more required control plane providers are not deployed yet: [k0s]\none or more required infrastructure providers are not deployed yet: [azure]",
		},
		{
			name:              "should succeed",
			oldManagedCluster: managedcluster.NewManagedCluster(managedcluster.WithTemplate(templateUpgradeSource)),
			newManagedCluster: managedcluster.NewManagedCluster(managedcluster.WithTemplate(templateUpgradeTarget)),
			existingObjects: []runtime.Object{
				mgmt,
				template.NewClusterTemplate(template.WithName(templateUpgradeTarget), withAWSK0sProviders, withValidStatus),
			},
		},
	}
)

func TestManagedClusterValidateCreate(t *testing.T) {
	g := NewWithT(t)

	ctx := context.Background()

	for _, tt := range createAndUpdateTests {
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

	tests := []testCase{
		{
			name:              "should fail if the target template is not available for update",
			oldManagedCluster: managedcluster.NewManagedCluster(managedcluster.WithTemplate(templateUpgradeSource)),
			newManagedCluster: managedcluster.NewManagedCluster(managedcluster.WithTemplate(testTemplateName)),
			existingObjects: []runtime.Object{
				mgmt,
				template.NewClusterTemplate(template.WithName(testTemplateName), managedByHMC, withAWSK0sProviders, withValidStatus),
			},
			err: fmt.Sprintf("the ManagedCluster upgrade is forbidden: ClusterTemplate %s is not available for the upgrade from %s", testTemplateName, templateUpgradeSource),
		},
		{
			name:              "should succeed - template is not managed by HMC, skipping available upgrades check",
			oldManagedCluster: managedcluster.NewManagedCluster(managedcluster.WithTemplate(templateUpgradeSource)),
			newManagedCluster: managedcluster.NewManagedCluster(managedcluster.WithTemplate(testTemplateName)),
			existingObjects: []runtime.Object{
				mgmt,
				template.NewClusterTemplate(template.WithName(testTemplateName), withAWSK0sProviders, withValidStatus),
			},
		},
		{
			name:              "should succeed - cluster is in dry run mode, skipping available upgrades check",
			oldManagedCluster: managedcluster.NewManagedCluster(managedcluster.WithTemplate(templateUpgradeSource), managedcluster.WithDryRun(true)),
			newManagedCluster: managedcluster.NewManagedCluster(managedcluster.WithTemplate(testTemplateName), managedcluster.WithDryRun(true)),
			existingObjects: []runtime.Object{
				mgmt,
				template.NewClusterTemplate(template.WithName(testTemplateName), managedByHMC, withAWSK0sProviders, withValidStatus),
			},
		},
		{
			name:              "should succeed - template is available for upgrade and valid",
			oldManagedCluster: managedcluster.NewManagedCluster(managedcluster.WithTemplate(templateUpgradeSource)),
			newManagedCluster: managedcluster.NewManagedCluster(managedcluster.WithTemplate(templateUpgradeTarget)),
			existingObjects: []runtime.Object{
				mgmt,
				template.NewClusterTemplate(template.WithName(templateUpgradeTarget), managedByHMC, withAWSK0sProviders, withValidStatus),
			},
		},
	}
	tests = append(tests, createAndUpdateTests...)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.existingObjects = append(tt.existingObjects, tmObj, ctChain)
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
