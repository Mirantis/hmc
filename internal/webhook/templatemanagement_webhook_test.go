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
	"testing"

	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/Mirantis/hmc/api/v1alpha1"
	"github.com/Mirantis/hmc/internal/utils"
	"github.com/Mirantis/hmc/test/objects/managedcluster"
	"github.com/Mirantis/hmc/test/objects/template"
	chain "github.com/Mirantis/hmc/test/objects/templatechain"
	tm "github.com/Mirantis/hmc/test/objects/templatemanagement"
	"github.com/Mirantis/hmc/test/scheme"
)

func TestTemplateManagementValidateUpdate(t *testing.T) {
	g := NewWithT(t)

	ctx := context.Background()

	namespaceDevName := "dev"
	namespaceProdName := "prod"

	namespaceDev := &v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: namespaceDevName,
			Labels: map[string]string{
				"environment": "dev",
			},
		},
	}
	namespaceProd := &v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: namespaceProdName,
			Labels: map[string]string{
				"environment": "prod",
			},
		},
	}

	awsCtChainName := "aws-chain"
	azureCtChainName := "azure-chain"
	awsStandaloneCpTemplateName := "aws-standalone-cp"
	awsHostedCpTemplateName := "aws-hosted-cp"
	azureStandaloneCpTemplateName := "azure-standalone-cp"
	azureHostedCpTemplateName := "azure-hosted-cp"

	awsAccessRule := v1alpha1.AccessRule{
		TargetNamespaces: v1alpha1.TargetNamespaces{
			StringSelector: "environment=dev",
		},
		ClusterTemplateChains: []string{awsCtChainName},
	}

	azureProdAccessRule := v1alpha1.AccessRule{
		TargetNamespaces: v1alpha1.TargetNamespaces{
			Selector: &metav1.LabelSelector{
				MatchExpressions: []metav1.LabelSelectorRequirement{
					{
						Key:      "environment",
						Operator: metav1.LabelSelectorOpIn,
						Values:   []string{"prod"},
					},
				},
			},
		},
		ClusterTemplateChains: []string{azureCtChainName},
	}

	awsCtChain := chain.NewClusterTemplateChain(chain.WithName(awsCtChainName), chain.WithSupportedTemplates(
		[]v1alpha1.SupportedTemplate{
			{Name: awsStandaloneCpTemplateName},
			{Name: awsHostedCpTemplateName},
		},
	))
	azureCtChain := chain.NewClusterTemplateChain(chain.WithName(azureCtChainName), chain.WithSupportedTemplates(
		[]v1alpha1.SupportedTemplate{
			{Name: azureStandaloneCpTemplateName},
			{Name: azureHostedCpTemplateName},
		},
	))

	hmcClusterTemplate := template.WithLabels(map[string]string{v1alpha1.HMCManagedLabelKey: v1alpha1.HMCManagedLabelValue})

	tests := []struct {
		name            string
		newTm           *v1alpha1.TemplateManagement
		existingObjects []runtime.Object
		err             string
		warnings        admission.Warnings
	}{
		{
			name:  `should fail if new access rules require to remove the in-used ClusterTemplate (only aws in dev namespace is allowed)`,
			newTm: tm.NewTemplateManagement(tm.WithAccessRules([]v1alpha1.AccessRule{awsAccessRule})),
			existingObjects: []runtime.Object{
				awsCtChain,
				azureCtChain,
				template.NewClusterTemplate(hmcClusterTemplate, template.WithNamespace(namespaceDevName), template.WithName(awsStandaloneCpTemplateName)),
				template.NewClusterTemplate(hmcClusterTemplate, template.WithNamespace(namespaceProdName), template.WithName(awsHostedCpTemplateName)),
				template.NewClusterTemplate(hmcClusterTemplate, template.WithNamespace(namespaceDevName), template.WithName(azureStandaloneCpTemplateName)),
				template.NewClusterTemplate(hmcClusterTemplate, template.WithNamespace(namespaceProdName), template.WithName(azureHostedCpTemplateName)),
				template.NewClusterTemplate(template.WithName("unmanaged")),
				managedcluster.NewManagedCluster(
					managedcluster.WithName("aws-standalone"),
					managedcluster.WithNamespace(namespaceDevName),
					managedcluster.WithTemplate(awsStandaloneCpTemplateName),
				),
				managedcluster.NewManagedCluster(
					managedcluster.WithName("aws-hosted"),
					managedcluster.WithNamespace(namespaceProdName),
					managedcluster.WithTemplate(awsHostedCpTemplateName),
				),
				managedcluster.NewManagedCluster(
					managedcluster.WithName("azure-standalone"),
					managedcluster.WithNamespace(namespaceDevName),
					managedcluster.WithTemplate(azureStandaloneCpTemplateName),
				),
				managedcluster.NewManagedCluster(
					managedcluster.WithName("azure-hosted-1"),
					managedcluster.WithNamespace(namespaceProdName),
					managedcluster.WithTemplate(azureHostedCpTemplateName),
				),
				managedcluster.NewManagedCluster(
					managedcluster.WithName("azure-hosted-2"),
					managedcluster.WithNamespace(namespaceProdName),
					managedcluster.WithTemplate(azureHostedCpTemplateName),
				),
			},
			warnings: admission.Warnings{
				"ClusterTemplate \"dev/azure-standalone-cp\" can't be removed: found ManagedClusters that reference it: \"dev/azure-standalone\"",
				"ClusterTemplate \"prod/aws-hosted-cp\" can't be removed: found ManagedClusters that reference it: \"prod/aws-hosted\"",
				"ClusterTemplate \"prod/azure-hosted-cp\" can't be removed: found ManagedClusters that reference it: \"prod/azure-hosted-1\", \"prod/azure-hosted-2\"",
			},
			err: "can not apply new access rules",
		},
		{
			name:  "should succeed if new access rules don't affect in-used ClusterTemplates (only azure hosted in prod namespace is allowed)",
			newTm: tm.NewTemplateManagement(tm.WithAccessRules([]v1alpha1.AccessRule{azureProdAccessRule})),
			existingObjects: []runtime.Object{
				awsCtChain,
				azureCtChain,
				template.NewClusterTemplate(hmcClusterTemplate, template.WithNamespace(namespaceDevName), template.WithName(awsStandaloneCpTemplateName)),
				template.NewClusterTemplate(hmcClusterTemplate, template.WithNamespace(namespaceProdName), template.WithName(awsHostedCpTemplateName)),
				template.NewClusterTemplate(hmcClusterTemplate, template.WithNamespace(namespaceDevName), template.WithName(azureStandaloneCpTemplateName)),
				template.NewClusterTemplate(hmcClusterTemplate, template.WithNamespace(namespaceProdName), template.WithName(azureHostedCpTemplateName)),
				template.NewClusterTemplate(template.WithName("unmanaged")),
				managedcluster.NewManagedCluster(
					managedcluster.WithName("azure-hosted-1"),
					managedcluster.WithNamespace(namespaceProdName),
					managedcluster.WithTemplate(azureHostedCpTemplateName),
				),
				managedcluster.NewManagedCluster(
					managedcluster.WithName("azure-hosted-2"),
					managedcluster.WithNamespace(namespaceProdName),
					managedcluster.WithTemplate(azureHostedCpTemplateName),
				),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			obj := []runtime.Object{namespaceDev, namespaceProd}
			c := fake.NewClientBuilder().
				WithScheme(scheme.Scheme).
				WithRuntimeObjects(append(obj, tt.existingObjects...)...).
				WithIndex(&v1alpha1.ManagedCluster{}, TemplateKey, ExtractTemplateName).
				Build()
			validator := &TemplateManagementValidator{Client: c, SystemNamespace: utils.DefaultSystemNamespace}
			warn, err := validator.ValidateUpdate(ctx, tm.NewTemplateManagement(), tt.newTm)
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
