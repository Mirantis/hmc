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

package managedcluster

import (
	"context"
	_ "embed"
	"fmt"
	"os"
	"strings"

	"github.com/a8m/envsubst"
	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"gopkg.in/yaml.v3"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/env"

	"github.com/Mirantis/hmc/test/e2e/kubeclient"
	"github.com/Mirantis/hmc/test/utils"
)

type ProviderType string

const (
	ProviderCAPI                  ProviderType = "cluster-api"
	ProviderAWS                   ProviderType = "infrastructure-aws"
	ProviderAzure                 ProviderType = "infrastructure-azure"
	ProviderVSphere               ProviderType = "infrastructure-vsphere"
	ProviderK0smotron             ProviderType = "infrastructure-k0sproject-k0smotron"
	ProviderK0smotronBootstrap    ProviderType = "bootstrap-k0sproject-k0smotron"
	ProviderK0smotronControlPlane ProviderType = "control-plane-k0sproject-k0smotron"

	providerLabel = "cluster.x-k8s.io/provider"
)

type Template string

const (
	TemplateAWSStandaloneCP     Template = "aws-standalone-cp"
	TemplateAWSHostedCP         Template = "aws-hosted-cp"
	TemplateAzureHostedCP       Template = "azure-hosted-cp"
	TemplateAzureStandaloneCP   Template = "azure-standalone-cp"
	TemplateVSphereStandaloneCP Template = "vsphere-standalone-cp"
	TemplateVSphereHostedCP     Template = "vsphere-hosted-cp"
)

//go:embed resources/aws-standalone-cp.yaml.tpl
var awsStandaloneCPManagedClusterTemplateBytes []byte

//go:embed resources/aws-hosted-cp.yaml.tpl
var awsHostedCPManagedClusterTemplateBytes []byte

//go:embed resources/azure-standalone-cp.yaml.tpl
var azureStandaloneCPManagedClusterTemplateBytes []byte

//go:embed resources/azure-hosted-cp.yaml.tpl
var azureHostedCPManagedClusterTemplateBytes []byte

//go:embed resources/vsphere-standalone-cp.yaml.tpl
var vsphereStandaloneCPManagedClusterTemplateBytes []byte

//go:embed resources/vsphere-hosted-cp.yaml.tpl
var vsphereHostedCPManagedClusterTemplateBytes []byte

func FilterAllProviders() []string {
	return []string{
		utils.HMCControllerLabel,
		GetProviderLabel(ProviderAWS),
		GetProviderLabel(ProviderAzure),
		GetProviderLabel(ProviderCAPI),
		GetProviderLabel(ProviderVSphere),
	}
}

func GetProviderLabel(provider ProviderType) string {
	return fmt.Sprintf("%s=%s", providerLabel, provider)
}

func setClusterName(templateName Template) {
	var generatedName string

	mcName := os.Getenv(EnvVarManagedClusterName)
	if mcName == "" {
		mcName = "e2e-test-" + uuid.New().String()[:8]
	}

	providerName := strings.Split(string(templateName), "-")[0]

	// Append the provider name to the cluster name to ensure uniqueness between
	// different deployed ManagedClusters.
	generatedName = fmt.Sprintf("%s-%s", mcName, providerName)
	if strings.Contains(string(templateName), "hosted") {
		generatedName = fmt.Sprintf("%s-%s", mcName, "hosted")
	}

	GinkgoT().Setenv(EnvVarManagedClusterName, generatedName)
}

// GetUnstructured returns an unstructured ManagedCluster object based on the
// provider and template.
func GetUnstructured(templateName Template) *unstructured.Unstructured {
	GinkgoHelper()

	setClusterName(templateName)

	var managedClusterTemplateBytes []byte
	switch templateName {
	case TemplateAWSStandaloneCP:
		managedClusterTemplateBytes = awsStandaloneCPManagedClusterTemplateBytes
	case TemplateAWSHostedCP:
		// Validate environment vars that do not have defaults are populated.
		// We perform this validation here instead of within a Before block
		// since we populate the vars from standalone prior to this step.
		ValidateDeploymentVars([]string{
			EnvVarAWSVPCID,
			EnvVarAWSSubnets,
		})
		managedClusterTemplateBytes = awsHostedCPManagedClusterTemplateBytes
	case TemplateVSphereStandaloneCP:
		managedClusterTemplateBytes = vsphereStandaloneCPManagedClusterTemplateBytes
	case TemplateVSphereHostedCP:
		managedClusterTemplateBytes = vsphereHostedCPManagedClusterTemplateBytes
	case TemplateAzureHostedCP:
		managedClusterTemplateBytes = azureHostedCPManagedClusterTemplateBytes
	case TemplateAzureStandaloneCP:
		managedClusterTemplateBytes = azureStandaloneCPManagedClusterTemplateBytes
	default:
		Fail(fmt.Sprintf("Unsupported template: %s", templateName))
	}

	version := env.GetString("VERSION", "")
	index := strings.LastIndex(env.GetString("VERSION", ""), "-")
	if index > 0 {
		version = version[index:]
	}
	GinkgoT().Setenv("BUILD_VERSION", version)

	managedClusterConfigBytes, err := envsubst.Bytes(managedClusterTemplateBytes)
	Expect(err).NotTo(HaveOccurred(), "failed to substitute environment variables")

	var managedClusterConfig map[string]any
	By(fmt.Sprintf("Cluster being applied\n %s", managedClusterConfigBytes))
	err = yaml.Unmarshal(managedClusterConfigBytes, &managedClusterConfig)
	Expect(err).NotTo(HaveOccurred(), "failed to unmarshal deployment config")

	return &unstructured.Unstructured{Object: managedClusterConfig}
}

func ValidateClusterTemplates(ctx context.Context, client *kubeclient.KubeClient) error {
	templates, err := client.ListClusterTemplates(ctx)
	if err != nil {
		return fmt.Errorf("failed to list cluster templates: %w", err)
	}

	for _, template := range templates {
		valid, found, err := unstructured.NestedBool(template.Object, "status", "valid")
		if err != nil {
			return fmt.Errorf("failed to get valid flag for template %s: %w", template.GetName(), err)
		}

		if !found {
			return fmt.Errorf("valid flag for template %s not found", template.GetName())
		}

		if !valid {
			return fmt.Errorf("template %s is still invalid", template.GetName())
		}
	}

	return nil
}

func ValidateDeploymentVars(v []string) {
	GinkgoHelper()

	for _, envVar := range v {
		Expect(os.Getenv(envVar)).NotTo(BeEmpty(), envVar+" must be set")
	}
}
