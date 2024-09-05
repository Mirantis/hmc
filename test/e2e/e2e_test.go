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

package e2e

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	"github.com/Mirantis/hmc/test/kubeclient"
	"github.com/Mirantis/hmc/test/managedcluster"
	"github.com/Mirantis/hmc/test/utils"
)

const (
	namespace          = "hmc-system"
	hmcControllerLabel = "app.kubernetes.io/name=hmc"
)

var _ = Describe("controller", Ordered, func() {
	BeforeAll(func() {
		By("building and deploying the controller-manager")
		cmd := exec.Command("make", "test-apply")
		_, err := utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred())
	})

	AfterAll(func() {
		By("removing the controller-manager")
		cmd := exec.Command("make", "test-destroy")
		_, err := utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred())
	})

	Context("Operator", func() {
		It("should run successfully", func() {
			kc, err := kubeclient.NewFromLocal(namespace)
			ExpectWithOffset(1, err).NotTo(HaveOccurred())
			ExpectWithOffset(1, kc.CreateAWSCredentialsKubeSecret(context.Background())).To(Succeed())

			By("validating that the hmc-controller and capi provider controllers are running")
			verifyControllersUp := func() error {
				if err := verifyControllerUp(kc, hmcControllerLabel, "hmc-controller-manager"); err != nil {
					return err
				}

				for _, provider := range []managedcluster.ProviderType{
					managedcluster.ProviderCAPI,
					managedcluster.ProviderAWS,
					managedcluster.ProviderAzure,
				} {
					// Ensure only one controller pod is running.
					if err := verifyControllerUp(kc, managedcluster.GetProviderLabel(provider), string(provider)); err != nil {
						return err
					}
				}

				return nil
			}
			Eventually(func() error {
				err := verifyControllersUp()
				if err != nil {
					_, _ = fmt.Fprintf(GinkgoWriter, "Controller pod validation failed: %v\n", err)
					return err
				}

				return nil
			}).WithTimeout(15 * time.Minute).WithPolling(10 * time.Second).Should(Succeed())
		})
	})

	Context("AWS Templates", func() {
		var (
			kc          *kubeclient.KubeClient
			deleteFunc  func() error
			clusterName string
			err         error
		)

		BeforeAll(func() {
			By("ensuring AWS credentials are set")
			kc, err = kubeclient.NewFromLocal(namespace)
			ExpectWithOffset(2, err).NotTo(HaveOccurred())
			ExpectWithOffset(2, kc.CreateAWSCredentialsKubeSecret(context.Background())).To(Succeed())
		})

		AfterEach(func() {
			// If we failed collect logs from each of the affiliated controllers
			// as well as the output of clusterctl to store as artifacts.
			if CurrentSpecReport().Failed() {
				By("collecting failure logs from controllers")
				collectLogArtifacts(kc, clusterName, managedcluster.ProviderAWS, managedcluster.ProviderCAPI)
			}

			// Delete the deployments if they were created.
			if deleteFunc != nil {
				By("deleting the deployment")
				err = deleteFunc()
				Expect(err).NotTo(HaveOccurred())
			}

			// Purge the AWS resources, the AfterAll for the controller will
			// clean up the management cluster.
			By("nuking remaining AWS resources")
			err = os.Setenv("CLUSTER_NAME", clusterName)
			Expect(err).NotTo(HaveOccurred())
			cmd := exec.Command("make", "dev-aws-nuke")
			_, err := utils.Run(cmd)
			ExpectWithOffset(2, err).NotTo(HaveOccurred())
		})

		for _, template := range []managedcluster.Template{
			managedcluster.TemplateAWSStandaloneCP,
			managedcluster.TemplateAWSHostedCP,
		} {
			It(fmt.Sprintf("should work with an AWS provider and %s template", template), func() {
				if template == managedcluster.TemplateAWSHostedCP {
					// TODO: Create AWS resources for hosted control plane.
					Skip("AWS hosted control plane not yet implemented")
				}

				By("creating a Deployment")
				d := managedcluster.GetUnstructured(managedcluster.ProviderAWS, template)
				clusterName = d.GetName()

				deleteFunc, err = kc.CreateManagedCluster(context.Background(), d)
				Expect(err).NotTo(HaveOccurred())

				By("waiting for infrastructure providers to deploy successfully")
				Eventually(func() error {
					return managedcluster.VerifyProviderDeployed(context.Background(), kc, clusterName)
				}).WithTimeout(30 * time.Minute).WithPolling(10 * time.Second).Should(Succeed())

				By("verify the deployment deletes successfully")
				err = deleteFunc()
				Expect(err).NotTo(HaveOccurred())
				Eventually(func() error {
					return managedcluster.VerifyProviderDeleted(context.Background(), kc, clusterName)
				}).WithTimeout(10 * time.Minute).WithPolling(10 * time.Second).Should(Succeed())
			})
		}
	})
})

func verifyControllerUp(kc *kubeclient.KubeClient, labelSelector string, name string) error {
	deployList, err := kc.Client.AppsV1().Deployments(kc.Namespace).List(context.Background(), metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		return fmt.Errorf("failed to list %s controller deployments: %w", name, err)
	}

	if len(deployList.Items) < 1 {
		return fmt.Errorf("expected at least 1 %s controller deployment, got %d",
			name, len(deployList.Items))
	}

	deployment := deployList.Items[0]

	// Ensure the deployment is not being deleted.
	if deployment.DeletionTimestamp != nil {
		return fmt.Errorf("controller pod: %s deletion timestamp should be nil, got: %v",
			deployment.Name, deployment.DeletionTimestamp)
	}
	// Ensure the deployment is running and has the expected name.
	if !strings.Contains(deployment.Name, "controller-manager") {
		return fmt.Errorf("controller deployment name %s does not contain 'controller-manager'", deployment.Name)
	}
	if deployment.Status.ReadyReplicas < 1 {
		return fmt.Errorf("controller deployment: %s does not yet have any ReadyReplicas", deployment.Name)
	}

	return nil
}

// collectLogArtfiacts collects log output from each the HMC controller,
// CAPI controller and the provider controller(s) as well as output from clusterctl
// and stores them in the test/e2e directory as artifacts.  If it fails it
// produces a warning message to the GinkgoWriter, but does not fail the test.
func collectLogArtifacts(kc *kubeclient.KubeClient, clusterName string, providerTypes ...managedcluster.ProviderType) {
	GinkgoHelper()

	filterLabels := []string{hmcControllerLabel}

	for _, providerType := range providerTypes {
		filterLabels = append(filterLabels, managedcluster.GetProviderLabel(providerType))
	}

	for _, label := range filterLabels {
		pods, _ := kc.Client.CoreV1().Pods(kc.Namespace).List(context.Background(), metav1.ListOptions{
			LabelSelector: label,
		})

		for _, pod := range pods.Items {
			req := kc.Client.CoreV1().Pods(kc.Namespace).GetLogs(pod.Name, &corev1.PodLogOptions{
				TailLines: ptr.To(int64(1000)),
			})
			podLogs, err := req.Stream(context.Background())
			if err != nil {
				utils.WarnError(fmt.Errorf("failed to get log stream for pod %s: %w", pod.Name, err))
				continue
			}
			defer podLogs.Close() //nolint:errcheck

			output, err := os.Create(fmt.Sprintf("./test/e2e/%s.log", pod.Name))
			if err != nil {
				utils.WarnError(fmt.Errorf("failed to create log file for pod %s: %w", pod.Name, err))
				continue
			}
			defer output.Close() //nolint:errcheck

			r := bufio.NewReader(podLogs)
			_, err = r.WriteTo(output)
			if err != nil {
				utils.WarnError(fmt.Errorf("failed to write log file for pod %s: %w", pod.Name, err))
			}
		}
	}

	cmd := exec.Command("./bin/clusterctl",
		"describe", "cluster", clusterName, "--namespace", namespace, "--show-conditions=all")
	output, err := utils.Run(cmd)
	if err != nil {
		utils.WarnError(fmt.Errorf("failed to get clusterctl log: %w", err))
		return
	}

	err = os.WriteFile(filepath.Join("test/e2e", "clusterctl.log"), output, 0644)
	if err != nil {
		utils.WarnError(fmt.Errorf("failed to write clusterctl log: %w", err))
	}
}
