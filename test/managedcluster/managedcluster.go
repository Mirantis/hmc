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

	"github.com/Mirantis/hmc/internal/utils"
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

func GetProviderLabel(provider ProviderType) string {
	return fmt.Sprintf("%s=%s", providerLabel, provider)
}

// GetUnstructured returns an unstructured ManagedCluster object based on the
// provider and template.
func GetUnstructured(templateName Template) *unstructured.Unstructured {
	GinkgoHelper()

	generatedName := os.Getenv(EnvVarManagedClusterName)
	if generatedName == "" {
		generatedName = "e2e-test-" + uuid.New().String()[:8]
		_, _ = fmt.Fprintf(GinkgoWriter, "Generated cluster name: %q\n", generatedName)
		GinkgoT().Setenv(EnvVarManagedClusterName, generatedName)
	} else {
		_, _ = fmt.Fprintf(GinkgoWriter, "Using configured cluster name: %q\n", generatedName)
	}

	var hostedName string
	if strings.Contains(string(templateName), "-hosted") {
		hostedName = generatedName + "-hosted"
		GinkgoT().Setenv(EnvVarHostedManagedClusterName, hostedName)
		_, _ = fmt.Fprintf(GinkgoWriter, "Creating hosted ManagedCluster with name: %q\n", hostedName)
	}

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
		//case ProviderAzure:
		//	Expect(os.Setenv("NAMESPACE", namespace)).NotTo(HaveOccurred())

	//var managedClusterTemplateBytes []byte
	//switch templateName {
	case TemplateAzureHostedCP:
		managedClusterTemplateBytes = azureHostedCPManagedClusterTemplateBytes
	case TemplateAzureStandaloneCP:
		managedClusterTemplateBytes = azureStandaloneCPManagedClusterTemplateBytes
		//default:
		//	Fail(fmt.Sprintf("unsupported Azure template: %s", templateName))
		//}

		//managedClusterConfigBytes, err := envsubst.Bytes(managedClusterTemplateBytes)
		//Expect(err).NotTo(HaveOccurred(), "failed to substitute environment variables")

	default:
		Fail(fmt.Sprintf("unsupported AWS template: %s", templateName))
	}

	Expect(os.Setenv("NAMESPACE", utils.DefaultSystemNamespace)).NotTo(HaveOccurred())
	managedClusterConfigBytes, err := envsubst.Bytes(managedClusterTemplateBytes)
	Expect(err).NotTo(HaveOccurred(), "failed to substitute environment variables")

	var managedClusterConfig map[string]interface{}

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
