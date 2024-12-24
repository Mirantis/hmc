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

	"github.com/Mirantis/hmc/test/e2e/templates"
	"github.com/Mirantis/hmc/test/utils"
)

type ProviderType string

const (
	ProviderCAPI    ProviderType = "cluster-api"
	ProviderAWS     ProviderType = "infrastructure-aws"
	ProviderAzure   ProviderType = "infrastructure-azure"
	ProviderVSphere ProviderType = "infrastructure-vsphere"

	providerLabel = "cluster.x-k8s.io/provider"

	Namespace = "default"
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

func setClusterName(templateType templates.Type) {
	var generatedName string

	mcName := os.Getenv(EnvVarManagedClusterName)
	if mcName == "" {
		mcName = "e2e-test-" + uuid.New().String()[:8]
	}

	providerName := strings.Split(string(templateType), "-")[0]

	// Append the provider name to the cluster name to ensure uniqueness between
	// different deployed ManagedClusters.
	generatedName = fmt.Sprintf("%s-%s", mcName, providerName)
	if strings.Contains(string(templateType), "hosted") {
		generatedName = fmt.Sprintf("%s-%s", mcName, "hosted")
	}

	GinkgoT().Setenv(EnvVarManagedClusterName, generatedName)
}

func setTemplate(templateName string) {
	GinkgoT().Setenv(EnvVarManagedClusterTemplate, templateName)
}

// GetUnstructured returns an unstructured ManagedCluster object based on the
// provider and template.
func GetUnstructured(templateType templates.Type, templateName string) *unstructured.Unstructured {
	GinkgoHelper()

	setClusterName(templateType)
	setTemplate(templateName)

	var managedClusterTemplateBytes []byte
	switch templateType {
	case templates.TemplateAWSStandaloneCP:
		managedClusterTemplateBytes = awsStandaloneCPManagedClusterTemplateBytes
	case templates.TemplateAWSHostedCP:
		// Validate environment vars that do not have defaults are populated.
		// We perform this validation here instead of within a Before block
		// since we populate the vars from standalone prior to this step.
		ValidateDeploymentVars([]string{
			EnvVarAWSVPCID,
			EnvVarAWSSubnetID,
			EnvVarAWSSubnetAvailabilityZone,
			EnvVarAWSSecurityGroupID,
		})
		managedClusterTemplateBytes = awsHostedCPManagedClusterTemplateBytes
	case templates.TemplateVSphereStandaloneCP:
		managedClusterTemplateBytes = vsphereStandaloneCPManagedClusterTemplateBytes
	case templates.TemplateVSphereHostedCP:
		managedClusterTemplateBytes = vsphereHostedCPManagedClusterTemplateBytes
	case templates.TemplateAzureHostedCP:
		managedClusterTemplateBytes = azureHostedCPManagedClusterTemplateBytes
	case templates.TemplateAzureStandaloneCP:
		managedClusterTemplateBytes = azureStandaloneCPManagedClusterTemplateBytes
	default:
		Fail(fmt.Sprintf("Unsupported template type: %s", templateType))
	}

	managedClusterConfigBytes, err := envsubst.Bytes(managedClusterTemplateBytes)
	Expect(err).NotTo(HaveOccurred(), "failed to substitute environment variables")

	var managedClusterConfig map[string]any

	err = yaml.Unmarshal(managedClusterConfigBytes, &managedClusterConfig)
	Expect(err).NotTo(HaveOccurred(), "failed to unmarshal deployment config")

	return &unstructured.Unstructured{Object: managedClusterConfig}
}

func ValidateDeploymentVars(v []string) {
	GinkgoHelper()

	for _, envVar := range v {
		Expect(os.Getenv(envVar)).NotTo(BeEmpty(), envVar+" must be set")
	}
}
