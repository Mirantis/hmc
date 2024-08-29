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

	"github.com/Mirantis/hmc/test/deployment"
	"github.com/Mirantis/hmc/test/kubeclient"
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

			By("validating that the hmc-controller and capi provider controllers are running")
			verifyControllersUp := func() error {
				if err := verifyControllerUp(kc, hmcControllerLabel, "hmc-controller-manager"); err != nil {
					return err
				}

				for _, provider := range []deployment.ProviderType{
					deployment.ProviderCAPI,
					deployment.ProviderAWS,
					deployment.ProviderAzure,
				} {
					// Ensure only one controller pod is running.
					if err := verifyControllerUp(kc, deployment.GetProviderLabel(provider), string(provider)); err != nil {
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
				collectLogArtifacts(kc, clusterName, deployment.ProviderAWS, deployment.ProviderCAPI)
			}

			// Delete the deployments if they were created.
			if deleteFunc != nil {
				err = deleteFunc()
				Expect(err).NotTo(HaveOccurred())
			}

			// Purge the AWS resources, the AfterAll for the controller will
			// clean up the management cluster.
			err = os.Setenv("CLUSTER_NAME", clusterName)
			Expect(err).NotTo(HaveOccurred())
			cmd := exec.Command("make", "dev-aws-nuke")
			_, err := utils.Run(cmd)
			ExpectWithOffset(2, err).NotTo(HaveOccurred())
		})

		for _, template := range []deployment.Template{deployment.TemplateAWSStandaloneCP, deployment.TemplateAWSHostedCP} {
			It(fmt.Sprintf("should work with an AWS provider and %s template", template), func() {
				if template == deployment.TemplateAWSHostedCP {
					// TODO: Create AWS resources for hosted control plane.
					Skip("AWS hosted control plane not yet implemented")
				}

				By("creating a Deployment")
				d := deployment.GetUnstructuredDeployment(deployment.ProviderAWS, template)
				clusterName = d.GetName()

				deleteFunc, err = kc.CreateDeployment(context.Background(), d)
				Expect(err).NotTo(HaveOccurred())

				By("waiting for infrastructure providers to deploy successfully")
				Eventually(func() error {
					return deployment.VerifyProviderDeployed(context.Background(), kc, clusterName)
				}).WithTimeout(30 * time.Minute).WithPolling(10 * time.Second).Should(Succeed())

				By("verify the deployment deletes successfully")
				err = deleteFunc()
				Expect(err).NotTo(HaveOccurred())
				Eventually(func() error {
					return deployment.VerifyProviderDeleted(context.Background(), kc, clusterName)
				}).WithTimeout(10 * time.Minute).WithPolling(10 * time.Second).Should(Succeed())
			})
		}
	})
})

func verifyControllerUp(kc *kubeclient.KubeClient, labelSelector string, name string) error {
	podList, err := kc.Client.CoreV1().Pods(kc.Namespace).List(context.Background(), metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		return fmt.Errorf("failed to list %s controller pods: %v", name, err)
	}

	if len(podList.Items) != 1 {
		return fmt.Errorf("expected 1 %s controller pod, got %d", name, len(podList.Items))
	}

	controllerPod := podList.Items[0]

	// Ensure the pod is not being deleted.
	if controllerPod.DeletionTimestamp != nil {
		return fmt.Errorf("deletion timestamp should be nil, got: %v", controllerPod)
	}
	// Ensure the pod is running and has the expected name.
	if !strings.Contains(controllerPod.Name, "controller-manager") {
		return fmt.Errorf("controller pod name %s does not contain 'controller-manager'", controllerPod.Name)
	}
	if controllerPod.Status.Phase != "Running" {
		return fmt.Errorf("controller pod in %s status", controllerPod.Status.Phase)
	}

	return nil
}

// collectLogArtfiacts collects log output from each the HMC controller,
// CAPI controller and the provider controller(s) as well as output from clusterctl
// and stores them in the test/e2e directory as artifacts.
// We could do this at the end or we could use Kubernetes' CopyPodLogs from
// https://github.com/kubernetes/kubernetes/blob/v1.31.0/test/e2e/storage/podlogs/podlogs.go#L88
// to stream the logs to GinkgoWriter during the test.
func collectLogArtifacts(kc *kubeclient.KubeClient, clusterName string, providerTypes ...deployment.ProviderType) {
	GinkgoHelper()

	filterLabels := []string{hmcControllerLabel}

	for _, providerType := range providerTypes {
		filterLabels = append(filterLabels, deployment.GetProviderLabel(providerType))
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
			Expect(err).NotTo(HaveOccurred(), "failed to get log stream for pod %s", pod.Name)
			defer Expect(podLogs.Close()).NotTo(HaveOccurred())

			output, err := os.Create(fmt.Sprintf("test/e2e/%s.log", pod.Name))
			Expect(err).NotTo(HaveOccurred(), "failed to create log file for pod %s", pod.Name)
			defer Expect(output.Close()).NotTo(HaveOccurred())

			r := bufio.NewReader(podLogs)
			_, err = r.WriteTo(output)
			Expect(err).NotTo(HaveOccurred(), "failed to write log file for pod %s", pod.Name)
		}
	}

	cmd := exec.Command("./bin/clusterctl",
		"describe", "cluster", clusterName, "--show-conditions=all")
	output, err := utils.Run(cmd)
	Expect(err).NotTo(HaveOccurred(), "failed to get clusterctl log")

	err = os.WriteFile(filepath.Join("test/e2e", "clusterctl.log"), output, 0644)
	Expect(err).NotTo(HaveOccurred(), "failed to write clusterctl log")
}
