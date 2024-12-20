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
	"time"

	hcv2 "github.com/fluxcd/helm-controller/api/v2"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	hmc "github.com/Mirantis/hmc/api/v1alpha1"
)

var _ = Describe("ManagedCluster Controller", func() {
	Context("When reconciling a resource", func() {
		const (
			managedClusterName      = "test-managed-cluster"
			managedClusterNamespace = "test"

			templateName    = "test-template"
			svcTemplateName = "test-svc-template"
			credentialName  = "test-credential"
		)

		ctx := context.Background()

		typeNamespacedName := types.NamespacedName{
			Name:      managedClusterName,
			Namespace: managedClusterNamespace,
		}
		managedCluster := &hmc.ManagedCluster{}
		template := &hmc.ClusterTemplate{}
		svcTemplate := &hmc.ServiceTemplate{}
		management := &hmc.Management{}
		credential := &hmc.Credential{}
		namespace := &corev1.Namespace{}

		BeforeEach(func() {
			By("creating ManagedCluster namespace")
			err := k8sClient.Get(ctx, types.NamespacedName{Name: managedClusterNamespace}, namespace)
			if err != nil && errors.IsNotFound(err) {
				namespace = &corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name: managedClusterNamespace,
					},
				}
				Expect(k8sClient.Create(ctx, namespace)).To(Succeed())
			}

			By("creating the custom resource for the Kind ClusterTemplate")
			err = k8sClient.Get(ctx, typeNamespacedName, template)
			if err != nil && errors.IsNotFound(err) {
				template = &hmc.ClusterTemplate{
					ObjectMeta: metav1.ObjectMeta{
						Name:      templateName,
						Namespace: managedClusterNamespace,
					},
					Spec: hmc.ClusterTemplateSpec{
						Helm: hmc.HelmSpec{
							ChartRef: &hcv2.CrossNamespaceSourceReference{
								Kind:      "HelmChart",
								Name:      "ref-test",
								Namespace: "default",
							},
						},
					},
				}
				Expect(k8sClient.Create(ctx, template)).To(Succeed())
				template.Status = hmc.ClusterTemplateStatus{
					TemplateStatusCommon: hmc.TemplateStatusCommon{
						TemplateValidationStatus: hmc.TemplateValidationStatus{
							Valid: true,
						},
						Config: &apiextensionsv1.JSON{
							Raw: []byte(`{"foo":"bar"}`),
						},
					},
					Providers: hmc.Providers{"infrastructure-aws"},
				}
				Expect(k8sClient.Status().Update(ctx, template)).To(Succeed())
			}

			By("creating the custom resource for the Kind ServiceTemplate")
			err = k8sClient.Get(ctx, client.ObjectKey{Namespace: managedClusterNamespace, Name: svcTemplateName}, svcTemplate)
			if err != nil && errors.IsNotFound(err) {
				svcTemplate = &hmc.ServiceTemplate{
					ObjectMeta: metav1.ObjectMeta{
						Name:      svcTemplateName,
						Namespace: managedClusterNamespace,
					},
					Spec: hmc.ServiceTemplateSpec{
						Helm: hmc.HelmSpec{
							ChartRef: &hcv2.CrossNamespaceSourceReference{
								Kind:      "HelmChart",
								Name:      "ref-test",
								Namespace: "default",
							},
						},
					},
				}
				Expect(k8sClient.Create(ctx, svcTemplate)).To(Succeed())
				svcTemplate.Status = hmc.ServiceTemplateStatus{
					TemplateStatusCommon: hmc.TemplateStatusCommon{
						TemplateValidationStatus: hmc.TemplateValidationStatus{
							Valid: true,
						},
					},
				}
				Expect(k8sClient.Status().Update(ctx, svcTemplate)).To(Succeed())
			}

			By("creating the custom resource for the Kind Management")
			err = k8sClient.Get(ctx, typeNamespacedName, management)
			if err != nil && errors.IsNotFound(err) {
				management = &hmc.Management{
					ObjectMeta: metav1.ObjectMeta{
						Name: hmc.ManagementName,
					},
					Spec: hmc.ManagementSpec{
						Release: "test-release",
					},
				}
				Expect(k8sClient.Create(ctx, management)).To(Succeed())
				management.Status = hmc.ManagementStatus{
					AvailableProviders: hmc.Providers{"infrastructure-aws"},
				}
				Expect(k8sClient.Status().Update(ctx, management)).To(Succeed())
			}
			By("creating the custom resource for the Kind Credential")
			err = k8sClient.Get(ctx, typeNamespacedName, credential)
			if err != nil && errors.IsNotFound(err) {
				credential = &hmc.Credential{
					ObjectMeta: metav1.ObjectMeta{
						Name:      credentialName,
						Namespace: managedClusterNamespace,
					},
					Spec: hmc.CredentialSpec{
						IdentityRef: &corev1.ObjectReference{
							APIVersion: "infrastructure.cluster.x-k8s.io/v1beta2",
							Kind:       "AWSClusterStaticIdentity",
							Name:       "foo",
						},
					},
				}
				Expect(k8sClient.Create(ctx, credential)).To(Succeed())
				credential.Status = hmc.CredentialStatus{
					Ready: true,
				}
				Expect(k8sClient.Status().Update(ctx, credential)).To(Succeed())
			}

			By("creating the custom resource for the Kind ManagedCluster")
			err = k8sClient.Get(ctx, typeNamespacedName, managedCluster)
			if err != nil && errors.IsNotFound(err) {
				managedCluster = &hmc.ManagedCluster{
					ObjectMeta: metav1.ObjectMeta{
						Name:      managedClusterName,
						Namespace: managedClusterNamespace,
					},
					Spec: hmc.ManagedClusterSpec{
						Template:   templateName,
						Credential: credentialName,
						ServiceSpec: hmc.ServiceSpec{
							Services: []hmc.Service{
								{
									Template: svcTemplateName,
									Name:     "test-svc-name",
								},
							},
						},
					},
				}
				Expect(k8sClient.Create(ctx, managedCluster)).To(Succeed())
			}
		})

		AfterEach(func() {
			By("Cleanup")

			controllerReconciler := &ManagedClusterReconciler{
				Client: k8sClient,
			}

			Expect(k8sClient.Delete(ctx, managedCluster)).To(Succeed())
			// Running reconcile to remove the finalizer and delete the ManagedCluster
			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{NamespacedName: typeNamespacedName})
			Expect(err).NotTo(HaveOccurred())

			Eventually(k8sClient.Get, 1*time.Minute, 5*time.Second).WithArguments(ctx, typeNamespacedName, managedCluster).Should(HaveOccurred())

			Expect(k8sClient.Delete(ctx, template)).To(Succeed())
			Expect(k8sClient.Delete(ctx, management)).To(Succeed())
			Expect(k8sClient.Delete(ctx, namespace)).To(Succeed())
		})
		It("should successfully reconcile the resource", func() {
			By("Reconciling the created resource")
			controllerReconciler := &ManagedClusterReconciler{
				Client: k8sClient,
				Config: &rest.Config{},
			}

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())
		})
	})
})
