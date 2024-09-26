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

// Package aws contains specific helpers for testing a managed cluster
// that uses the AWS infrastructure provider.
package aws

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"

	"github.com/a8m/envsubst"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer/yaml"
	"k8s.io/apimachinery/pkg/types"
	yamlutil "k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/restmapper"

	"github.com/Mirantis/hmc/test/kubeclient"
	"github.com/Mirantis/hmc/test/managedcluster"
)

func CreateCredentialSecret(ctx context.Context, kc *kubeclient.KubeClient) {
	GinkgoHelper()
	serializer := yaml.NewDecodingSerializer(unstructured.UnstructuredJSONScheme)
	yamlFile, err := os.ReadFile("config/dev/aws-credentials.yaml")
	Expect(err).NotTo(HaveOccurred())

	yamlFile, err = envsubst.Bytes(yamlFile)
	Expect(err).NotTo(HaveOccurred())

	c := discovery.NewDiscoveryClientForConfigOrDie(kc.Config)
	groupResources, err := restmapper.GetAPIGroupResources(c)
	Expect(err).NotTo(HaveOccurred())

	yamlReader := yamlutil.NewYAMLReader(bufio.NewReader(bytes.NewReader(yamlFile)))
	for {
		yamlDoc, err := yamlReader.Read()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			Expect(err).NotTo(HaveOccurred(), "failed to read yaml file")
		}

		credentialResource := &unstructured.Unstructured{}
		_, _, err = serializer.Decode(yamlDoc, nil, credentialResource)
		Expect(err).NotTo(HaveOccurred(), "failed to parse credential resource")

		mapper := restmapper.NewDiscoveryRESTMapper(groupResources)
		mapping, err := mapper.RESTMapping(credentialResource.GroupVersionKind().GroupKind())
		Expect(err).NotTo(HaveOccurred(), "failed to get rest mapping")

		dc := kc.GetDynamicClient(schema.GroupVersionResource{
			Group:    credentialResource.GroupVersionKind().Group,
			Version:  credentialResource.GroupVersionKind().Version,
			Resource: mapping.Resource.Resource,
		})

		exists, err := dc.Get(ctx, credentialResource.GetName(), metav1.GetOptions{})
		if !apierrors.IsNotFound(err) {
			Expect(err).NotTo(HaveOccurred(), "failed to get azure credential secret")
		}

		if exists == nil {
			if _, err := dc.Create(ctx, credentialResource, metav1.CreateOptions{}); err != nil {
				Expect(err).NotTo(HaveOccurred(), "failed to create azure credential secret")
			}
		}
	}
}

// PopulateHostedTemplateVars populates the environment variables required for
// the AWS hosted CP template by querying the standalone CP cluster with the
// given kubeclient.
func PopulateHostedTemplateVars(ctx context.Context, kc *kubeclient.KubeClient, clusterName string) {
	GinkgoHelper()

	c := getAWSClusterClient(kc)
	awsCluster, err := c.Get(ctx, clusterName, metav1.GetOptions{})
	Expect(err).NotTo(HaveOccurred(), "failed to get AWS cluster")

	vpcID, found, err := unstructured.NestedString(awsCluster.Object, "spec", "network", "vpc", "id")
	Expect(err).NotTo(HaveOccurred(), "failed to get AWS cluster VPC ID")
	Expect(found).To(BeTrue(), "AWS cluster has no VPC ID")

	subnets, found, err := unstructured.NestedSlice(awsCluster.Object, "spec", "network", "subnets")
	Expect(err).NotTo(HaveOccurred(), "failed to get AWS cluster subnets")
	Expect(found).To(BeTrue(), "AWS cluster has no subnets")

	subnet, ok := subnets[0].(map[string]any)
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

	trueStatus := map[string]any{
		"status": map[string]any{
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
