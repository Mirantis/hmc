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
	"time"

	helmcontrollerv2 "github.com/fluxcd/helm-controller/api/v2"
	sourcev1 "github.com/fluxcd/source-controller/api/v1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	hmcmirantiscomv1alpha1 "github.com/Mirantis/hmc/api/v1alpha1"
	"github.com/Mirantis/hmc/internal/utils"
)

var _ = Describe("Management Controller", func() {
	Context("When reconciling a resource", func() {
		const resourceName = "test-resource"

		ctx := context.Background()

		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: "default",
		}
		management := &hmcmirantiscomv1alpha1.Management{}

		BeforeEach(func() {
			By("creating the custom resource for the Kind Management")
			err := k8sClient.Get(ctx, typeNamespacedName, management)
			if err != nil && apierrors.IsNotFound(err) {
				resource := &hmcmirantiscomv1alpha1.Management{
					ObjectMeta: metav1.ObjectMeta{
						Name:      resourceName,
						Namespace: "default",
					},
				}
				Expect(k8sClient.Create(ctx, resource)).To(Succeed())
			}
		})

		AfterEach(func() {
			resource := &hmcmirantiscomv1alpha1.Management{}
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the specific resource instance Management")
			Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
		})

		It("should successfully reconcile the resource", func() {
			// NOTE: this node just checks that the finalizer has been set
			By("Reconciling the created resource")
			controllerReconciler := &ManagementReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())
		})

		It("should successfully delete providers components on its removal", func() {
			const (
				mgmtName = "test-management-name-mgmt-removal"

				providerTemplateName              = "test-provider-template-name-mgmt-removal"
				providerTemplateUID               = types.UID("some-uid")
				providerTemplateRequiredComponent = "test-provider-for-required-mgmt-removal"

				someComponentName = "test-component-name-mgmt-removal"

				helmChartName, helmChartNamespace = "helm-chart-test-name", utils.DefaultSystemNamespace

				helmReleaseName      = someComponentName // WARN: helm release name should be equal to the component name
				helmReleaseNamespace = utils.DefaultSystemNamespace

				timeout  = time.Second * 10
				interval = time.Millisecond * 250
			)

			// NOTE: other tests for some reason are manipulating with the NS globally and interfer with each other,
			// so try to avoid depending on their implementation ignoring its removal
			By("Creating the hmc-system namespace")
			Expect(client.IgnoreAlreadyExists(k8sClient.Create(ctx, &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: utils.DefaultSystemNamespace,
				},
			}))).To(Succeed())
			Eventually(k8sClient.Get).WithArguments(ctx, client.ObjectKey{Name: utils.DefaultSystemNamespace}, &corev1.Namespace{}).
				WithTimeout(10 * time.Second).WithPolling(250 * time.Millisecond).Should(Succeed())

			By("Creating the Release object")
			release := &hmcmirantiscomv1alpha1.Release{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-release-name",
				},
				Spec: hmcmirantiscomv1alpha1.ReleaseSpec{
					Version: "test-version",
					HMC:     hmcmirantiscomv1alpha1.CoreProviderTemplate{Template: "test-release-hmc"},
					CAPI:    hmcmirantiscomv1alpha1.CoreProviderTemplate{Template: "test-release-capi"},
				},
			}
			Expect(k8sClient.Create(ctx, release)).To(Succeed())
			Eventually(k8sClient.Get).WithArguments(ctx, client.ObjectKeyFromObject(release), release).
				WithTimeout(10 * time.Second).WithPolling(250 * time.Millisecond).Should(Succeed())

			By("Creating a ProviderTemplate object for other required components")
			providerTemplateRequired := &hmcmirantiscomv1alpha1.ProviderTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name: providerTemplateRequiredComponent,
				},
				Spec: hmcmirantiscomv1alpha1.ProviderTemplateSpec{
					Helm: hmcmirantiscomv1alpha1.HelmSpec{
						ChartName:    "required-chart",
						ChartVersion: "required-version",
					},
				},
			}
			Expect(k8sClient.Create(ctx, providerTemplateRequired)).To(Succeed())
			providerTemplateRequired.Status = hmcmirantiscomv1alpha1.ProviderTemplateStatus{
				TemplateStatusCommon: hmcmirantiscomv1alpha1.TemplateStatusCommon{
					TemplateValidationStatus: hmcmirantiscomv1alpha1.TemplateValidationStatus{
						Valid: true,
					},
					ChartRef: &helmcontrollerv2.CrossNamespaceSourceReference{
						Kind:      sourcev1.HelmChartKind,
						Name:      "required-chart",
						Namespace: helmChartNamespace,
					},
				},
			}
			Expect(k8sClient.Status().Update(ctx, providerTemplateRequired)).To(Succeed())

			By("Creating a HelmRelease object for the removed component")
			helmRelease := &helmcontrollerv2.HelmRelease{
				ObjectMeta: metav1.ObjectMeta{
					Name:      helmReleaseName,
					Namespace: helmReleaseNamespace,
					Labels: map[string]string{
						hmcmirantiscomv1alpha1.HMCManagedLabelKey: hmcmirantiscomv1alpha1.HMCManagedLabelValue,
					},
				},
				Spec: helmcontrollerv2.HelmReleaseSpec{
					ChartRef: &helmcontrollerv2.CrossNamespaceSourceReference{
						Kind:      sourcev1.HelmChartKind,
						Name:      helmChartName,
						Namespace: helmChartNamespace,
					},
				},
			}
			Expect(k8sClient.Create(ctx, helmRelease)).To(Succeed())

			By("Creating a Management object with removed component in the spec and containing it in the status")
			mgmt := &hmcmirantiscomv1alpha1.Management{
				ObjectMeta: metav1.ObjectMeta{
					Name:       mgmtName,
					Finalizers: []string{hmcmirantiscomv1alpha1.ManagementFinalizer},
				},
				Spec: hmcmirantiscomv1alpha1.ManagementSpec{
					Release: release.Name,
					Core: &hmcmirantiscomv1alpha1.Core{
						HMC: hmcmirantiscomv1alpha1.Component{
							Template: providerTemplateRequiredComponent,
						},
						CAPI: hmcmirantiscomv1alpha1.Component{
							Template: providerTemplateRequiredComponent,
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, mgmt)).To(Succeed())
			mgmt.Status = hmcmirantiscomv1alpha1.ManagementStatus{
				AvailableProviders: []string{someComponentName},
				Components: map[string]hmcmirantiscomv1alpha1.ComponentStatus{
					someComponentName: {Template: providerTemplateName},
				},
			}
			Expect(k8sClient.Status().Update(ctx, mgmt)).To(Succeed())

			By("Checking created objects have expected spec and status")
			Eventually(func() error {
				// Management
				if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(mgmt), mgmt); err != nil {
					return err
				}
				if l := len(mgmt.Status.AvailableProviders); l != 1 {
					return fmt.Errorf("expected .status.availableProviders length to be exactly 1, got %d", l)
				}
				if l := len(mgmt.Status.Components); l != 1 {
					return fmt.Errorf("expected .status.components length to be exactly 2, got %d", l)
				}
				if v := mgmt.Status.Components[someComponentName]; v.Template != providerTemplateName {
					return fmt.Errorf("expected .status.components[%s] template be %s, got %s", someComponentName, providerTemplateName, v.Template)
				}

				// HelmRelease
				return k8sClient.Get(ctx, client.ObjectKey{Name: helmReleaseName, Namespace: helmReleaseNamespace}, &helmcontrollerv2.HelmRelease{})
			}).WithTimeout(timeout).WithPolling(interval).Should(Succeed())

			By("Reconciling the Management object")
			controllerReconciler := &ManagementReconciler{
				Client:          k8sClient,
				Scheme:          k8sClient.Scheme(),
				SystemNamespace: utils.DefaultSystemNamespace,
				DynamicClient:   dynamicClient,
			}

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: client.ObjectKeyFromObject(mgmt),
			})
			Expect(err).NotTo(HaveOccurred())

			By("Checking the HelmRelease objects have been removed")
			Eventually(func() bool {
				return apierrors.IsNotFound(k8sClient.Get(ctx, client.ObjectKeyFromObject(helmRelease), &helmcontrollerv2.HelmRelease{}))
			}).WithTimeout(timeout).WithPolling(interval).Should(BeTrue())

			By("Checking the Management object does not have the removed component in its spec")
			Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(mgmt), mgmt)).To(Succeed())
			Expect(mgmt.Status.AvailableProviders).To(BeEmpty())
			Expect(mgmt.Status.Components).To(HaveLen(2)) // required: capi, hmc
			Expect(mgmt.Status.Components).To(BeEquivalentTo(map[string]hmcmirantiscomv1alpha1.ComponentStatus{
				hmcmirantiscomv1alpha1.CoreHMCName:  {Success: true, Template: providerTemplateRequiredComponent},
				hmcmirantiscomv1alpha1.CoreCAPIName: {Success: true, Template: providerTemplateRequiredComponent},
			}))

			By("Removing the leftover objects")
			mgmt.Finalizers = nil
			Expect(k8sClient.Update(ctx, mgmt)).To(Succeed())
			Expect(k8sClient.Delete(ctx, mgmt)).To(Succeed())
			Eventually(func() bool {
				return apierrors.IsNotFound(k8sClient.Get(ctx, client.ObjectKeyFromObject(mgmt), &hmcmirantiscomv1alpha1.Management{}))
			}).WithTimeout(timeout).WithPolling(interval).Should(BeTrue())

			Expect(k8sClient.Delete(ctx, release)).To(Succeed())
			Eventually(func() bool {
				return apierrors.IsNotFound(k8sClient.Get(ctx, client.ObjectKeyFromObject(release), &hmcmirantiscomv1alpha1.Release{}))
			}).WithTimeout(timeout).WithPolling(interval).Should(BeTrue())

			Expect(k8sClient.Delete(ctx, providerTemplateRequired)).To(Succeed())
			Eventually(func() bool {
				return apierrors.IsNotFound(k8sClient.Get(ctx, client.ObjectKeyFromObject(providerTemplateRequired), &hmcmirantiscomv1alpha1.ProviderTemplate{}))
			}).WithTimeout(timeout).WithPolling(interval).Should(BeTrue())
		})
	})
})
