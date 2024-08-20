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
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	. "github.com/onsi/ginkgo/v2" //nolint:golint,revive
	"gopkg.in/yaml.v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func warnError(err error) {
	_, _ = fmt.Fprintf(GinkgoWriter, "warning: %v\n", err)
}

// Run executes the provided command within this context
func Run(cmd *exec.Cmd) ([]byte, error) {
	dir, _ := GetProjectDir()
	cmd.Dir = dir

	if err := os.Chdir(cmd.Dir); err != nil {
		_, _ = fmt.Fprintf(GinkgoWriter, "chdir dir: %s\n", err)
	}

	cmd.Env = append(os.Environ(), "GO111MODULE=on")
	command := strings.Join(cmd.Args, " ")
	_, _ = fmt.Fprintf(GinkgoWriter, "running: %s\n", command)

	output, err := cmd.Output()
	if err != nil {
		var exitError *exec.ExitError

		if errors.As(err, &exitError) {
			return output, fmt.Errorf("%s failed with error: (%v) %s", command, err, string(output))
		}
	}

	return output, nil
}

// LoadImageToKindCluster loads a local docker image to the kind cluster
func LoadImageToKindClusterWithName(name string) error {
	cluster := "kind"
	if v, ok := os.LookupEnv("KIND_CLUSTER_NAME"); ok {
		cluster = v
	}
	kindOptions := []string{"load", "docker-image", name, "--name", cluster}

	kindBinary := "kind"

	if kindVersion, ok := os.LookupEnv("KIND_VERSION"); ok {
		kindBinary = fmt.Sprintf("./bin/kind-%s", kindVersion)
	}

	cmd := exec.Command(kindBinary, kindOptions...)
	_, err := Run(cmd)
	return err
}

// GetNonEmptyLines converts given command output string into individual objects
// according to line breakers, and ignores the empty elements in it.
func GetNonEmptyLines(output string) []string {
	var res []string
	elements := strings.Split(output, "\n")
	for _, element := range elements {
		if element != "" {
			res = append(res, element)
		}
	}

	return res
}

// GetProjectDir will return the directory where the project is
func GetProjectDir() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return wd, err
	}
	wd = strings.Replace(wd, "/test/e2e", "", -1)
	return wd, nil
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

// ConfigureDeploymentConfig modifies the ./config/dev/deployment.yaml for
// use in test.  For now we modify only the AWS_REGION and AWSAMI's but in the
// future this may mean more complex configuration.
func ConfigureDeploymentConfig() error {
	amiID, err := getAWSAMI()
	if err != nil {
		return fmt.Errorf("failed to get AWS AMI: %w", err)
	}

	deploymentConfigBytes, err := os.ReadFile("./config/dev/deployment.yaml")
	if err != nil {
		return fmt.Errorf("failed to read deployment config: %w", err)
	}

	var deploymentConfig map[string]interface{}

	err = yaml.Unmarshal(deploymentConfigBytes, &deploymentConfig)
	if err != nil {
		return fmt.Errorf("failed to unmarshal deployment config: %w", err)
	}

	awsRegion := os.Getenv("AWS_REGION")

	// Modify the existing ./config/dev/deployment.yaml file to use the
	// AMI we just found and our AWS_REGION.
	if spec, ok := deploymentConfig["spec"].(map[interface{}]interface{}); ok {
		if config, ok := spec["config"].(map[interface{}]interface{}); ok {
			if awsRegion != "" {
				config["region"] = awsRegion
			}

			if worker, ok := config["worker"].(map[interface{}]interface{}); ok {
				worker["amiID"] = amiID
			}

			if controlPlane, ok := config["controlPlane"].(map[interface{}]interface{}); ok {
				controlPlane["amiID"] = amiID
			}
		}
	}

	deploymentConfigBytes, err = yaml.Marshal(deploymentConfig)
	if err != nil {
		return fmt.Errorf("failed to marshal deployment config: %w", err)
	}

	return os.WriteFile("./config/dev/deployment.yaml", deploymentConfigBytes, 0644)
}

// ValidateConditionsTrue iterates over the conditions of the given
// unstructured object and returns an error if any of the conditions are not
// true.  Conditions are expected to be of type metav1.Condition.
func ValidateConditionsTrue(unstrObj *unstructured.Unstructured) error {
	objKind, objName := ObjKindName(unstrObj)

	// Iterate the status conditions and ensure each condition reports a "Ready"
	// status.
	conditions, found, err := unstructured.NestedSlice(unstrObj.Object, "status", "conditions")
	if !found {
		return fmt.Errorf("no status conditions found for %s: %s", objKind, objName)
	}
	if err != nil {
		return fmt.Errorf("failed to get status conditions for %s: %s: %w", objKind, objName, err)
	}

	for _, condition := range conditions {
		condition, ok := condition.(metav1.Condition)
		if !ok {
			return fmt.Errorf("expected %s: %s condition to be type metav1.Condition, got: %T",
				objKind, objName, condition)
		}

		if condition.Status == metav1.ConditionTrue {
			continue
		}

		return fmt.Errorf("%s %s condition %s is not ready: %s", objKind, objName, condition.Type, condition.Message)
	}

	return nil
}

// ValidateObjectNamePrefix checks if the given object name has the given prefix.
func ValidateObjectNamePrefix(unstrObj *unstructured.Unstructured, clusterName string) error {
	objKind, objName := ObjKindName(unstrObj)

	// Verify the machines are prefixed with the cluster name and fail
	// the test if they are not.
	if !strings.HasPrefix(objName, clusterName) {
		return fmt.Errorf("object %s %s does not have cluster name prefix: %s", objKind, objName, clusterName)
	}

	return nil
}

func ObjKindName(unstrObj *unstructured.Unstructured) (string, string) {
	return unstrObj.GetKind(), unstrObj.GetName()
}
