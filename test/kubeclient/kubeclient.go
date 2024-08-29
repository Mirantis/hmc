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
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/Mirantis/hmc/test/utils"
	. "github.com/onsi/ginkgo/v2"
	corev1 "k8s.io/api/core/v1"
	apiextensionsclientset "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

const (
	awsCredentialsSecretName = "aws-variables"
)

type KubeClient struct {
	Namespace string

	Client         kubernetes.Interface
	ExtendedClient apiextensionsclientset.Interface
	Config         *rest.Config
}

// NewFromLocal creates a new instance of KubeClient from a given namespace
// using the locally found kubeconfig.
func NewFromLocal(namespace string) (*KubeClient, error) {
	configBytes, err := getLocalKubeConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to get local kubeconfig: %w", err)
	}

	return new(configBytes, namespace)
}

// NewFromCluster creates a new KubeClient using the kubeconfig stored in the
// secret affiliated with the given clusterName.  Since it relies on fetching
// the kubeconfig from secret it needs an existing kubeclient.
func (kc *KubeClient) NewFromCluster(ctx context.Context, namespace, clusterName string) (*KubeClient, error) {
	secret, err := kc.Client.CoreV1().Secrets(kc.Namespace).Get(ctx, clusterName+"-kubeconfig", metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get cluster: %q kubeconfig secret: %w", clusterName, err)
	}

	secretData, ok := secret.Data["value"]
	if !ok {
		return nil, fmt.Errorf("kubeconfig secret %q has no 'value' key", clusterName)
	}

	return new(secretData, namespace)
}

// getLocalKubeConfig returns the kubeconfig file content.
func getLocalKubeConfig() ([]byte, error) {
	// Use the KUBECONFIG environment variable if it is set, otherwise use the
	// default path.
	kubeConfig, ok := os.LookupEnv("KUBECONFIG")
	if !ok {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("failed to get user home directory: %w", err)
		}

		kubeConfig = filepath.Join(homeDir, ".kube", "config")
	}

	configBytes, err := os.ReadFile(kubeConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to read %q: %w", kubeConfig, err)
	}

	return configBytes, nil
}

// new creates a new instance of KubeClient from a given namespace using
// the local kubeconfig.
func new(configBytes []byte, namespace string) (*KubeClient, error) {
	config, err := clientcmd.RESTConfigFromKubeConfig(configBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse kubeconfig: %w", err)
	}

	clientSet, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("could not initialize kubernetes client: %w", err)
	}

	extendedClientSet, err := apiextensionsclientset.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize apiextensions clientset: %w", err)
	}

	return &KubeClient{
		Namespace:      namespace,
		Client:         clientSet,
		ExtendedClient: extendedClientSet,
		Config:         config,
	}, nil
}

// CreateAWSCredentialsKubeSecret uses clusterawsadm to encode existing AWS
// credentials and create a secret named 'aws-credentials' in the given
// namespace if one does not already exist.
func (kc *KubeClient) CreateAWSCredentialsKubeSecret(ctx context.Context) error {
	_, err := kc.Client.CoreV1().Secrets(kc.Namespace).Get(ctx, awsCredentialsSecretName, metav1.GetOptions{})
	if !apierrors.IsNotFound(err) {
		return nil
	}

	cmd := exec.Command("./bin/clusterawsadm",
		"bootstrap", "credentials", "encode-as-profile", "--output", "rawSharedConfig")
	output, err := utils.Run(cmd)
	if err != nil {
		return fmt.Errorf("failed to encode AWS credentials with clusterawsadm: %w", err)
	}

	_, err = kc.Client.CoreV1().Secrets(kc.Namespace).Create(ctx, &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: awsCredentialsSecretName,
		},
		Data: map[string][]byte{
			"credentials": output,
		},
		Type: corev1.SecretTypeOpaque,
	}, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create AWS credentials secret: %w", err)
	}

	return nil
}

// GetDynamicClient returns a dynamic client for the given GroupVersionResource.
func (kc *KubeClient) GetDynamicClient(gvr schema.GroupVersionResource) (dynamic.ResourceInterface, error) {
	client, err := dynamic.NewForConfig(kc.Config)
	if err != nil {
		return nil, fmt.Errorf("failed to create dynamic client: %w", err)
	}

	return client.Resource(gvr).Namespace(kc.Namespace), nil
}

// CreateDeployment creates a deployment.hmc.mirantis.com in the given
// namespace and returns a DeleteFunc to clean up the deployment.
// The DeleteFunc is a no-op if the deployment has already been deleted.
func (kc *KubeClient) CreateDeployment(
	ctx context.Context, deployment *unstructured.Unstructured) (func() error, error) {
	kind := deployment.GetKind()

	if kind != "Deployment" {
		return nil, fmt.Errorf("expected kind Deployment, got: %s", kind)
	}

	client, err := kc.GetDynamicClient(schema.GroupVersionResource{
		Group:    "hmc.mirantis.com",
		Version:  "v1alpha1",
		Resource: "deployments",
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get dynamic client: %w", err)
	}

	_, err = client.Create(ctx, deployment, metav1.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to create Deployment: %w", err)
	}

	return func() error {
		err := client.Delete(ctx, deployment.GetName(), metav1.DeleteOptions{})
		if apierrors.IsNotFound(err) {
			return nil
		}
		return err
	}, nil
}

// GetCluster returns a Cluster resource by name.
func (kc *KubeClient) GetCluster(ctx context.Context, clusterName string) (*unstructured.Unstructured, error) {
	gvr := schema.GroupVersionResource{
		Group:    "cluster.x-k8s.io",
		Version:  "v1beta1",
		Resource: "clusters",
	}

	client, err := kc.GetDynamicClient(gvr)
	if err != nil {
		Fail(fmt.Sprintf("failed to get %s client: %v", gvr.Resource, err))
	}

	cluster, err := client.Get(ctx, clusterName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get %s %s: %w", gvr.Resource, clusterName, err)
	}

	return cluster, nil
}

// listResource returns a list of resources for the given GroupVersionResource
// affiliated with the given clusterName.
func (kc *KubeClient) listResource(
	ctx context.Context, gvr schema.GroupVersionResource, clusterName string) ([]unstructured.Unstructured, error) {
	client, err := kc.GetDynamicClient(gvr)
	if err != nil {
		Fail(fmt.Sprintf("failed to get %s client: %v", gvr.Resource, err))
	}

	resources, err := client.List(ctx, metav1.ListOptions{
		LabelSelector: "cluster.x-k8s.io/cluster-name=" + clusterName,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list %s: %w", gvr.Resource, err)
	}

	return resources.Items, nil
}

// ListMachines returns a list of Machine resources for the given cluster.
func (kc *KubeClient) ListMachines(ctx context.Context, clusterName string) ([]unstructured.Unstructured, error) {
	return kc.listResource(ctx, schema.GroupVersionResource{
		Group:    "cluster.x-k8s.io",
		Version:  "v1beta1",
		Resource: "machines",
	}, clusterName)
}

// ListMachineDeployments returns a list of MachineDeployment resources for the
// given cluster.
func (kc *KubeClient) ListMachineDeployments(
	ctx context.Context, clusterName string) ([]unstructured.Unstructured, error) {
	return kc.listResource(ctx, schema.GroupVersionResource{
		Group:    "cluster.x-k8s.io",
		Version:  "v1beta1",
		Resource: "machinedeployments",
	}, clusterName)
}

func (kc *KubeClient) ListK0sControlPlanes(
	ctx context.Context, clusterName string) ([]unstructured.Unstructured, error) {
	return kc.listResource(ctx, schema.GroupVersionResource{
		Group:    "controlplane.cluster.x-k8s.io",
		Version:  "v1beta1",
		Resource: "k0scontrolplanes",
	}, clusterName)
}
