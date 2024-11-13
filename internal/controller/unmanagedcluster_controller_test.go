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

package controller

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"
	capi "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/secret"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	hmc "github.com/Mirantis/hmc/api/v1alpha1"
)

var _ = Describe("UnmanagedCluster Controller", func() {
	Context("When reconciling a resource", func() {
		const (
			unmanagedClusterName      = "test-managed-cluster"
			unmanagedClusterNamespace = "default"
		)

		ctx := context.Background()

		typeNamespacedName := types.NamespacedName{
			Name:      unmanagedClusterName,
			Namespace: unmanagedClusterNamespace,
		}
		unmanagedcluster := &hmc.UnmanagedCluster{}

		BeforeEach(func() {
			By("creating the custom resource for the Kind UnmanagedCluster")

			secretName := secret.Name(unmanagedClusterName, secret.Kubeconfig)
			secret := &corev1.Secret{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Secret",
					APIVersion: "v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      secretName,
					Namespace: unmanagedClusterNamespace,
					Labels:    map[string]string{capi.ClusterNameLabel: unmanagedClusterName},
				},
				Data: map[string][]byte{secret.KubeconfigDataName: generateTestKubeConfig()},
			}

			err := k8sClient.Get(ctx, types.NamespacedName{Name: secretName, Namespace: unmanagedClusterNamespace}, secret)
			if err != nil && errors.IsNotFound(err) {
				Expect(k8sClient.Create(ctx, secret)).To(Succeed())
			}

			err = k8sClient.Get(ctx, typeNamespacedName, unmanagedcluster)
			if err != nil && errors.IsNotFound(err) {
				resource := &hmc.UnmanagedCluster{
					ObjectMeta: metav1.ObjectMeta{
						Name:      unmanagedClusterName,
						Namespace: unmanagedClusterNamespace,
					},
					Spec: hmc.UnmanagedClusterSpec{
						Services:         nil,
						ServicesPriority: 1,
						StopOnConflict:   true,
					},
				}
				Expect(k8sClient.Create(ctx, resource)).To(Succeed())
			}
		})

		AfterEach(func() {
			resource := &hmc.UnmanagedCluster{}
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the specific resource instance UnmanagedCluster")
			Expect(k8sClient.Delete(ctx, resource)).To(Succeed())

			secretName := secret.Name(unmanagedClusterName, secret.Kubeconfig)
			Expect(k8sClient.Delete(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{
				Name:      secretName,
				Namespace: unmanagedClusterNamespace,
			}})).To(Succeed())

			Expect(k8sClient.Delete(ctx,
				&capi.Cluster{
					ObjectMeta: metav1.ObjectMeta{
						Name:      unmanagedClusterName,
						Namespace: unmanagedClusterNamespace,
					},
				})).To(Succeed())
		})

		It("should successfully reconcile the resource", func() {
			By("Reconciling the created resource")
			controllerReconciler := &UnmanagedClusterReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())
		})
	})
})

func generateTestKubeConfig() []byte {
	GinkgoHelper()
	clusters := make(map[string]*api.Cluster)
	clusters["default-cluster"] = &api.Cluster{
		Server:                   cfg.Host,
		CertificateAuthorityData: cfg.CAData,
	}
	contexts := make(map[string]*api.Context)
	contexts["default-context"] = &api.Context{
		Cluster:  "default-cluster",
		AuthInfo: "default-user",
	}
	authinfos := make(map[string]*api.AuthInfo)
	authinfos["default-user"] = &api.AuthInfo{
		ClientCertificateData: cfg.CertData,
		ClientKeyData:         cfg.KeyData,
	}
	clientConfig := api.Config{
		Kind:           "Config",
		APIVersion:     "v1",
		Clusters:       clusters,
		Contexts:       contexts,
		CurrentContext: "default-context",
		AuthInfos:      authinfos,
	}

	kubecfg, err := clientcmd.Write(clientConfig)
	Expect(err).NotTo(HaveOccurred())
	return kubecfg
}
