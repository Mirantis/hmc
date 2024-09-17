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
	"net/url"
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
	"github.com/Mirantis/hmc/test/managedcluster/aws"
	"github.com/Mirantis/hmc/test/managedcluster/vsphere"
	"github.com/Mirantis/hmc/test/utils"
)

const (
	namespace = "hmc-system"
)

var _ = Describe("controller", Ordered, func() {
	BeforeAll(func() {
		By("building and deploying the controller-manager")
		cmd := exec.Command("make", "dev-apply")
		_, err := utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred())
	})

	AfterAll(func() {
		if !noCleanup() {
			By("removing the controller-manager")
			cmd := exec.Command("make", "dev-destroy")
			_, err := utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred())
		}
	})

	Context("Operator", func() {
		It("should run successfully", func() {
			kc := kubeclient.NewFromLocal(namespace)
			aws.CreateCredentialSecret(context.Background(), kc)

			By("validating that the hmc-controller and capi provider controllers are running")
			Eventually(func() error {
				err := verifyControllersUp(kc)
				if err != nil {
					_, _ = fmt.Fprintf(GinkgoWriter, "Controller validation failed: %v\n", err)
					return err
				}
				return nil
			}).WithTimeout(15 * time.Minute).WithPolling(10 * time.Second).Should(Succeed())
		})
	})

	Describe("AWS Templates", func() {
		var (
			kc                   *kubeclient.KubeClient
			standaloneClient     *kubeclient.KubeClient
			standaloneDeleteFunc func() error
			hostedDeleteFunc     func() error
			kubecfgDeleteFunc    func() error
			clusterName          string
		)

		BeforeAll(func() {
			By("ensuring AWS credentials are set")
			kc = kubeclient.NewFromLocal(namespace)
			aws.CreateCredentialSecret(context.Background(), kc)
		})

		AfterEach(func() {
			// If we failed collect logs from each of the affiliated controllers
			// as well as the output of clusterctl to store as artifacts.
			if CurrentSpecReport().Failed() && !noCleanup() {
				By("collecting failure logs from controllers")
				if kc != nil {
					collectLogArtifacts(kc, clusterName, managedcluster.ProviderAWS, managedcluster.ProviderCAPI)
				}
				if standaloneClient != nil {
					collectLogArtifacts(standaloneClient, clusterName, managedcluster.ProviderAWS, managedcluster.ProviderCAPI)
				}

				By("deleting resources after failure")
				for _, deleteFunc := range []func() error{
					kubecfgDeleteFunc,
					hostedDeleteFunc,
					standaloneDeleteFunc,
				} {
					if deleteFunc != nil {
						err := deleteFunc()
						Expect(err).NotTo(HaveOccurred())
					}
				}
			}
		})

		It("should work with an AWS provider", func() {
			// Deploy a standalone cluster and verify it is running/ready.
			// Deploy standalone with an xlarge instance since it will also be
			// hosting the hosted cluster.
			GinkgoT().Setenv(managedcluster.EnvVarAWSInstanceType, "t3.xlarge")
			GinkgoT().Setenv(managedcluster.EnvVarInstallBeachHeadServices, "false")

			templateBy(managedcluster.TemplateAWSStandaloneCP, "creating a ManagedCluster")
			sd := managedcluster.GetUnstructured(managedcluster.TemplateAWSStandaloneCP)
			clusterName = sd.GetName()

			standaloneDeleteFunc = kc.CreateManagedCluster(context.Background(), sd)

			templateBy(managedcluster.TemplateAWSStandaloneCP, "waiting for infrastructure to deploy successfully")
			deploymentValidator := managedcluster.NewProviderValidator(
				managedcluster.TemplateAWSStandaloneCP,
				clusterName,
				managedcluster.ValidationActionDeploy,
			)

			Eventually(func() error {
				return deploymentValidator.Validate(context.Background(), kc)
			}).WithTimeout(30 * time.Minute).WithPolling(10 * time.Second).Should(Succeed())

			templateBy(managedcluster.TemplateAWSHostedCP, "installing controller and templates on standalone cluster")

			// Download the KUBECONFIG for the standalone cluster and load it
			// so we can call Make targets against this cluster.
			// TODO: Ideally we shouldn't use Make here and should just convert
			// these Make targets into Go code, but this will require a
			// helmclient.
			var kubeCfgPath string
			kubeCfgPath, kubecfgDeleteFunc = kc.WriteKubeconfig(context.Background(), clusterName)

			GinkgoT().Setenv("KUBECONFIG", kubeCfgPath)
			cmd := exec.Command("make", "dev-deploy")
			_, err := utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred())
			cmd = exec.Command("make", "dev-templates")
			_, err = utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred())
			Expect(os.Unsetenv("KUBECONFIG")).To(Succeed())

			// Ensure AWS credentials are set in the standalone cluster.
			standaloneClient = kc.NewFromCluster(context.Background(), namespace, clusterName)
			aws.CreateCredentialSecret(context.Background(), standaloneClient)

			templateBy(managedcluster.TemplateAWSHostedCP, "validating that the controller is ready")
			Eventually(func() error {
				err := verifyControllersUp(standaloneClient, managedcluster.ProviderCAPI, managedcluster.ProviderAWS)
				if err != nil {
					_, _ = fmt.Fprintf(
						GinkgoWriter, "[%s] controller validation failed: %v\n",
						string(managedcluster.TemplateAWSHostedCP), err)
					return err
				}
				return nil
			}).WithTimeout(15 * time.Minute).WithPolling(10 * time.Second).Should(Succeed())

			// Populate the environment variables required for the hosted
			// cluster.
			aws.PopulateHostedTemplateVars(context.Background(), kc)

			templateBy(managedcluster.TemplateAWSHostedCP, "creating a ManagedCluster")
			hd := managedcluster.GetUnstructured(managedcluster.TemplateAWSHostedCP)
			hdName := hd.GetName()

			// Deploy the hosted cluster on top of the standalone cluster.
			hostedDeleteFunc = standaloneClient.CreateManagedCluster(context.Background(), hd)

			// Patch the AWSCluster resource as Ready, see:
			// https://docs.k0smotron.io/stable/capi-aws/#prepare-the-aws-infra-provider
			// Use Eventually as the AWSCluster might not be available
			// immediately.
			templateBy(managedcluster.TemplateAWSHostedCP, "Patching AWSCluster to ready")
			Eventually(func() error {
				if err := aws.PatchAWSClusterReady(context.Background(), standaloneClient, hd.GetName()); err != nil {
					_, _ = fmt.Fprintf(GinkgoWriter, "failed to patch AWSCluster to ready: %v, retrying...\n", err)
					return err
				}
				_, _ = fmt.Fprintf(GinkgoWriter, "Patch succeeded\n")
				return nil
			}).WithTimeout(time.Minute).WithPolling(5 * time.Second).Should(Succeed())

			// Verify the hosted cluster is running/ready.
			templateBy(managedcluster.TemplateAWSHostedCP, "waiting for infrastructure to deploy successfully")
			deploymentValidator = managedcluster.NewProviderValidator(
				managedcluster.TemplateAWSHostedCP,
				hdName,
				managedcluster.ValidationActionDeploy,
			)
			Eventually(func() error {
				return deploymentValidator.Validate(context.Background(), standaloneClient)
			}).WithTimeout(30 * time.Minute).WithPolling(10 * time.Second).Should(Succeed())

			// Delete the hosted ManagedCluster and verify it is removed.
			templateBy(managedcluster.TemplateAWSHostedCP, "deleting the ManagedCluster")
			err = hostedDeleteFunc()
			Expect(err).NotTo(HaveOccurred())

			deletionValidator := managedcluster.NewProviderValidator(
				managedcluster.TemplateAWSHostedCP,
				hdName,
				managedcluster.ValidationActionDelete,
			)
			Eventually(func() error {
				return deletionValidator.Validate(context.Background(), standaloneClient)
			}).WithTimeout(10 * time.Minute).WithPolling(10 * time.Second).Should(Succeed())

			// Now delete the standalone ManagedCluster and verify it is
			// removed, it is deleted last since it is the basis for the hosted
			// cluster.
			/*
				FIXME(#339): This is currently disabled as the deletion of the
				standalone cluster is failing due to outstanding issues.
				templateBy(managedcluster.TemplateAWSStandaloneCP, "deleting the ManagedCluster")
				err = standaloneDeleteFunc()
				Expect(err).NotTo(HaveOccurred())

				deletionValidator = managedcluster.NewProviderValidator(
					managedcluster.TemplateAWSStandaloneCP,
					clusterName,
					managedcluster.ValidationActionDelete,
				)
				Eventually(func() error {
					return deletionValidator.Validate(context.Background(), kc)
				}).WithTimeout(10 * time.Minute).WithPolling(10 *
					time.Second).Should(Succeed())
			*/
		})
	})

	Context("vSphere templates", func() {
		var (
			kc          *kubeclient.KubeClient
			deleteFunc  func() error
			clusterName string
			err         error
		)

		BeforeAll(func() {
			// Set here to skip CI runs for now
			_, testVsphere := os.LookupEnv("TEST_VSPHERE")
			if !testVsphere {
				Skip("Skipping vSphere tests")
			}

			By("ensuring that env vars are set correctly")
			vsphere.CheckEnv()
			By("creating kube client")
			kc := kubeclient.NewFromLocal(namespace)
			By("providing cluster identity")
			credSecretName := "vsphere-cluster-identity-secret-e2e"
			clusterIdentityName := "vsphere-cluster-identity-e2e"
			Expect(vsphere.CreateSecret(kc, credSecretName)).Should(Succeed())
			Expect(vsphere.CreateClusterIdentity(kc, credSecretName, clusterIdentityName)).Should(Succeed())
			By("setting VSPHERE_CLUSTER_IDENTITY env variable")
			Expect(os.Setenv("VSPHERE_CLUSTER_IDENTITY", clusterIdentityName)).Should(Succeed())
		})

		AfterEach(func() {
			// If we failed collect logs from each of the affiliated controllers
			// as well as the output of clusterctl to store as artifacts.
			if CurrentSpecReport().Failed() {
				By("collecting failure logs from controllers")
				collectLogArtifacts(kc, clusterName, managedcluster.ProviderVSphere, managedcluster.ProviderCAPI)
			}

			if deleteFunc != nil {
				By("deleting the deployment")
				err = deleteFunc()
				Expect(err).NotTo(HaveOccurred())
			}

		})

		It("should deploy standalone managed cluster", func() {
			By("creating a managed cluster")
			d := managedcluster.GetUnstructured(managedcluster.TemplateVSphereStandaloneCP)
			clusterName = d.GetName()

			deleteFunc := kc.CreateManagedCluster(context.Background(), d)

			By("waiting for infrastructure providers to deploy successfully")
			deploymentValidator := managedcluster.NewProviderValidator(
				managedcluster.TemplateVSphereStandaloneCP,
				clusterName,
				managedcluster.ValidationActionDeploy,
			)
			Eventually(func() error {
				return deploymentValidator.Validate(context.Background(), kc)
			}).WithTimeout(30 * time.Minute).WithPolling(10 * time.Second).Should(Succeed())

			deletionValidator := managedcluster.NewProviderValidator(
				managedcluster.TemplateVSphereStandaloneCP,
				clusterName,
				managedcluster.ValidationActionDelete,
			)
			By("verify the deployment deletes successfully")
			err = deleteFunc()
			Expect(err).NotTo(HaveOccurred())
			Eventually(func() error {
				return deletionValidator.Validate(context.Background(), kc)
			}).WithTimeout(10 * time.Minute).WithPolling(10 * time.Second).Should(Succeed())
		})
	})

})

// templateBy wraps a Ginkgo By with a block describing the template being
// tested.
func templateBy(t managedcluster.Template, description string) {
	GinkgoHelper()
	By(fmt.Sprintf("[%s] %s", t, description))
}

// collectLogArtfiacts collects log output from each the HMC controller,
// CAPI controller and the provider controller(s) as well as output from clusterctl
// and stores them in the test/e2e directory as artifacts.  If it fails it
// produces a warning message to the GinkgoWriter, but does not fail the test.
func collectLogArtifacts(kc *kubeclient.KubeClient, clusterName string, providerTypes ...managedcluster.ProviderType) {
	GinkgoHelper()

	filterLabels := []string{hmcControllerLabel}

	var host string
	hostURL, err := url.Parse(kc.Config.Host)
	if err != nil {
		utils.WarnError(fmt.Errorf("failed to parse host from kubeconfig: %w", err))
	} else {
		host = strings.ReplaceAll(hostURL.Host, ":", "_")
	}

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

			output, err := os.Create(fmt.Sprintf("./test/e2e/%s.log", host+"-"+pod.Name))
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

	err = os.WriteFile(filepath.Join("test/e2e", host+"-"+"clusterctl.log"), output, 0644)
	if err != nil {
		utils.WarnError(fmt.Errorf("failed to write clusterctl log: %w", err))
	}
}

func noCleanup() bool {
	noCleanup := os.Getenv(managedcluster.EnvVarNoCleanup)
	if noCleanup != "" {
		By(fmt.Sprintf("skipping After nodes as %s is set", managedcluster.EnvVarNoCleanup))
	}

	return noCleanup != ""
}
