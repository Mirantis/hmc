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

// This file contains specific helpers for testing a managed cluster
// that uses the AWS infrastructure provider.
package aws

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"

	corev1 "k8s.io/api/core/v1"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"

	"github.com/Mirantis/hmc/test/kubeclient"
	"github.com/Mirantis/hmc/test/managedcluster"
	"github.com/Mirantis/hmc/test/utils"
)

// CreateCredentialSecret uses clusterawsadm to encode existing AWS
// credentials and create a secret in the given namespace if one does not
// already exist.
func CreateCredentialSecret(ctx context.Context, kc *kubeclient.KubeClient) {
	GinkgoHelper()

	_, err := kc.Client.CoreV1().Secrets(kc.Namespace).
		Get(ctx, managedcluster.AWSCredentialsSecretName, metav1.GetOptions{})
	if !apierrors.IsNotFound(err) {
		Expect(err).NotTo(HaveOccurred(), "failed to get AWS credentials secret")
		return
	}

	cmd := exec.Command("./bin/clusterawsadm", "bootstrap", "credentials", "encode-as-profile")
	output, err := utils.Run(cmd)
	Expect(err).NotTo(HaveOccurred(), "failed to encode AWS credentials with clusterawsadm")

	_, err = kc.Client.CoreV1().Secrets(kc.Namespace).Create(ctx, &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: managedcluster.AWSCredentialsSecretName,
		},
		Data: map[string][]byte{
			"AWS_B64ENCODED_CREDENTIALS": output,
		},
		Type: corev1.SecretTypeOpaque,
	}, metav1.CreateOptions{})
	Expect(err).NotTo(HaveOccurred(), "failed to create AWS credentials secret")
}

// PopulateHostedTemplateVars populates the environment variables required for
// the AWS hosted CP template by querying the standalone CP cluster with the
// given kubeclient.
func PopulateHostedTemplateVars(ctx context.Context, kc *kubeclient.KubeClient) {
	GinkgoHelper()

	c := getAWSClusterClient(kc)
	awsCluster, err := c.Get(ctx, os.Getenv(managedcluster.EnvVarManagedClusterName), metav1.GetOptions{})
	Expect(err).NotTo(HaveOccurred(), "failed to get AWS cluster")

	vpcID, found, err := unstructured.NestedString(awsCluster.Object, "spec", "network", "vpc", "id")
	Expect(err).NotTo(HaveOccurred(), "failed to get AWS cluster VPC ID")
	Expect(found).To(BeTrue(), "AWS cluster has no VPC ID")

	subnets, found, err := unstructured.NestedSlice(awsCluster.Object, "spec", "network", "subnets")
	Expect(err).NotTo(HaveOccurred(), "failed to get AWS cluster subnets")
	Expect(found).To(BeTrue(), "AWS cluster has no subnets")

	subnet, ok := subnets[0].(map[string]interface{})
	Expect(ok).To(BeTrue(), "failed to cast subnet to map")

	subnetID, ok := subnet["resourceID"].(string)
	Expect(ok).To(BeTrue(), "failed to cast subnet ID to string")

	subnetAZ, ok := subnet["availabilityZone"].(string)
	Expect(ok).To(BeTrue(), "failed to cast subnet availability zone to string")

	securityGroupID, found, err := unstructured.NestedString(
		awsCluster.Object, "status", "networkStatus", "securityGroups", "node", "id")
	Expect(err).NotTo(HaveOccurred(), "failed to get AWS cluster security group ID")
	Expect(found).To(BeTrue(), "AWS cluster has no security group ID")

	GinkgoT().Setenv(managedcluster.EnvVarAWSVPCID, vpcID)
	GinkgoT().Setenv(managedcluster.EnvVarAWSSubnetID, subnetID)
	GinkgoT().Setenv(managedcluster.EnvVarAWSSubnetAvailabilityZone, subnetAZ)
	GinkgoT().Setenv(managedcluster.EnvVarAWSSecurityGroupID, securityGroupID)
}

func PatchAWSClusterReady(ctx context.Context, kc *kubeclient.KubeClient, clusterName string) error {
	GinkgoHelper()

	c := getAWSClusterClient(kc)

	trueStatus := map[string]interface{}{
		"status": map[string]interface{}{
			"ready": true,
		},
	}

	patchBytes, err := json.Marshal(trueStatus)
	Expect(err).NotTo(HaveOccurred(), "failed to marshal patch bytes")

	_, err = c.Patch(ctx, clusterName, types.MergePatchType,
		patchBytes, metav1.PatchOptions{}, "status")
	if err != nil {
		return err
	}

	return nil
}

func getAWSClusterClient(kc *kubeclient.KubeClient) dynamic.ResourceInterface {
	return kc.GetDynamicClient(schema.GroupVersionResource{
		Group:    "infrastructure.cluster.x-k8s.io",
		Version:  "v1beta2",
		Resource: "awsclusters",
	})
}
