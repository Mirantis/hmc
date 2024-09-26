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

package azure

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/a8m/envsubst"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer/yaml"
	yamlutil "k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/restmapper"

	hmc "github.com/Mirantis/hmc/api/v1alpha1"
	"github.com/Mirantis/hmc/test/kubeclient"
)

func getAzureInfo(ctx context.Context, name string, kc *kubeclient.KubeClient) map[string]any {
	GinkgoHelper()
	resourceID := schema.GroupVersionResource{
		Group:    "infrastructure.cluster.x-k8s.io",
		Version:  "v1beta1",
		Resource: "azureclusters",
	}

	dc := kc.GetDynamicClient(resourceID)
	list, err := dc.List(ctx, metav1.ListOptions{
		LabelSelector: labels.SelectorFromSet(map[string]string{hmc.FluxHelmChartNameKey: name}).String(),
	})

	Expect(err).NotTo(HaveOccurred())
	Expect(len(list.Items)).NotTo(BeEquivalentTo(0))

	spec, found, err := unstructured.NestedMap(list.Items[0].Object, "spec")
	Expect(err).NotTo(HaveOccurred())
	Expect(found).To(BeTrue())
	return spec
}

func SetAzureEnvironmentVariables(clusterName string, kc *kubeclient.KubeClient) {
	GinkgoHelper()
	spec := getAzureInfo(context.Background(), clusterName, kc)

	networkSpec, found, err := unstructured.NestedMap(spec, "networkSpec")
	Expect(err).NotTo(HaveOccurred())
	Expect(found).To(BeTrue())

	vnet, found, err := unstructured.NestedMap(networkSpec, "vnet")
	Expect(err).NotTo(HaveOccurred())
	Expect(found).To(BeTrue())
	vnetName, ok := vnet["name"].(string)
	Expect(ok).To(BeTrue())
	GinkgoT().Setenv("AZURE_VM_NET_NAME", vnetName)

	subnets, found, err := unstructured.NestedSlice(networkSpec, "subnets")
	Expect(err).NotTo(HaveOccurred())
	Expect(found).To(BeTrue())

	resourceGroup := spec["resourceGroup"]
	GinkgoT().Setenv("AZURE_RESOURCE_GROUP", fmt.Sprintf("%s", resourceGroup))
	subnetMap, ok := subnets[0].(map[string]any)
	Expect(ok).To(BeTrue())
	subnetName := subnetMap["name"]
	GinkgoT().Setenv("AZURE_NODE_SUBNET", fmt.Sprintf("%s", subnetName))

	securityGroup, found, err := unstructured.NestedMap(subnetMap, "securityGroup")
	Expect(err).NotTo(HaveOccurred())
	Expect(found).To(BeTrue())
	securityGroupName := securityGroup["name"]
	GinkgoT().Setenv("AZURE_SECURITY_GROUP", fmt.Sprintf("%s", securityGroupName))

	routeTable, found, err := unstructured.NestedMap(subnetMap, "routeTable")
	Expect(err).NotTo(HaveOccurred())
	Expect(found).To(BeTrue())
	routeTableName := routeTable["name"]
	GinkgoT().Setenv("AZURE_ROUTE_TABLE", fmt.Sprintf("%s", routeTableName))
}

func CreateCredentialSecret(ctx context.Context, kc *kubeclient.KubeClient) {
	GinkgoHelper()
	serializer := yaml.NewDecodingSerializer(unstructured.UnstructuredJSONScheme)
	yamlFile, err := os.ReadFile("config/dev/azure-credentials.yaml")
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
			if _, createErr := dc.Create(ctx, credentialResource, metav1.CreateOptions{}); err != nil {
				Expect(createErr).NotTo(HaveOccurred(), "failed to create azure credential secret")
			}
		}
	}
}
