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

package kubeclient

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apiextensionsclientset "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	crclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/Mirantis/hmc/internal/utils/status"
)

type KubeClient struct {
	Client         kubernetes.Interface
	ExtendedClient apiextensionsclientset.Interface
	Config         *rest.Config

	Namespace string
}

// NewFromLocal creates a new instance of KubeClient from a given namespace
// using the locally found kubeconfig.
func NewFromLocal(namespace string) *KubeClient {
	GinkgoHelper()
	return newKubeClient(getLocalKubeConfig(), namespace)
}

// NewFromCluster creates a new KubeClient using the kubeconfig stored in the
// secret affiliated with the given clusterName.  Since it relies on fetching
// the kubeconfig from secret it needs an existing kubeclient.
func (kc *KubeClient) NewFromCluster(ctx context.Context, namespace, clusterName string) *KubeClient {
	GinkgoHelper()
	return newKubeClient(kc.getKubeconfigSecretData(ctx, clusterName), namespace)
}

// WriteKubeconfig writes the kubeconfig for the given clusterName to the
// test/e2e directory returning the path to the file and a function to delete
// it later.
func (kc *KubeClient) WriteKubeconfig(ctx context.Context, clusterName string) (string, func() error) {
	GinkgoHelper()

	secretData := kc.getKubeconfigSecretData(ctx, clusterName)

	dir, err := os.Getwd()
	Expect(err).NotTo(HaveOccurred())

	path := filepath.Join(dir, clusterName+"-kubeconfig")

	Expect(
		os.WriteFile(path, secretData, 0o644)).
		To(Succeed())

	deleteFunc := func() error {
		if err = os.Remove(filepath.Join(dir, path)); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return nil
			}
			return err
		}
		return nil
	}

	return path, deleteFunc
}

func (kc *KubeClient) getKubeconfigSecretData(ctx context.Context, clusterName string) []byte {
	GinkgoHelper()

	secret, err := kc.Client.CoreV1().Secrets(kc.Namespace).Get(ctx, clusterName+"-kubeconfig", metav1.GetOptions{})
	Expect(err).NotTo(HaveOccurred(), "failed to get cluster: %q kubeconfig secret", clusterName)

	secretData, ok := secret.Data["value"]
	Expect(ok).To(BeTrue(), "kubeconfig secret %q has no 'value' key", clusterName)

	return secretData
}

// getLocalKubeConfig returns the kubeconfig file content.
func getLocalKubeConfig() []byte {
	GinkgoHelper()

	// Use the KUBECONFIG environment variable if it is set, otherwise use the
	// default path.
	kubeConfig, ok := os.LookupEnv("KUBECONFIG")
	if !ok {
		homeDir, err := os.UserHomeDir()
		Expect(err).NotTo(HaveOccurred(), "failed to get user home directory")

		kubeConfig = filepath.Join(homeDir, ".kube", "config")
	}

	configBytes, err := os.ReadFile(kubeConfig)
	Expect(err).NotTo(HaveOccurred(), "failed to read %q", kubeConfig)

	return configBytes
}

// newKubeClient creates a new instance of KubeClient from a given namespace using
// the local kubeconfig.
func newKubeClient(configBytes []byte, namespace string) *KubeClient {
	GinkgoHelper()

	config, err := clientcmd.RESTConfigFromKubeConfig(configBytes)
	Expect(err).NotTo(HaveOccurred(), "failed to parse kubeconfig")

	clientSet, err := kubernetes.NewForConfig(config)
	Expect(err).NotTo(HaveOccurred(), "failed to initialize kubernetes client")

	extendedClientSet, err := apiextensionsclientset.NewForConfig(config)
	Expect(err).NotTo(HaveOccurred(), "failed to initialize apiextensions clientset")

	return &KubeClient{
		Namespace:      namespace,
		Client:         clientSet,
		ExtendedClient: extendedClientSet,
		Config:         config,
	}
}

// GetDynamicClient returns a dynamic client for the given GroupVersionResource.
func (kc *KubeClient) GetDynamicClient(gvr schema.GroupVersionResource, namespaced bool) dynamic.ResourceInterface { //nolint:revive
	GinkgoHelper()

	client, err := dynamic.NewForConfig(kc.Config)
	Expect(err).NotTo(HaveOccurred(), "failed to create dynamic client for resource: %s", gvr.String())

	if !namespaced {
		return client.Resource(gvr)
	}

	return client.Resource(gvr).Namespace(kc.Namespace)
}

func (kc *KubeClient) CreateOrUpdateUnstructuredObject(gvr schema.GroupVersionResource, obj *unstructured.Unstructured, namespaced bool) {
	GinkgoHelper()

	client := kc.GetDynamicClient(gvr, namespaced)

	kind, name := status.ObjKindName(obj)

	resp, err := client.Get(context.Background(), name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		_, err = client.Create(context.Background(), obj, metav1.CreateOptions{})
		Expect(err).NotTo(HaveOccurred(), "failed to create %s: %s", kind, name)
	} else {
		Expect(err).NotTo(HaveOccurred(), "failed to get existing %s: %s", kind, name)

		obj.SetResourceVersion(resp.GetResourceVersion())
		_, err = client.Update(context.Background(), obj, metav1.UpdateOptions{})
		Expect(err).NotTo(HaveOccurred(), "failed to update existing %s: %s", kind, name)
	}
}

// CreateClusterDeployment creates a clusterdeployment.hmc.mirantis.com in the given
// namespace and returns a DeleteFunc to clean up the deployment.
// The DeleteFunc is a no-op if the deployment has already been deleted.
func (kc *KubeClient) CreateClusterDeployment(
	ctx context.Context, clusterDeployment *unstructured.Unstructured,
) func() error {
	GinkgoHelper()

	kind := clusterDeployment.GetKind()
	Expect(kind).To(Equal("ClusterDeployment"))

	client := kc.GetDynamicClient(schema.GroupVersionResource{
		Group:    "hmc.mirantis.com",
		Version:  "v1alpha1",
		Resource: "clusterdeployments",
	}, true)

	_, err := client.Create(ctx, clusterDeployment, metav1.CreateOptions{})
	if !apierrors.IsAlreadyExists(err) {
		Expect(err).NotTo(HaveOccurred(), "failed to create %s", kind)
	}

	return func() error {
		name := clusterDeployment.GetName()
		if err := client.Delete(ctx, name, metav1.DeleteOptions{}); crclient.IgnoreNotFound(err) != nil {
			return err
		}
		Eventually(func() bool {
			_, err := client.Get(ctx, name, metav1.GetOptions{})
			return apierrors.IsNotFound(err)
		}, 30*time.Minute, 1*time.Minute).Should(BeTrue())
		return nil
	}
}

// GetCluster returns a Cluster resource by name.
func (kc *KubeClient) GetCluster(ctx context.Context, clusterName string) (*unstructured.Unstructured, error) {
	gvr := schema.GroupVersionResource{
		Group:    "cluster.x-k8s.io",
		Version:  "v1beta1",
		Resource: "clusters",
	}

	client := kc.GetDynamicClient(gvr, true)

	cluster, err := client.Get(ctx, clusterName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get %s %s: %w", gvr.Resource, clusterName, err)
	}

	return cluster, nil
}

// listResource returns a list of resources for the given GroupVersionResource
// affiliated with the given clusterName.
func (kc *KubeClient) listResource(
	ctx context.Context, gvr schema.GroupVersionResource, clusterName string,
) ([]unstructured.Unstructured, error) {
	client := kc.GetDynamicClient(gvr, true)

	resources, err := client.List(ctx, metav1.ListOptions{
		LabelSelector: "cluster.x-k8s.io/cluster-name=" + clusterName,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list %s", gvr.Resource)
	}

	return resources.Items, nil
}

// ListMachines returns a list of Machine resources for the given cluster.
func (kc *KubeClient) ListMachines(ctx context.Context, clusterName string) ([]unstructured.Unstructured, error) {
	GinkgoHelper()

	return kc.listResource(ctx, schema.GroupVersionResource{
		Group:    "cluster.x-k8s.io",
		Version:  "v1beta1",
		Resource: "machines",
	}, clusterName)
}

// ListMachineDeployments returns a list of MachineDeployment resources for the
// given cluster.
func (kc *KubeClient) ListMachineDeployments(
	ctx context.Context, clusterName string,
) ([]unstructured.Unstructured, error) {
	GinkgoHelper()

	return kc.listResource(ctx, schema.GroupVersionResource{
		Group:    "cluster.x-k8s.io",
		Version:  "v1beta1",
		Resource: "machinedeployments",
	}, clusterName)
}

func (kc *KubeClient) ListK0sControlPlanes(
	ctx context.Context, clusterName string,
) ([]unstructured.Unstructured, error) {
	GinkgoHelper()

	return kc.listResource(ctx, schema.GroupVersionResource{
		Group:    "controlplane.cluster.x-k8s.io",
		Version:  "v1beta1",
		Resource: "k0scontrolplanes",
	}, clusterName)
}
