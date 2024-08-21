package kubeclient

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/Mirantis/hmc/test/utils"
	corev1 "k8s.io/api/core/v1"
	apiextensionsclientset "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

const (
	awsCredentialsSecretName = "aws-credentials"
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
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get user home directory: %w", err)
	}

	// Use the KUBECONFIG environment variable if it is set, otherwise use the
	// default path.
	kubeConfig, ok := os.LookupEnv("KUBECONFIG")
	if !ok {
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
