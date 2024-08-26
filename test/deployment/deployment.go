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
	"encoding/json"
	"fmt"
	"os"
	"os/exec"

	"github.com/Mirantis/hmc/test/utils"
	"github.com/a8m/envsubst"
	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"gopkg.in/yaml.v3"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type ProviderType string

const (
	ProviderAWS ProviderType = "aws"
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

// GetUnstructuredDeployment returns an unstructured deployment object based on
// the provider and template.
func GetUnstructuredDeployment(provider ProviderType, templateName Template) *unstructured.Unstructured {
	GinkgoHelper()

	generatedName := uuid.New().String()[:8] + "-e2e-test"
	_, _ = fmt.Fprintf(GinkgoWriter, "Generated cluster name: %q\n", generatedName)

	switch provider {
	case ProviderAWS:
		// XXX: Maybe we should just use automatic AMI selection here.
		amiID := getAWSAMI()
		Expect(os.Setenv("AWS_AMI_ID", amiID)).NotTo(HaveOccurred())
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

// getAWSAMI returns an AWS AMI ID to use for test.
func getAWSAMI() string {
	GinkgoHelper()

	// For now we'll just use the latest Kubernetes version for ubuntu 20.04,
	// but we could potentially pin the Kube version and specify that here.
	cmd := exec.Command("./bin/clusterawsadm", "ami", "list", "--os=ubuntu-20.04", "-o", "json")
	output, err := utils.Run(cmd)
	Expect(err).NotTo(HaveOccurred(), "failed to list AMIs")

	var amiList map[string]interface{}

	err = json.Unmarshal(output, &amiList)
	Expect(err).NotTo(HaveOccurred(), "failed to unmarshal AMI list")

	// ami list returns a sorted list of AMIs by kube version, just get the
	// first one.
	for _, item := range amiList["items"].([]interface{}) {
		spec := item.(map[string]interface{})["spec"].(map[string]interface{})
		if imageID, ok := spec["imageID"]; ok {
			ami, ok := imageID.(string)
			if !ok {
				continue
			}

			return ami
		}
	}

	Fail("no AMIs found")

	return ""
}
