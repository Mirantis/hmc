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
	crclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	hmc "github.com/Mirantis/hmc/api/v1alpha1"
	tc "github.com/Mirantis/hmc/test/objects/templatechain"
	tm "github.com/Mirantis/hmc/test/objects/templatemanagement"
)

var _ = Describe("Template Management Controller", func() {
	Context("When reconciling a resource", func() {
		const (
			tmName              = "hmc-tm"
			ctChainName         = "hmc-ct-chain"
			stChainName         = "hmc-st-chain"
			ctChainToDeleteName = "hmc-ct-chain-to-delete"
			stChainToDeleteName = "hmc-st-chain-to-delete"

			namespace1Name = "namespace1"
			namespace2Name = "namespace2"
			namespace3Name = "namespace3"

			ctChainUnmanagedName = "ct-chain-unmanaged"
			stChainUnmanagedName = "st-chain-unmanaged"
		)

		ctx := context.Background()

		systemNamespace := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: "hmc",
			},
		}

		namespace1 := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name:   namespace1Name,
				Labels: map[string]string{"environment": "dev", "test": "test"},
			},
		}
		namespace2 := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name:   namespace2Name,
				Labels: map[string]string{"environment": "prod"},
			},
		}
		namespace3 := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespace3Name}}

		accessRules := []hmc.AccessRule{
			{
				// Target namespaces: namespace1, namespace2
				TargetNamespaces: hmc.TargetNamespaces{
					Selector: &metav1.LabelSelector{
						MatchExpressions: []metav1.LabelSelectorRequirement{
							{
								Key:      "environment",
								Operator: metav1.LabelSelectorOpIn,
								Values:   []string{"prod", "dev"},
							},
						},
					},
				},
				ClusterTemplateChains: []string{ctChainName},
			},
			{
				// Target namespace: namespace1
				TargetNamespaces: hmc.TargetNamespaces{
					StringSelector: "environment=dev",
				},
				ClusterTemplateChains: []string{ctChainName},
				ServiceTemplateChains: []string{stChainName},
			},
			{
				// Target namespace: namespace3
				TargetNamespaces: hmc.TargetNamespaces{
					List: []string{namespace3Name},
				},
				ServiceTemplateChains: []string{stChainName},
			},
		}

		tm := tm.NewTemplateManagement(
			tm.WithName(tmName),
			tm.WithAccessRules(accessRules),
		)

		ctChain := tc.NewClusterTemplateChain(tc.WithName(ctChainName), tc.WithNamespace(systemNamespace.Name), tc.ManagedByHMC())
		stChain := tc.NewServiceTemplateChain(tc.WithName(stChainName), tc.WithNamespace(systemNamespace.Name), tc.ManagedByHMC())

		ctChainToDelete := tc.NewClusterTemplateChain(tc.WithName(ctChainToDeleteName), tc.WithNamespace(namespace2Name), tc.ManagedByHMC())
		stChainToDelete := tc.NewServiceTemplateChain(tc.WithName(stChainToDeleteName), tc.WithNamespace(namespace3Name), tc.ManagedByHMC())

		ctChainUnmanaged := tc.NewClusterTemplateChain(tc.WithName(ctChainUnmanagedName), tc.WithNamespace(namespace1Name))
		stChainUnmanaged := tc.NewServiceTemplateChain(tc.WithName(stChainUnmanagedName), tc.WithNamespace(namespace2Name))

		BeforeEach(func() {
			By("creating test namespaces")
			var err error
			for _, ns := range []*corev1.Namespace{systemNamespace, namespace1, namespace2, namespace3} {
				err = k8sClient.Get(ctx, types.NamespacedName{Name: ns.Name}, ns)
				if err != nil && errors.IsNotFound(err) {
					Expect(k8sClient.Create(ctx, ns)).To(Succeed())
				}
			}
			By("creating the custom resource for the Kind TemplateManagement")
			err = k8sClient.Get(ctx, types.NamespacedName{Name: tmName}, tm)
			if err != nil && errors.IsNotFound(err) {
				Expect(k8sClient.Create(ctx, tm)).To(Succeed())
			}

			By("creating custom resources for the Kind ClusterTemplateChain and ServiceTemplateChain")
			for _, chain := range []crclient.Object{
				ctChain, ctChainToDelete, ctChainUnmanaged,
				stChain, stChainToDelete, stChainUnmanaged,
			} {
				err = k8sClient.Get(ctx, types.NamespacedName{Name: chain.GetName(), Namespace: chain.GetNamespace()}, chain)
				if err != nil && errors.IsNotFound(err) {
					Expect(k8sClient.Create(ctx, chain)).To(Succeed())
				}
			}
		})

		AfterEach(func() {
			for _, chain := range []*hmc.ClusterTemplateChain{ctChain, ctChainToDelete, ctChainUnmanaged} {
				for _, ns := range []*corev1.Namespace{systemNamespace, namespace1, namespace2, namespace3} {
					chain.Namespace = ns.Name
					err := k8sClient.Delete(ctx, chain)
					Expect(crclient.IgnoreNotFound(err)).To(Succeed())
				}
			}
			for _, chain := range []*hmc.ServiceTemplateChain{stChain, stChainToDelete, stChainUnmanaged} {
				for _, ns := range []*corev1.Namespace{systemNamespace, namespace1, namespace2, namespace3} {
					chain.Namespace = ns.Name
					err := k8sClient.Delete(ctx, chain)
					Expect(crclient.IgnoreNotFound(err)).To(Succeed())
				}
			}
			for _, ns := range []*corev1.Namespace{namespace1, namespace2, namespace3} {
				err := k8sClient.Get(ctx, types.NamespacedName{Name: ns.Name}, ns)
				Expect(err).NotTo(HaveOccurred())
				By("Cleanup the namespace")
				Expect(k8sClient.Delete(ctx, ns)).To(Succeed())
			}
		})
		It("should successfully reconcile the resource", func() {
			By("Get unmanaged template chains before the reconciliation to verify it wasn't changed")
			ctChainUnmanagedBefore := &hmc.ClusterTemplateChain{}
			err := k8sClient.Get(ctx, types.NamespacedName{Namespace: ctChainUnmanaged.Namespace, Name: ctChainUnmanaged.Name}, ctChainUnmanagedBefore)
			Expect(err).NotTo(HaveOccurred())
			stChainUnmanagedBefore := &hmc.ServiceTemplateChain{}
			err = k8sClient.Get(ctx, types.NamespacedName{Namespace: stChainUnmanaged.Namespace, Name: stChainUnmanaged.Name}, stChainUnmanagedBefore)
			Expect(err).NotTo(HaveOccurred())

			By("Reconciling the created resource")
			controllerReconciler := &TemplateManagementReconciler{
				Client:          k8sClient,
				SystemNamespace: systemNamespace.Name,
			}
			_, err = controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: types.NamespacedName{Name: tmName},
			})
			Expect(err).NotTo(HaveOccurred())
			/*
				Expected state:
					* namespace1/hmc-ct-chain - should be created
					* namespace1/hmc-st-chain - should be created
					* namespace2/hmc-ct-chain - should be created
					* namespace3/hmc-st-chain - should be created
					* namespace1/ct-chain-unmanaged - should be unchanged (unmanaged by HMC)
					* namespace2/st-chain-unmanaged - should be unchanged (unmanaged by HMC)
					* namespace2/hmc-ct-chain-to-delete - should be deleted
					* namespace3/hmc-st-chain-to-delete - should be deleted
			*/
			verifyObjectCreated(ctx, namespace1Name, ctChain)
			verifyObjectCreated(ctx, namespace1Name, stChain)
			verifyObjectCreated(ctx, namespace2Name, ctChain)
			verifyObjectCreated(ctx, namespace3Name, stChain)

			verifyObjectUnchanged(ctx, namespace1Name, ctChainUnmanaged, ctChainUnmanagedBefore)
			verifyObjectUnchanged(ctx, namespace2Name, stChainUnmanaged, stChainUnmanagedBefore)

			verifyObjectDeleted(ctx, namespace2Name, ctChainToDelete)
			verifyObjectDeleted(ctx, namespace3Name, stChainToDelete)
		})
	})
})
