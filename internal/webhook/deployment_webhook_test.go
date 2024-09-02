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
	"github.com/Mirantis/hmc/test/objects/deployment"
	"github.com/Mirantis/hmc/test/objects/management"
	"github.com/Mirantis/hmc/test/objects/template"
	"github.com/Mirantis/hmc/test/scheme"
)

var (
	testTemplateName = "template-test"

	mgmt = management.NewManagement(
		management.WithAvailableProviders(v1alpha1.Providers{
			InfrastructureProviders: []string{"aws"},
			BootstrapProviders:      []string{"k0s"},
			ControlPlaneProviders:   []string{"k0s"},
		}),
	)

	createAndUpdateTests = []struct {
		name            string
		deployment      *v1alpha1.Deployment
		existingObjects []runtime.Object
		err             string
		warnings        admission.Warnings
	}{
		{
			name:       "should fail if the template is unset",
			deployment: deployment.NewDeployment(),
			err:        "the deployment is invalid: templates.hmc.mirantis.com \"\" not found",
		},
		{
			name:       "should fail if the template is not found in hmc-system namespace",
			deployment: deployment.NewDeployment(deployment.WithTemplate(testTemplateName)),
			existingObjects: []runtime.Object{
				mgmt,
				template.NewTemplate(
					template.WithName(testTemplateName),
					template.WithNamespace("default"),
				),
			},
			err: fmt.Sprintf("the deployment is invalid: templates.hmc.mirantis.com \"%s\" not found", testTemplateName),
		},
		{
			name:       "should fail if the template was found but is invalid (type is unset)",
			deployment: deployment.NewDeployment(deployment.WithTemplate(testTemplateName)),
			existingObjects: []runtime.Object{
				mgmt,
				template.NewTemplate(template.WithName(testTemplateName)),
			},
			err: "the deployment is invalid: the template should be of the deployment type. Current: ",
		},
		{
			name:       "should fail if the template was found but is invalid (some validation error)",
			deployment: deployment.NewDeployment(deployment.WithTemplate(testTemplateName)),
			existingObjects: []runtime.Object{
				mgmt,
				template.NewTemplate(
					template.WithName(testTemplateName),
					template.WithTypeStatus(v1alpha1.TemplateTypeDeployment),
					template.WithValidationStatus(v1alpha1.TemplateValidationStatus{
						Valid:           false,
						ValidationError: "validation error example",
					}),
				),
			},
			err: "the deployment is invalid: the template is not valid: validation error example",
		},
		{
			name:       "should fail if one or more requested providers are not available yet",
			deployment: deployment.NewDeployment(deployment.WithTemplate(testTemplateName)),
			existingObjects: []runtime.Object{
				management.NewManagement(
					management.WithAvailableProviders(v1alpha1.Providers{
						InfrastructureProviders: []string{"aws"},
						BootstrapProviders:      []string{"k0s"},
					}),
				),
				template.NewTemplate(
					template.WithName(testTemplateName),
					template.WithTypeStatus(v1alpha1.TemplateTypeDeployment),
					template.WithProvidersStatus(v1alpha1.Providers{
						InfrastructureProviders: []string{"azure"},
						BootstrapProviders:      []string{"k0s"},
						ControlPlaneProviders:   []string{"k0s"},
					}),
					template.WithValidationStatus(v1alpha1.TemplateValidationStatus{Valid: true}),
				),
			},
			err: "the deployment is invalid: providers verification failed: one or more required control plane providers are not deployed yet: [k0s]\none or more required infrastructure providers are not deployed yet: [azure]",
		},
		{
			name:       "should succeed",
			deployment: deployment.NewDeployment(deployment.WithTemplate(testTemplateName)),
			existingObjects: []runtime.Object{
				mgmt,
				template.NewTemplate(
					template.WithName(testTemplateName),
					template.WithTypeStatus(v1alpha1.TemplateTypeDeployment),
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

func TestDeploymentValidateCreate(t *testing.T) {
	g := NewWithT(t)

	ctx := context.Background()
	for _, tt := range createAndUpdateTests {
		t.Run(tt.name, func(t *testing.T) {
			c := fake.NewClientBuilder().WithScheme(scheme.Scheme).WithRuntimeObjects(tt.existingObjects...).Build()
			validator := &DeploymentValidator{Client: c}
			warn, err := validator.ValidateCreate(ctx, tt.deployment)
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

func TestDeploymentValidateUpdate(t *testing.T) {
	g := NewWithT(t)

	ctx := context.Background()
	for _, tt := range createAndUpdateTests {
		t.Run(tt.name, func(t *testing.T) {
			c := fake.NewClientBuilder().WithScheme(scheme.Scheme).WithRuntimeObjects(tt.existingObjects...).Build()
			validator := &DeploymentValidator{Client: c}
			warn, err := validator.ValidateUpdate(ctx, deployment.NewDeployment(), tt.deployment)
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

func TestDeploymentDefault(t *testing.T) {
	g := NewWithT(t)

	ctx := context.Background()

	deploymentConfig := `{"foo":"bar"}`

	tests := []struct {
		name            string
		input           *v1alpha1.Deployment
		output          *v1alpha1.Deployment
		existingObjects []runtime.Object
		err             string
	}{
		{
			name:   "should not set defaults if the config is provided",
			input:  deployment.NewDeployment(deployment.WithConfig(deploymentConfig)),
			output: deployment.NewDeployment(deployment.WithConfig(deploymentConfig)),
		},
		{
			name:   "should not set defaults: template is invalid",
			input:  deployment.NewDeployment(deployment.WithTemplate(testTemplateName)),
			output: deployment.NewDeployment(deployment.WithTemplate(testTemplateName)),
			existingObjects: []runtime.Object{
				mgmt,
				template.NewTemplate(
					template.WithName(testTemplateName),
					template.WithTypeStatus(v1alpha1.TemplateTypeDeployment),
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
			input:  deployment.NewDeployment(deployment.WithTemplate(testTemplateName)),
			output: deployment.NewDeployment(deployment.WithTemplate(testTemplateName)),
			existingObjects: []runtime.Object{
				mgmt,
				template.NewTemplate(
					template.WithName(testTemplateName),
					template.WithTypeStatus(v1alpha1.TemplateTypeDeployment),
					template.WithValidationStatus(v1alpha1.TemplateValidationStatus{Valid: true}),
				),
			},
		},
		{
			name:  "should set defaults",
			input: deployment.NewDeployment(deployment.WithTemplate(testTemplateName)),
			output: deployment.NewDeployment(
				deployment.WithTemplate(testTemplateName),
				deployment.WithConfig(deploymentConfig),
				deployment.WithDryRun(true),
			),
			existingObjects: []runtime.Object{
				mgmt,
				template.NewTemplate(
					template.WithName(testTemplateName),
					template.WithTypeStatus(v1alpha1.TemplateTypeDeployment),
					template.WithValidationStatus(v1alpha1.TemplateValidationStatus{Valid: true}),
					template.WithConfigStatus(deploymentConfig),
				),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := fake.NewClientBuilder().WithScheme(scheme.Scheme).WithRuntimeObjects(tt.existingObjects...).Build()
			validator := &DeploymentValidator{Client: c}
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
