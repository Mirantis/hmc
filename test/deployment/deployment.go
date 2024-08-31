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

package deployment

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
)

type ProviderType string

const (
	ProviderCAPI  ProviderType = "cluster-api"
	ProviderAWS   ProviderType = "infrastructure-aws"
	ProviderAzure ProviderType = "infrastructure-azure"

	providerLabel = "cluster.x-k8s.io/provider"
)

type Template string

const (
	TemplateAWSStandaloneCP Template = "aws-standalone-cp"
	TemplateAWSHostedCP     Template = "aws-hosted-cp"
)

//go:embed resources/aws-standalone-cp.yaml.tpl
var awsStandaloneCPDeploymentTemplateBytes []byte

//go:embed resources/aws-hosted-cp.yaml.tpl
var awsHostedCPDeploymentTemplateBytes []byte

func GetProviderLabel(provider ProviderType) string {
	return fmt.Sprintf("%s=%s", providerLabel, provider)
}

// GetUnstructuredDeployment returns an unstructured deployment object based on
// the provider and template.
func GetUnstructuredDeployment(provider ProviderType, templateName Template) *unstructured.Unstructured {
	GinkgoHelper()

	generatedName := uuid.New().String()[:8] + "-e2e-test"
	_, _ = fmt.Fprintf(GinkgoWriter, "Generated cluster name: %q\n", generatedName)

	switch provider {
	case ProviderAWS:
		Expect(os.Setenv("DEPLOYMENT_NAME", generatedName)).NotTo(HaveOccurred())

		var deploymentTemplateBytes []byte
		switch templateName {
		case TemplateAWSStandaloneCP:
			deploymentTemplateBytes = awsStandaloneCPDeploymentTemplateBytes
		case TemplateAWSHostedCP:
			deploymentTemplateBytes = awsHostedCPDeploymentTemplateBytes
		default:
			Fail(fmt.Sprintf("unsupported AWS template: %s", templateName))
		}

		deploymentConfigBytes, err := envsubst.Bytes(deploymentTemplateBytes)
		Expect(err).NotTo(HaveOccurred(), "failed to substitute environment variables")

		var deploymentConfig map[string]interface{}

		err = yaml.Unmarshal(deploymentConfigBytes, &deploymentConfig)
		Expect(err).NotTo(HaveOccurred(), "failed to unmarshal deployment config")

		return &unstructured.Unstructured{Object: deploymentConfig}
	default:
		Fail(fmt.Sprintf("unsupported provider: %s", provider))
	}

	return nil
}
