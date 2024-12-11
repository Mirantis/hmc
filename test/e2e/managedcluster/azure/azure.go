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
	"context"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/utils/ptr"

	hmc "github.com/Mirantis/hmc/api/v1alpha1"
	"github.com/Mirantis/hmc/test/e2e/kubeclient"
)

func getAzureInfo(ctx context.Context, name string, kc *kubeclient.KubeClient) map[string]any {
	GinkgoHelper()
	resourceID := schema.GroupVersionResource{
		Group:    "infrastructure.cluster.x-k8s.io",
		Version:  "v1beta1",
		Resource: "azureclusters",
	}

	dc := kc.GetDynamicClient(resourceID, true)
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

	for _, subnet := range subnets {
		sMap, ok := subnet.(map[string]any)
		Expect(ok).To(BeTrue())

		routeTable, exists, err := unstructured.NestedMap(sMap, "routeTable")
		Expect(err).NotTo(HaveOccurred())
		Expect(exists).To(BeTrue())
		routeTableName := routeTable["name"]

		subnetName := sMap["name"]

		securityGroup, found, err := unstructured.NestedMap(sMap, "securityGroup")
		Expect(err).NotTo(HaveOccurred())
		Expect(found).To(BeTrue())
		securityGroupName := securityGroup["name"]

		role, exists, err := unstructured.NestedString(sMap, "role")
		Expect(err).NotTo(HaveOccurred())
		Expect(exists).To(BeTrue())

		if role == "control-plane" {
			GinkgoT().Setenv("AZURE_CP_SUBNET", fmt.Sprintf("%s", subnetName))
			GinkgoT().Setenv("AZURE_CP_SECURITY_GROUP", fmt.Sprintf("%s", securityGroupName))
			GinkgoT().Setenv("AZURE_CP_ROUTE_TABLE", fmt.Sprintf("%s", routeTableName))
		} else {
			GinkgoT().Setenv("AZURE_NODE_SUBNET", fmt.Sprintf("%s", subnetName))
			GinkgoT().Setenv("AZURE_NODE_SECURITY_GROUP", fmt.Sprintf("%s", securityGroupName))
			GinkgoT().Setenv("AZURE_NODE_ROUTE_TABLE", fmt.Sprintf("%s", routeTableName))
		}
	}
}

// CreateDefaultStorageClass configures the default storage class for Azure
// based on the azure-disk CSI driver that we deploy as part of our templates.
func CreateDefaultStorageClass(kc *kubeclient.KubeClient) {
	GinkgoHelper()

	ctx := context.Background()

	azureDiskSC := &storagev1.StorageClass{
		ObjectMeta: metav1.ObjectMeta{
			Name: "azure-disk",
			Annotations: map[string]string{
				"storageclass.kubernetes.io/is-default-class": "true",
			},
		},
		Provisioner:          "disk.csi.azure.com",
		ReclaimPolicy:        ptr.To(corev1.PersistentVolumeReclaimDelete),
		VolumeBindingMode:    ptr.To(storagev1.VolumeBindingWaitForFirstConsumer),
		AllowVolumeExpansion: ptr.To(true),
		Parameters: map[string]string{
			"skuName": "StandardSSD_LRS",
		},
	}

	sc, err := kc.Client.StorageV1().StorageClasses().Get(ctx, "azure-disk", metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			_, err := kc.Client.StorageV1().StorageClasses().Create(ctx, azureDiskSC, metav1.CreateOptions{})
			Expect(err).NotTo(HaveOccurred())
		}
	} else {
		azureDiskSC.SetResourceVersion(sc.GetResourceVersion())
		_, err = kc.Client.StorageV1().StorageClasses().Update(ctx, azureDiskSC, metav1.UpdateOptions{})
		Expect(err).NotTo(HaveOccurred())
	}
}
