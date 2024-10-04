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

package vsphere

import (
	"context"
	"fmt"
	"os"

	"github.com/Mirantis/hmc/test/kubeclient"
	"github.com/Mirantis/hmc/test/managedcluster"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

func CreateSecret(kc *kubeclient.KubeClient, secretName string) error {
	ctx := context.Background()
	_, err := kc.Client.CoreV1().Secrets(kc.Namespace).Get(ctx, secretName, metav1.GetOptions{})

	if !apierrors.IsNotFound(err) {
		return nil
	}
	username := os.Getenv("VSPHERE_USER")
	password := os.Getenv("VSPHERE_PASSWORD")

	_, err = kc.Client.CoreV1().Secrets(kc.Namespace).Create(ctx, &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: secretName,
		},
		StringData: map[string]string{
			"username": username,
			"password": password,
		},
		Type: corev1.SecretTypeOpaque,
	}, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create vSphere credentials secret: %w", err)
	}

	return nil
}

func CreateClusterIdentity(kc *kubeclient.KubeClient, secretName string, identityName string) error {
	ctx := context.Background()
	client, err := dynamic.NewForConfig(kc.Config)
	if err != nil {
		return fmt.Errorf("failed to create dynamic client: %w", err)
	}

	gvr := schema.GroupVersionResource{
		Group:    "infrastructure.cluster.x-k8s.io",
		Version:  "v1beta1",
		Resource: "vsphereclusteridentities",
	}

	clusterIdentity := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "infrastructure.cluster.x-k8s.io/v1beta1",
			"kind":       "VSphereClusterIdentity",
			"metadata": map[string]any{
				"name": identityName,
			},
			"spec": map[string]any{
				"secretName": secretName,
				"allowedNamespaces": map[string]any{
					"selector": map[string]any{
						"matchLabels": map[string]any{},
					},
				},
			},
		},
	}

	result, err := client.Resource(gvr).Create(ctx, clusterIdentity, metav1.CreateOptions{})
	if err != nil {
		fmt.Printf("%+v", result)
		return fmt.Errorf("failed to create vsphereclusteridentity: %w", err)
	}

	return nil
}

func CheckEnv() {
	managedcluster.ValidateDeploymentVars([]string{
		"VSPHERE_USER",
		"VSPHERE_PASSWORD",
		"VSPHERE_SERVER",
		"VSPHERE_THUMBPRINT",
		"VSPHERE_DATACENTER",
		"VSPHERE_DATASTORE",
		"VSPHERE_RESOURCEPOOL",
		"VSPHERE_FOLDER",
		"VSPHERE_CONTROL_PLANE_ENDPOINT",
		"VSPHERE_VM_TEMPLATE",
		"VSPHERE_NETWORK",
		"VSPHERE_SSH_KEY",
	})
}
