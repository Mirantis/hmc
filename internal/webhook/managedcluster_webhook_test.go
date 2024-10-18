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
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/Mirantis/hmc/api/v1alpha1"
	"github.com/Mirantis/hmc/test/objects/credential"
	"github.com/Mirantis/hmc/test/objects/managedcluster"
	"github.com/Mirantis/hmc/test/objects/management"
	"github.com/Mirantis/hmc/test/objects/template"
	"github.com/Mirantis/hmc/test/scheme"
)

var (
	testTemplateName   = "template-test"
	testCredentialName = "cred-test"
	newTemplateName    = "new-template-name"

	testNamespace = "test"

	mgmt = management.NewManagement(
		management.WithAvailableProviders(v1alpha1.ProvidersTupled{
			{Name: "infrastructure-aws"},
			{Name: "control-plane-k0s"},
			{Name: "bootstrap-k0s"},
		}),
	)

	cred = credential.NewCredential(
		credential.WithName(testCredentialName),
		credential.WithState(v1alpha1.CredentialReady),
		credential.WithIdentityRef(
			&corev1.ObjectReference{
				Kind: "AWSClusterStaticIdentity",
				Name: "awsclid",
			}),
	)
)

func TestManagedClusterValidateCreate(t *testing.T) {
	g := NewWithT(t)

	ctx := admission.NewContextWithRequest(context.Background(), admission.Request{
		AdmissionRequest: admissionv1.AdmissionRequest{
			Operation: admissionv1.Create,
		},
	})

	tests := []struct {
		name            string
		managedCluster  *v1alpha1.ManagedCluster
		existingObjects []runtime.Object
		err             string
		warnings        admission.Warnings
	}{
		{
			name:           "should fail if the template is unset",
			managedCluster: managedcluster.NewManagedCluster(),
			err:            "the ManagedCluster is invalid: clustertemplates.hmc.mirantis.com \"\" not found",
		},
		{
			name: "should fail if the ClusterTemplate is not found in the ManagedCluster's namespace",
			managedCluster: managedcluster.NewManagedCluster(
				managedcluster.WithClusterTemplate(testTemplateName),
				managedcluster.WithCredential(testCredentialName),
			),
			existingObjects: []runtime.Object{
				mgmt,
				cred,
				template.NewClusterTemplate(
					template.WithName(testTemplateName),
					template.WithNamespace(testNamespace),
				),
			},
			err: fmt.Sprintf("the ManagedCluster is invalid: clustertemplates.hmc.mirantis.com \"%s\" not found", testTemplateName),
		},
		{
			name: "should fail if the cluster template was found but is invalid (some validation error)",
			managedCluster: managedcluster.NewManagedCluster(
				managedcluster.WithClusterTemplate(testTemplateName),
				managedcluster.WithCredential(testCredentialName),
			),
			existingObjects: []runtime.Object{
				mgmt,
				cred,
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
			name: "should succeed",
			managedCluster: managedcluster.NewManagedCluster(
				managedcluster.WithClusterTemplate(testTemplateName),
				managedcluster.WithCredential(testCredentialName),
			),
			existingObjects: []runtime.Object{
				mgmt,
				cred,
				template.NewClusterTemplate(
					template.WithName(testTemplateName),
					template.WithProvidersStatus(v1alpha1.ProvidersTupled{
						{Name: "infrastructure-aws"},
						{Name: "control-plane-k0s"},
						{Name: "bootstrap-k0s"},
					}),
					template.WithValidationStatus(v1alpha1.TemplateValidationStatus{Valid: true}),
				),
			},
		},
		{
			name: "cluster template k8s version does not satisfy service template constraints",
			managedCluster: managedcluster.NewManagedCluster(
				managedcluster.WithClusterTemplate(testTemplateName),
				managedcluster.WithServiceTemplate(testTemplateName),
			),
			existingObjects: []runtime.Object{
				cred,
				management.NewManagement(management.WithAvailableProviders(v1alpha1.ProvidersTupled{
					{Name: "infrastructure-aws", VersionOrConstraint: "v1.0.0"},
					{Name: "control-plane-k0s", VersionOrConstraint: "v1.0.0"},
					{Name: "bootstrap-k0s", VersionOrConstraint: "v1.0.0"},
				})),
				template.NewClusterTemplate(
					template.WithName(testTemplateName),
					template.WithValidationStatus(v1alpha1.TemplateValidationStatus{Valid: true}),
					template.WithClusterStatusK8sVersion("v1.30.0"),
				),
				template.NewServiceTemplate(
					template.WithName(testTemplateName),
					template.WithServiceK8sConstraint("<1.30"),
					template.WithValidationStatus(v1alpha1.TemplateValidationStatus{Valid: true}),
				),
			},
			err:      fmt.Sprintf(`failed to validate k8s compatibility: k8s version v1.30.0 of the ManagedCluster default/%s does not satisfy constrained version <1.30 from the ServiceTemplate default/%s`, managedcluster.DefaultName, testTemplateName),
			warnings: admission.Warnings{"Failed to validate k8s version compatibility with ServiceTemplates"},
		},
		{
			name:           "should fail if the credential is unset",
			managedCluster: managedcluster.NewManagedCluster(managedcluster.WithClusterTemplate(testTemplateName)),
			existingObjects: []runtime.Object{
				mgmt,
				template.NewClusterTemplate(
					template.WithName(testTemplateName),
					template.WithProvidersStatus(v1alpha1.ProvidersTupled{
						{Name: "infrastructure-aws"},
						{Name: "control-plane-k0s"},
						{Name: "bootstrap-k0s"},
					}),
					template.WithValidationStatus(v1alpha1.TemplateValidationStatus{Valid: true}),
				),
			},
			err: "the ManagedCluster is invalid: credentials.hmc.mirantis.com \"\" not found",
		},
		{
			name: "should fail if credential is not Ready",
			managedCluster: managedcluster.NewManagedCluster(
				managedcluster.WithClusterTemplate(testTemplateName),
				managedcluster.WithCredential(testCredentialName),
			),
			existingObjects: []runtime.Object{
				mgmt,
				credential.NewCredential(
					credential.WithName(testCredentialName),
					credential.WithState(v1alpha1.CredentialNotFound),
					credential.WithIdentityRef(
						&corev1.ObjectReference{
							Kind: "AWSClusterStaticIdentity",
							Name: "awsclid",
						}),
				),
				template.NewClusterTemplate(
					template.WithName(testTemplateName),
					template.WithProvidersStatus(v1alpha1.ProvidersTupled{
						{Name: "infrastructure-aws"},
						{Name: "control-plane-k0s"},
						{Name: "bootstrap-k0s"},
					}),
					template.WithValidationStatus(v1alpha1.TemplateValidationStatus{Valid: true}),
				),
			},
			err: "the ManagedCluster is invalid: credential is not Ready",
		},
		{
			name: "should fail if credential and template providers doesn't match",
			managedCluster: managedcluster.NewManagedCluster(
				managedcluster.WithClusterTemplate(testTemplateName),
				managedcluster.WithCredential(testCredentialName),
			),
			existingObjects: []runtime.Object{
				cred,
				management.NewManagement(
					management.WithAvailableProviders(v1alpha1.ProvidersTupled{
						{Name: "infrastructure-azure"},
						{Name: "control-plane-k0s"},
						{Name: "bootstrap-k0s"},
					}),
				),
				template.NewClusterTemplate(
					template.WithName(testTemplateName),
					template.WithProvidersStatus(v1alpha1.ProvidersTupled{
						{Name: "infrastructure-azure"},
						{Name: "control-plane-k0s"},
						{Name: "bootstrap-k0s"},
					}),
					template.WithValidationStatus(v1alpha1.TemplateValidationStatus{Valid: true}),
				),
			},
			err: "the ManagedCluster is invalid: wrong kind of the ClusterIdentity \"AWSClusterStaticIdentity\" for provider \"infrastructure-azure\"",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := fake.NewClientBuilder().WithScheme(scheme.Scheme).WithRuntimeObjects(tt.existingObjects...).Build()
			validator := &ManagedClusterValidator{Client: c}
			warn, err := validator.ValidateCreate(ctx, tt.managedCluster)
			if tt.err != "" {
				g.Expect(err).To(HaveOccurred())
				if err.Error() != tt.err {
					t.Fatalf("expected error '%s', got error: %s", tt.err, err.Error())
				}
			} else {
				g.Expect(err).To(Succeed())
			}

			g.Expect(warn).To(Equal(tt.warnings))
		})
	}
}

func TestManagedClusterValidateUpdate(t *testing.T) {
	g := NewWithT(t)

	ctx := admission.NewContextWithRequest(context.Background(), admission.Request{
		AdmissionRequest: admissionv1.AdmissionRequest{
			Operation: admissionv1.Update,
		},
	})

	tests := []struct {
		name              string
		oldManagedCluster *v1alpha1.ManagedCluster
		newManagedCluster *v1alpha1.ManagedCluster
		existingObjects   []runtime.Object
		err               string
		warnings          admission.Warnings
	}{
		{
			name:              "should fail if the new cluster template was found but is invalid (some validation error)",
			oldManagedCluster: managedcluster.NewManagedCluster(managedcluster.WithClusterTemplate(testTemplateName)),
			newManagedCluster: managedcluster.NewManagedCluster(managedcluster.WithClusterTemplate(newTemplateName)),
			existingObjects: []runtime.Object{
				mgmt,
				template.NewClusterTemplate(
					template.WithName(newTemplateName),
					template.WithValidationStatus(v1alpha1.TemplateValidationStatus{
						Valid:           false,
						ValidationError: "validation error example",
					}),
				),
			},
			err: "the ManagedCluster is invalid: the template is not valid: validation error example",
		},
		{
			name: "should succeed if template is not changed",
			oldManagedCluster: managedcluster.NewManagedCluster(
				managedcluster.WithClusterTemplate(testTemplateName),
				managedcluster.WithConfig(`{"foo":"bar"}`),
				managedcluster.WithCredential(testCredentialName),
			),
			newManagedCluster: managedcluster.NewManagedCluster(
				managedcluster.WithClusterTemplate(testTemplateName),
				managedcluster.WithConfig(`{"a":"b"}`),
				managedcluster.WithCredential(testCredentialName),
			),
			existingObjects: []runtime.Object{
				mgmt,
				cred,
				template.NewClusterTemplate(
					template.WithName(testTemplateName),
					template.WithValidationStatus(v1alpha1.TemplateValidationStatus{
						Valid:           false,
						ValidationError: "validation error example",
					}),
					template.WithProvidersStatus(v1alpha1.ProvidersTupled{
						{Name: "infrastructure-aws"},
						{Name: "control-plane-k0s"},
						{Name: "bootstrap-k0s"},
					}),
				),
			},
		},
	}
	for _, tt := range tests {
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

			g.Expect(warn).To(Equal(tt.warnings))
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
			input:  managedcluster.NewManagedCluster(managedcluster.WithClusterTemplate(testTemplateName)),
			output: managedcluster.NewManagedCluster(managedcluster.WithClusterTemplate(testTemplateName)),
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
			input:  managedcluster.NewManagedCluster(managedcluster.WithClusterTemplate(testTemplateName)),
			output: managedcluster.NewManagedCluster(managedcluster.WithClusterTemplate(testTemplateName)),
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
			input: managedcluster.NewManagedCluster(managedcluster.WithClusterTemplate(testTemplateName)),
			output: managedcluster.NewManagedCluster(
				managedcluster.WithClusterTemplate(testTemplateName),
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
