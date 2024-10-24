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

	"github.com/k0sproject/k0smotron/api/infrastructure/v1beta1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	capi "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/secret"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	hmc "github.com/Mirantis/hmc/api/v1alpha1"
)

var _ = Describe("UnmanagedMachine Controller", func() {
	Context("When reconciling a resource", func() {
		const (
			unmanagedClusterName      = "test-managed-cluster"
			unmanagedClusterNamespace = "default"
			unmanagedMachineName      = "test-machine"
		)
		ctx := context.Background()

		typeNamespacedName := types.NamespacedName{
			Name:      unmanagedMachineName,
			Namespace: unmanagedClusterNamespace,
		}
		unmanagedmachine := &hmc.UnmanagedMachine{}

		BeforeEach(func() {
			By("creating the custom resource for the Kind UnmanagedCluster")
			Expect(v1beta1.AddToScheme(k8sClient.Scheme())).To(Succeed())
			Expect(capi.AddToScheme(k8sClient.Scheme())).To(Succeed())
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

			cluster := &capi.Cluster{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Cluster",
					APIVersion: capi.GroupVersion.Identifier(),
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      unmanagedClusterName,
					Namespace: unmanagedClusterNamespace,
				},
			}
			err = k8sClient.Get(ctx, typeNamespacedName, cluster)
			if err != nil && errors.IsNotFound(err) {
				Expect(k8sClient.Create(ctx, cluster)).To(Succeed())
			}

			By("creating the custom resource for the Kind UnmanagedMachine")
			Expect(k8sClient.Create(ctx, &corev1.Node{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Node",
					APIVersion: "v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: unmanagedMachineName,
				},
			})).To(Succeed())

			err = k8sClient.Get(ctx, typeNamespacedName, unmanagedmachine)
			if err != nil && errors.IsNotFound(err) {
				resource := &hmc.UnmanagedMachine{
					ObjectMeta: metav1.ObjectMeta{
						Name:      unmanagedMachineName,
						Namespace: "default",
					},
					Spec: hmc.UnmanagedMachineSpec{
						ProviderID:  unmanagedMachineName,
						ClusterName: unmanagedClusterName,
					},
				}
				Expect(k8sClient.Create(ctx, resource)).To(Succeed())
			}
		})

		AfterEach(func() {
			resource := &hmc.UnmanagedMachine{}
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the specific resource instance UnmanagedMachine")
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

			Expect(k8sClient.Delete(ctx, &corev1.Node{ObjectMeta: metav1.ObjectMeta{
				Name:      unmanagedMachineName,
				Namespace: unmanagedClusterNamespace,
			}})).To(Succeed())
		})
		It("should successfully reconcile the resource", func() {
			By("Reconciling the created resource")
			controllerReconciler := &UnmanagedMachineReconciler{
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
