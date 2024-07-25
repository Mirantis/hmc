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

	v2 "github.com/fluxcd/helm-controller/api/v2"
	sourcev1 "github.com/fluxcd/source-controller/api/v1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"helm.sh/helm/v3/pkg/chart"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	hmcmirantiscomv1alpha1 "github.com/Mirantis/hmc/api/v1alpha1"
)

var _ = Describe("Template Controller", func() {
	Context("When reconciling a resource", func() {
		const resourceName = "test-resource"
		const helmRepoNamespace = "default"
		const helmRepoName = "test-helmrepo"
		const helmChartName = "test-helmchart"
		const helmChartURL = "http://source-controller.hmc-system.svc.cluster.local./helmchart/hmc-system/test-chart/0.1.0.tar.gz"

		var fakeDownloadHelmChartFunc = func(context.Context, *sourcev1.Artifact) (*chart.Chart, error) {
			return &chart.Chart{
				Metadata: &chart.Metadata{
					APIVersion: "v2",
					Version:    "0.1.0",
					Name:       "test-chart",
				},
			}, nil
		}

		ctx := context.Background()

		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: "default", // TODO(user):Modify as needed
		}
		template := &hmcmirantiscomv1alpha1.Template{}
		helmRepo := &sourcev1.HelmRepository{}
		helmChart := &sourcev1.HelmChart{}

		BeforeEach(func() {
			By("creating helm repository")
			err := k8sClient.Get(ctx, types.NamespacedName{Name: helmRepoName, Namespace: helmRepoNamespace}, helmRepo)
			if err != nil && errors.IsNotFound(err) {
				helmRepo = &sourcev1.HelmRepository{
					ObjectMeta: metav1.ObjectMeta{
						Name:      helmRepoName,
						Namespace: helmRepoNamespace,
					},
					Spec: sourcev1.HelmRepositorySpec{
						URL: "oci://test/helmrepo",
					},
				}
				Expect(k8sClient.Create(ctx, helmRepo)).To(Succeed())
			}

			By("creating helm chart")
			err = k8sClient.Get(ctx, types.NamespacedName{Name: helmChartName, Namespace: helmRepoNamespace}, helmChart)
			if err != nil && errors.IsNotFound(err) {
				helmChart = &sourcev1.HelmChart{
					ObjectMeta: metav1.ObjectMeta{
						Name:      helmChartName,
						Namespace: helmRepoNamespace,
					},
					Spec: sourcev1.HelmChartSpec{
						SourceRef: sourcev1.LocalHelmChartSourceReference{
							Kind: sourcev1.HelmRepositoryKind,
							Name: helmRepoName,
						},
					},
				}
				Expect(k8sClient.Create(ctx, helmChart)).To(Succeed())
			}

			By("updating HelmChart status with artifact URL")
			helmChart.Status.URL = helmChartURL
			helmChart.Status.Artifact = &sourcev1.Artifact{
				URL:            helmChartURL,
				LastUpdateTime: metav1.Now(),
			}
			Expect(k8sClient.Status().Update(ctx, helmChart)).Should(Succeed())

			By("creating the custom resource for the Kind Template")
			err = k8sClient.Get(ctx, typeNamespacedName, template)
			if err != nil && errors.IsNotFound(err) {
				resource := &hmcmirantiscomv1alpha1.Template{
					ObjectMeta: metav1.ObjectMeta{
						Name:      resourceName,
						Namespace: "default",
					},
					Spec: hmcmirantiscomv1alpha1.TemplateSpec{
						Helm: hmcmirantiscomv1alpha1.HelmSpec{
							ChartRef: &v2.CrossNamespaceSourceReference{
								Kind:      "HelmChart",
								Name:      helmChartName,
								Namespace: helmRepoNamespace,
							},
						},
						Type: hmcmirantiscomv1alpha1.TemplateTypeDeployment,
					},
					// TODO(user): Specify other spec details if needed.
				}
				Expect(k8sClient.Create(ctx, resource)).To(Succeed())
			}
		})

		AfterEach(func() {
			// TODO(user): Cleanup logic after each test, like removing the resource instance.
			resource := &hmcmirantiscomv1alpha1.Template{}
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the specific resource instance Template")
			Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
		})
		It("should successfully reconcile the resource", func() {
			By("Reconciling the created resource")
			controllerReconciler := &TemplateReconciler{
				Client:                k8sClient,
				Scheme:                k8sClient.Scheme(),
				downloadHelmChartFunc: fakeDownloadHelmChartFunc,
			}

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())
			// TODO(user): Add more specific assertions depending on your controller's reconciliation logic.
			// Example: If you expect a certain status condition after reconciliation, verify it here.
		})
	})
})
