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

type Template string

const (
	TemplateAWSStandaloneCP     Template = "aws-standalone-cp"
	TemplateAWSHostedCP         Template = "aws-hosted-cp"
	TemplateAzureHostedCP       Template = "azure-hosted-cp"
	TemplateAzureStandaloneCP   Template = "azure-standalone-cp"
	TemplateVSphereStandaloneCP Template = "vsphere-standalone-cp"
	TemplateVSphereHostedCP     Template = "vsphere-hosted-cp"
	TemplateEKSCP               Template = "aws-eks-cp"
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

//go:embed resources/aws-eks-cp.yaml.tpl
var eksCPManagedClusterTemplateBytes []byte

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
			EnvVarAWSSubnetID,
			EnvVarAWSSubnetAvailabilityZone,
			EnvVarAWSSecurityGroupID,
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
	case TemplateEKSCP:
		managedClusterTemplateBytes = eksCPManagedClusterTemplateBytes
	default:
		Fail(fmt.Sprintf("Unsupported template: %s", templateName))
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
