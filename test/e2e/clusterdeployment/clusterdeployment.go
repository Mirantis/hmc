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

package clusterdeployment

import (
	_ "embed"
	"fmt"
	"os"

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
)

//go:embed resources/aws-standalone-cp.yaml.tpl
var awsStandaloneCPClusterDeploymentTemplateBytes []byte

//go:embed resources/aws-hosted-cp.yaml.tpl
var awsHostedCPClusterDeploymentTemplateBytes []byte

//go:embed resources/azure-standalone-cp.yaml.tpl
var azureStandaloneCPClusterDeploymentTemplateBytes []byte

//go:embed resources/azure-hosted-cp.yaml.tpl
var azureHostedCPClusterDeploymentTemplateBytes []byte

//go:embed resources/vsphere-standalone-cp.yaml.tpl
var vsphereStandaloneCPClusterDeploymentTemplateBytes []byte

//go:embed resources/vsphere-hosted-cp.yaml.tpl
var vsphereHostedCPClusterDeploymentTemplateBytes []byte

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

func GenerateClusterName(postfix string) string {
	mcPrefix := os.Getenv(EnvVarClusterDeploymentPrefix)
	if mcPrefix == "" {
		mcPrefix = "e2e-test-" + uuid.New().String()[:8]
	}

	if postfix != "" {
		return fmt.Sprintf("%s-%s", mcPrefix, postfix)
	}
	return mcPrefix
}

func setClusterName(name string) {
	GinkgoT().Setenv(EnvVarClusterDeploymentName, name)
}

func setTemplate(templateName string) {
	GinkgoT().Setenv(EnvVarClusterDeploymentTemplate, templateName)
}

// GetUnstructured returns an unstructured ClusterDeployment object based on the
// provider and template.
func GetUnstructured(templateType templates.Type, clusterName, template string) *unstructured.Unstructured {
	GinkgoHelper()

	setClusterName(clusterName)
	setTemplate(template)

	var clusterDeploymentTemplateBytes []byte
	switch templateType {
	case templates.TemplateAWSStandaloneCP:
		clusterDeploymentTemplateBytes = awsStandaloneCPClusterDeploymentTemplateBytes
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
		clusterDeploymentTemplateBytes = awsHostedCPClusterDeploymentTemplateBytes
	case templates.TemplateVSphereStandaloneCP:
		clusterDeploymentTemplateBytes = vsphereStandaloneCPClusterDeploymentTemplateBytes
	case templates.TemplateVSphereHostedCP:
		clusterDeploymentTemplateBytes = vsphereHostedCPClusterDeploymentTemplateBytes
	case templates.TemplateAzureHostedCP:
		clusterDeploymentTemplateBytes = azureHostedCPClusterDeploymentTemplateBytes
	case templates.TemplateAzureStandaloneCP:
		clusterDeploymentTemplateBytes = azureStandaloneCPClusterDeploymentTemplateBytes
	default:
		Fail(fmt.Sprintf("Unsupported template type: %s", templateType))
	}

	clusterDeploymentConfigBytes, err := envsubst.Bytes(clusterDeploymentTemplateBytes)
	Expect(err).NotTo(HaveOccurred(), "failed to substitute environment variables")

	var clusterDeploymentConfig map[string]any

	err = yaml.Unmarshal(clusterDeploymentConfigBytes, &clusterDeploymentConfig)
	Expect(err).NotTo(HaveOccurred(), "failed to unmarshal deployment config")

	return &unstructured.Unstructured{Object: clusterDeploymentConfig}
}

func ValidateDeploymentVars(v []string) {
	GinkgoHelper()

	for _, envVar := range v {
		Expect(os.Getenv(envVar)).NotTo(BeEmpty(), envVar+" must be set")
	}
}
