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

package utils

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"

	"github.com/google/uuid"
	"gopkg.in/yaml.v3"
)

type ProviderType string

const (
	ProviderAWS ProviderType = "aws"
)

type Template string

const (
	AWSStandaloneCPTemplate Template = "aws-standalone-cp"
	AWSHostedCPTemplate     Template = "aws-hosted-cp"
)

// ConfigureDeploymentConfig modifies the ./config/dev/deployment.yaml for
// use in test and returns the generated cluster name.
func ConfigureDeploymentConfig(provider ProviderType, templateName Template) (string, error) {
	generatedName := uuid.NewString()[:8] + "-e2e-test"

	deploymentConfigBytes, err := os.ReadFile("./config/dev/deployment.yaml")
	if err != nil {
		return "", fmt.Errorf("failed to read deployment config: %w", err)
	}

	var deploymentConfig map[string]interface{}

	err = yaml.Unmarshal(deploymentConfigBytes, &deploymentConfig)
	if err != nil {
		return "", fmt.Errorf("failed to unmarshal deployment config: %w", err)
	}

	switch provider {
	case ProviderAWS:
		// XXX: Maybe we should just use automatic AMI selection here.
		amiID, err := getAWSAMI()
		if err != nil {
			return "", fmt.Errorf("failed to get AWS AMI: %w", err)
		}

		awsRegion := os.Getenv("AWS_REGION")

		// Modify the existing ./config/dev/deployment.yaml file to use the
		// AMI we just found and our AWS_REGION.
		if metadata, ok := deploymentConfig["metadata"].(map[string]interface{}); ok {
			metadata["name"] = generatedName
		} else {
			// Ensure we always have a metadata.name field populated.
			deploymentConfig["metadata"] = map[string]interface{}{"name": generatedName}
		}

		if spec, ok := deploymentConfig["spec"].(map[string]interface{}); ok {
			if config, ok := spec["config"].(map[string]interface{}); ok {
				if awsRegion != "" {
					config["region"] = awsRegion
				}

				if worker, ok := config["worker"].(map[string]interface{}); ok {
					worker["amiID"] = amiID
				}

				if controlPlane, ok := config["controlPlane"].(map[string]interface{}); ok {
					controlPlane["amiID"] = amiID
				}
			}
		}

		deploymentConfigBytes, err = yaml.Marshal(deploymentConfig)
		if err != nil {
			return "", fmt.Errorf("failed to marshal deployment config: %w", err)
		}

		return generatedName, os.WriteFile("./config/dev/deployment.yaml", deploymentConfigBytes, 0644)
	default:
		return "", fmt.Errorf("unsupported provider: %s", provider)
	}
}

// getAWSAMI returns an AWS AMI ID to use for test.
func getAWSAMI() (string, error) {
	// For now we'll just use the latest Kubernetes version for ubuntu 20.04,
	// but we could potentially pin the Kube version and specify that here.
	cmd := exec.Command("./bin/clusterawsadm", "ami", "list", "--os=ubuntu-20.04", "-o", "json")
	output, err := Run(cmd)
	if err != nil {
		return "", fmt.Errorf("failed to list AMIs: %w", err)
	}

	var amiList map[string]interface{}

	if err := json.Unmarshal(output, &amiList); err != nil {
		return "", fmt.Errorf("failed to unmarshal AMI list: %w", err)
	}

	// ami list returns a sorted list of AMIs by kube version, just get the
	// first one.
	for _, item := range amiList["items"].([]interface{}) {
		spec := item.(map[string]interface{})["spec"].(map[string]interface{})
		if imageID, ok := spec["imageID"]; ok {
			ami, ok := imageID.(string)
			if !ok {
				continue
			}

			return ami, nil
		}
	}

	return "", fmt.Errorf("no AMIs found")
}
