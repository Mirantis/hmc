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

	hmcmirantiscomv1alpha1 "github.com/Mirantis/hmc/api/v1alpha1"
	"github.com/Mirantis/hmc/test/objects/template"
	chain "github.com/Mirantis/hmc/test/objects/templatechain"
	tm "github.com/Mirantis/hmc/test/objects/templatemanagement"
)

var _ = Describe("Template Management Controller", func() {
	Context("When reconciling a resource", func() {
		const (
			tmName      = "hmc-tm"
			ctChainName = "hmc-ct-chain"
			stChainName = "hmc-st-chain"

			namespace1Name = "namespace1"
			namespace2Name = "namespace2"
			namespace3Name = "namespace3"

			ct1Name         = "tmpl"
			ct2Name         = "ct2"
			ct3Name         = "ct3"
			ctUnmanagedName = "ct-unmanaged"

			st1Name         = "tmpl"
			st2Name         = "st2"
			st3Name         = "st3"
			stUnmanagedName = "st-unmanaged"
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

		accessRules := []hmcmirantiscomv1alpha1.AccessRule{
			{
				// Target namespaces: namespace1, namespace2
				// ClusterTemplates: ct1, ct2
				TargetNamespaces: hmcmirantiscomv1alpha1.TargetNamespaces{
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
				// ClusterTemplates: ct1, ct2
				// ServiceTemplates: st1, st2
				TargetNamespaces: hmcmirantiscomv1alpha1.TargetNamespaces{
					StringSelector: "environment=dev",
				},
				ClusterTemplateChains: []string{ctChainName},
				ServiceTemplateChains: []string{stChainName},
			},
			{
				// Target namespace: namespace3
				// ServiceTemplates: st1, st2
				TargetNamespaces: hmcmirantiscomv1alpha1.TargetNamespaces{
					List: []string{namespace3Name},
				},
				ServiceTemplateChains: []string{stChainName},
			},
		}

		tm := tm.NewTemplateManagement(
			tm.WithName(tmName),
			tm.WithAccessRules(accessRules),
		)

		supportedClusterTemplates := []hmcmirantiscomv1alpha1.SupportedTemplate{
			{Name: ct1Name},
			{Name: ct2Name},
		}
		ctChain := chain.NewClusterTemplateChain(
			chain.WithName(ctChainName),
			chain.WithSupportedTemplates(supportedClusterTemplates),
		)

		supportedServiceTemplates := []hmcmirantiscomv1alpha1.SupportedTemplate{
			{Name: st1Name},
			{Name: st2Name},
		}
		stChain := chain.NewServiceTemplateChain(
			chain.WithName(stChainName),
			chain.WithSupportedTemplates(supportedServiceTemplates),
		)

		templateHelmSpec := hmcmirantiscomv1alpha1.HelmSpec{ChartName: "test"}
		ct1 := template.NewClusterTemplate(template.WithName(ct1Name), template.WithNamespace(systemNamespace.Name), template.WithHelmSpec(templateHelmSpec))
		ct2 := template.NewClusterTemplate(template.WithName(ct2Name), template.WithNamespace(systemNamespace.Name), template.WithHelmSpec(templateHelmSpec))
		ct3 := template.NewClusterTemplate(
			template.WithName(ct3Name),
			template.WithNamespace(namespace2Name),
			template.WithHelmSpec(templateHelmSpec),
			template.WithLabels(map[string]string{hmcmirantiscomv1alpha1.HMCManagedLabelKey: hmcmirantiscomv1alpha1.HMCManagedLabelValue}),
		)
		ctUnmanaged := template.NewClusterTemplate(template.WithName(ctUnmanagedName), template.WithNamespace(namespace1Name), template.WithHelmSpec(templateHelmSpec))

		st1 := template.NewServiceTemplate(template.WithName(st1Name), template.WithNamespace(systemNamespace.Name), template.WithHelmSpec(templateHelmSpec))
		st2 := template.NewServiceTemplate(template.WithName(st2Name), template.WithNamespace(systemNamespace.Name), template.WithHelmSpec(templateHelmSpec))
		st3 := template.NewServiceTemplate(
			template.WithName(st3Name),
			template.WithNamespace(namespace2Name),
			template.WithHelmSpec(templateHelmSpec),
			template.WithLabels(map[string]string{hmcmirantiscomv1alpha1.HMCManagedLabelKey: hmcmirantiscomv1alpha1.HMCManagedLabelValue}),
		)
		stUnmanaged := template.NewServiceTemplate(template.WithName(stUnmanagedName), template.WithNamespace(namespace2Name), template.WithHelmSpec(templateHelmSpec))

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
			By("creating the custom resource for the Kind ClusterTemplateChain")
			err = k8sClient.Get(ctx, types.NamespacedName{Name: ctChainName}, ctChain)
			if err != nil && errors.IsNotFound(err) {
				Expect(k8sClient.Create(ctx, ctChain)).To(Succeed())
			}
			By("creating the custom resource for the Kind ServiceTemplateChain")
			err = k8sClient.Get(ctx, types.NamespacedName{Name: stChainName}, stChain)
			if err != nil && errors.IsNotFound(err) {
				Expect(k8sClient.Create(ctx, stChain)).To(Succeed())
			}
			By("creating ClusterTemplates and ServiceTemplates")
			for _, template := range []crclient.Object{ct1, ct2, ct3, ctUnmanaged, st1, st2, st3, stUnmanaged} {
				err = k8sClient.Get(ctx, types.NamespacedName{Name: template.GetName(), Namespace: template.GetNamespace()}, template)
				if err != nil && errors.IsNotFound(err) {
					Expect(k8sClient.Create(ctx, template)).To(Succeed())
				}
			}
		})

		AfterEach(func() {
			for _, ns := range []*corev1.Namespace{namespace1, namespace2, namespace3} {
				err := k8sClient.Get(ctx, types.NamespacedName{Name: ns.Name}, ns)
				Expect(err).NotTo(HaveOccurred())
				By("Cleanup the namespace")
				Expect(k8sClient.Delete(ctx, ns)).To(Succeed())
			}

			ctChain := &hmcmirantiscomv1alpha1.ClusterTemplateChain{}
			err := k8sClient.Get(ctx, types.NamespacedName{Name: ctChainName}, ctChain)
			Expect(err).NotTo(HaveOccurred())
			By("Cleanup the specific resource instance ClusterTemplateChain")
			Expect(k8sClient.Delete(ctx, ctChain)).To(Succeed())

			stChain := &hmcmirantiscomv1alpha1.ServiceTemplateChain{}
			err = k8sClient.Get(ctx, types.NamespacedName{Name: stChainName}, stChain)
			Expect(err).NotTo(HaveOccurred())
			By("Cleanup the specific resource instance ServiceTemplateChain")
			Expect(k8sClient.Delete(ctx, stChain)).To(Succeed())
		})
		It("should successfully reconcile the resource", func() {
			By("Get unmanaged templates before the reconciliation to verify it wasn't changed")
			ctUnmanagedBefore := &hmcmirantiscomv1alpha1.ClusterTemplate{}
			err := k8sClient.Get(ctx, types.NamespacedName{Namespace: namespace1Name, Name: ctUnmanaged.Name}, ctUnmanagedBefore)
			Expect(err).NotTo(HaveOccurred())
			stUnmanagedBefore := &hmcmirantiscomv1alpha1.ServiceTemplate{}
			err = k8sClient.Get(ctx, types.NamespacedName{Namespace: namespace2Name, Name: stUnmanaged.Name}, stUnmanagedBefore)
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
					* namespace1/ct1 - should be created
					* namespace1/ct2 - should be created
					* namespace1/ctUnmanaged - should be unchanged (unmanaged by HMC)
					* namespace1/st1 - should be created
					* namespace1/st2 - should be created

					* namespace2/ct1 - should be created
					* namespace2/ct2 - should be created
					* namespace2/ct3 - should be deleted
					* namespace2/st3 - should be deleted
					* namespace2/stUnmanaged - should be unchanged (unmanaged by HMC)

					* namespace3/st1 - should be created
					* namespace3/st2 - should be created
			*/
			verifyTemplateCreated(ctx, namespace1Name, ct1)
			verifyTemplateCreated(ctx, namespace1Name, ct2)
			verifyTemplateUnchanged(ctx, namespace1Name, ctUnmanagedBefore, ctUnmanaged)
			verifyTemplateCreated(ctx, namespace1Name, st1)
			verifyTemplateCreated(ctx, namespace1Name, st2)

			verifyTemplateCreated(ctx, namespace2Name, ct1)
			verifyTemplateCreated(ctx, namespace2Name, ct2)
			verifyTemplateDeleted(ctx, namespace2Name, ct3)
			verifyTemplateDeleted(ctx, namespace2Name, st3)
			verifyTemplateUnchanged(ctx, namespace2Name, stUnmanagedBefore, stUnmanaged)

			verifyTemplateCreated(ctx, namespace3Name, st1)
			verifyTemplateCreated(ctx, namespace3Name, st2)
		})
	})
})

func verifyTemplateCreated(ctx context.Context, namespace string, tpl crclient.Object) {
	err := k8sClient.Get(ctx, types.NamespacedName{Namespace: namespace, Name: tpl.GetName()}, tpl)
	Expect(err).NotTo(HaveOccurred())
	checkHMCManagedLabelExistence(tpl.GetLabels())
}

func verifyTemplateDeleted(ctx context.Context, namespace string, tpl crclient.Object) {
	err := k8sClient.Get(ctx, types.NamespacedName{Namespace: namespace, Name: tpl.GetName()}, tpl)
	Expect(err).To(HaveOccurred())
}

func verifyTemplateUnchanged(ctx context.Context, namespace string, oldTpl, newTpl crclient.Object) {
	err := k8sClient.Get(ctx, types.NamespacedName{Namespace: namespace, Name: newTpl.GetName()}, newTpl)
	Expect(err).NotTo(HaveOccurred())
	Expect(oldTpl).To(Equal(newTpl))
	Expect(newTpl.GetLabels()).NotTo(HaveKeyWithValue(hmcmirantiscomv1alpha1.HMCManagedLabelKey, hmcmirantiscomv1alpha1.HMCManagedLabelValue))
}

func checkHMCManagedLabelExistence(labels map[string]string) {
	Expect(labels).To(HaveKeyWithValue(hmcmirantiscomv1alpha1.HMCManagedLabelKey, hmcmirantiscomv1alpha1.HMCManagedLabelValue))
}
