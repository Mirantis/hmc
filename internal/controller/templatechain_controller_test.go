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
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	crclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	hmcmirantiscomv1alpha1 "github.com/Mirantis/hmc/api/v1alpha1"
	"github.com/Mirantis/hmc/internal/utils"
	"github.com/Mirantis/hmc/test/objects/template"
)

var _ = Describe("Template Chain Controller", func() {
	Context("When reconciling a resource", func() {
		const (
			ctChainName = "ct-chain"
			stChainName = "st-chain"
		)

		ctx := context.Background()

		namespace := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-chains",
			},
		}

		templateHelmSpec := hmcmirantiscomv1alpha1.HelmSpec{ChartName: "test"}

		ctTemplates := map[string]*hmcmirantiscomv1alpha1.ClusterTemplate{
			// Should be created in target namespace
			"test": template.NewClusterTemplate(
				template.WithName("test"),
				template.WithNamespace(utils.DefaultSystemNamespace),
				template.WithHelmSpec(templateHelmSpec),
			),
			// Should be created in target namespace
			"ct0": template.NewClusterTemplate(
				template.WithName("ct0"),
				template.WithNamespace(utils.DefaultSystemNamespace),
				template.WithHelmSpec(templateHelmSpec),
			),
			// Should be deleted (not in the list of supported templates)
			"ct1": template.NewClusterTemplate(
				template.WithName("ct1"),
				template.WithNamespace(namespace.Name),
				template.WithHelmSpec(templateHelmSpec),
				template.ManagedByHMC(),
			),
			// Should be unchanged (unmanaged)
			"ct2": template.NewClusterTemplate(
				template.WithName("ct2"),
				template.WithNamespace(namespace.Name),
				template.WithHelmSpec(templateHelmSpec),
			),
		}
		stTemplates := map[string]*hmcmirantiscomv1alpha1.ServiceTemplate{
			// Should be created in target namespace
			"test": template.NewServiceTemplate(
				template.WithName("test"),
				template.WithNamespace(utils.DefaultSystemNamespace),
				template.WithHelmSpec(templateHelmSpec),
			),
			// Should be created in target namespace
			"st0": template.NewServiceTemplate(
				template.WithName("st0"),
				template.WithNamespace(utils.DefaultSystemNamespace),
				template.WithHelmSpec(templateHelmSpec),
			),
			// Should be deleted (not in the list of supported templates)
			"st1": template.NewServiceTemplate(
				template.WithName("st1"),
				template.WithNamespace(namespace.Name),
				template.WithHelmSpec(templateHelmSpec),
				template.ManagedByHMC(),
			),
			// Should be unchanged (unmanaged)
			"st2": template.NewServiceTemplate(
				template.WithName("st2"),
				template.WithNamespace(namespace.Name),
				template.WithHelmSpec(templateHelmSpec),
			),
		}

		ctChainNamespacedName := types.NamespacedName{
			Name:      ctChainName,
			Namespace: namespace.Name,
		}

		stChainNamespacedName := types.NamespacedName{
			Name:      stChainName,
			Namespace: namespace.Name,
		}
		clusterTemplateChain := &hmcmirantiscomv1alpha1.ClusterTemplateChain{}
		serviceTemplateChain := &hmcmirantiscomv1alpha1.ServiceTemplateChain{}
		supportedClusterTemplates := []hmcmirantiscomv1alpha1.SupportedTemplate{
			{
				Name: "test",
			},
			{
				Name: "ct0",
			},
			// Does not exist in the system namespace
			{
				Name: "ct3",
			},
		}
		supportedServiceTemplates := []hmcmirantiscomv1alpha1.SupportedTemplate{
			{
				Name: "test",
			},
			{
				Name: "st0",
			},
			// Does not exist in the system namespace
			{
				Name: "st3",
			},
		}

		BeforeEach(func() {
			By("creating the system and test namespaces")
			for _, ns := range []string{namespace.Name, utils.DefaultSystemNamespace} {
				namespace := &corev1.Namespace{}
				err := k8sClient.Get(ctx, types.NamespacedName{Name: ns}, namespace)
				if err != nil && errors.IsNotFound(err) {
					namespace := &corev1.Namespace{
						ObjectMeta: metav1.ObjectMeta{
							Name: ns,
						},
					}
					Expect(k8sClient.Create(ctx, namespace)).To(Succeed())
				}
			}

			By("creating the custom resource for the Kind ClusterTemplateChain")
			err := k8sClient.Get(ctx, ctChainNamespacedName, clusterTemplateChain)
			if err != nil && errors.IsNotFound(err) {
				resource := &hmcmirantiscomv1alpha1.ClusterTemplateChain{
					ObjectMeta: metav1.ObjectMeta{
						Name:      ctChainName,
						Namespace: namespace.Name,
					},
					Spec: hmcmirantiscomv1alpha1.TemplateChainSpec{SupportedTemplates: supportedClusterTemplates},
				}
				Expect(k8sClient.Create(ctx, resource)).To(Succeed())
			}
			By("creating the custom resource for the Kind ServiceTemplateChain")
			err = k8sClient.Get(ctx, stChainNamespacedName, serviceTemplateChain)
			if err != nil && errors.IsNotFound(err) {
				resource := &hmcmirantiscomv1alpha1.ServiceTemplateChain{
					ObjectMeta: metav1.ObjectMeta{
						Name:      stChainName,
						Namespace: namespace.Name,
					},
					Spec: hmcmirantiscomv1alpha1.TemplateChainSpec{SupportedTemplates: supportedServiceTemplates},
				}
				Expect(k8sClient.Create(ctx, resource)).To(Succeed())
			}
			By("creating the custom resource for the Kind ClusterTemplate")
			for name, template := range ctTemplates {
				ct := &hmcmirantiscomv1alpha1.ClusterTemplate{}
				err = k8sClient.Get(ctx, types.NamespacedName{Name: name, Namespace: utils.DefaultSystemNamespace}, ct)
				if err != nil && errors.IsNotFound(err) {
					Expect(k8sClient.Create(ctx, template)).To(Succeed())
				}
			}
			for name, template := range stTemplates {
				st := &hmcmirantiscomv1alpha1.ServiceTemplate{}
				err = k8sClient.Get(ctx, types.NamespacedName{Name: name, Namespace: utils.DefaultSystemNamespace}, st)
				if err != nil && errors.IsNotFound(err) {
					Expect(k8sClient.Create(ctx, template)).To(Succeed())
				}
			}
		})

		AfterEach(func() {
			for _, template := range []*hmcmirantiscomv1alpha1.ClusterTemplate{
				ctTemplates["test"], ctTemplates["ct0"], ctTemplates["ct1"], ctTemplates["ct2"],
			} {
				err := k8sClient.Delete(ctx, template)
				Expect(crclient.IgnoreNotFound(err)).To(Succeed())
			}
			for _, template := range []*hmcmirantiscomv1alpha1.ServiceTemplate{
				stTemplates["test"], stTemplates["st0"], stTemplates["st1"], stTemplates["st2"],
			} {
				err := k8sClient.Delete(ctx, template)
				Expect(crclient.IgnoreNotFound(err)).To(Succeed())
			}

			clusterTemplateChainResource := &hmcmirantiscomv1alpha1.ClusterTemplateChain{}
			err := k8sClient.Get(ctx, ctChainNamespacedName, clusterTemplateChainResource)
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the specific resource instance ClusterTemplateChain")
			Expect(k8sClient.Delete(ctx, clusterTemplateChainResource)).To(Succeed())

			serviceTemplateChainResource := &hmcmirantiscomv1alpha1.ServiceTemplateChain{}
			err = k8sClient.Get(ctx, stChainNamespacedName, serviceTemplateChainResource)
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the specific resource instance ServiceTemplateChain")
			Expect(k8sClient.Delete(ctx, serviceTemplateChainResource)).To(Succeed())

			By("Cleanup the namespace")
			err = k8sClient.Get(ctx, types.NamespacedName{Name: namespace.Name}, namespace)
			Expect(err).NotTo(HaveOccurred())
			Expect(crclient.IgnoreNotFound(k8sClient.Delete(ctx, namespace))).To(Succeed())
		})
		It("should successfully reconcile the resource", func() {
			By("Get unmanaged templates before the reconciliation to verify it wasn't changed")
			ctUnmanagedBefore := &hmcmirantiscomv1alpha1.ClusterTemplate{}
			err := k8sClient.Get(ctx, types.NamespacedName{Namespace: namespace.Name, Name: "ct2"}, ctUnmanagedBefore)
			Expect(err).NotTo(HaveOccurred())
			stUnmanagedBefore := &hmcmirantiscomv1alpha1.ServiceTemplate{}
			err = k8sClient.Get(ctx, types.NamespacedName{Namespace: namespace.Name, Name: "st2"}, stUnmanagedBefore)
			Expect(err).NotTo(HaveOccurred())

			templateChainReconciler := TemplateChainReconciler{
				Client:          k8sClient,
				SystemNamespace: utils.DefaultSystemNamespace,
			}
			By("Reconciling the ClusterTemplateChain resource")
			clusterTemplateChainReconciler := &ClusterTemplateChainReconciler{TemplateChainReconciler: templateChainReconciler}
			_, err = clusterTemplateChainReconciler.Reconcile(ctx, reconcile.Request{NamespacedName: ctChainNamespacedName})
			Expect(err).NotTo(HaveOccurred())

			By("Reconciling the ServiceTemplateChain resource")
			serviceTemplateChainReconciler := &ServiceTemplateChainReconciler{TemplateChainReconciler: templateChainReconciler}
			_, err = serviceTemplateChainReconciler.Reconcile(ctx, reconcile.Request{NamespacedName: stChainNamespacedName})
			Expect(err).NotTo(HaveOccurred())

			/*
				Expected state:
					* test/test - should be created
					* test/ct0 - should be created
					* test/ct1 - should be deleted
					* test/ct2 - should be unchanged (unmanaged by HMC)

					* test/test - should be created
					* test/st0 - should be created
					* test/st1 - should be deleted
					* test/st2 - should be unchanged (unmanaged by HMC)
			*/

			verifyObjectCreated(ctx, namespace.Name, ctTemplates["test"])
			verifyObjectCreated(ctx, namespace.Name, ctTemplates["ct0"])
			verifyObjectDeleted(ctx, namespace.Name, ctTemplates["ct1"])
			verifyObjectUnchanged(ctx, namespace.Name, ctUnmanagedBefore, ctTemplates["ct2"])

			verifyObjectCreated(ctx, namespace.Name, stTemplates["test"])
			verifyObjectCreated(ctx, namespace.Name, stTemplates["st0"])
			verifyObjectDeleted(ctx, namespace.Name, stTemplates["st1"])
			verifyObjectUnchanged(ctx, namespace.Name, stUnmanagedBefore, stTemplates["st2"])
		})
	})
})

func verifyObjectCreated(ctx context.Context, namespace string, obj crclient.Object) {
	By(fmt.Sprintf("Verifying existence of %s/%s", namespace, obj.GetName()))
	err := k8sClient.Get(ctx, types.NamespacedName{Namespace: namespace, Name: obj.GetName()}, obj)
	Expect(err).NotTo(HaveOccurred())
	checkHMCManagedLabelExistence(obj.GetLabels())
}

func verifyObjectDeleted(ctx context.Context, namespace string, obj crclient.Object) {
	By(fmt.Sprintf("Verifying %s/%s is deleted", namespace, obj.GetName()))
	err := k8sClient.Get(ctx, types.NamespacedName{Namespace: namespace, Name: obj.GetName()}, obj)
	Expect(err).To(HaveOccurred())
}

func verifyObjectUnchanged(ctx context.Context, namespace string, oldObj, newObj crclient.Object) {
	By(fmt.Sprintf("Verifying %s/%s is unchanged", namespace, newObj.GetName()))
	err := k8sClient.Get(ctx, types.NamespacedName{Namespace: namespace, Name: newObj.GetName()}, newObj)
	Expect(err).NotTo(HaveOccurred())
	Expect(oldObj).To(Equal(newObj))
	Expect(newObj.GetLabels()).NotTo(HaveKeyWithValue(hmcmirantiscomv1alpha1.HMCManagedLabelKey, hmcmirantiscomv1alpha1.HMCManagedLabelValue))
}

func checkHMCManagedLabelExistence(labels map[string]string) {
	Expect(labels).To(HaveKeyWithValue(hmcmirantiscomv1alpha1.HMCManagedLabelKey, hmcmirantiscomv1alpha1.HMCManagedLabelValue))
}
