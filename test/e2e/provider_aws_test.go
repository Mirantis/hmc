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
	"context"
	"fmt"
	"os"
	"os/exec"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	internalutils "github.com/Mirantis/hmc/internal/utils"
	"github.com/Mirantis/hmc/test/e2e/kubeclient"
	"github.com/Mirantis/hmc/test/e2e/managedcluster"
	"github.com/Mirantis/hmc/test/e2e/managedcluster/aws"
	"github.com/Mirantis/hmc/test/e2e/managedcluster/clusteridentity"
	"github.com/Mirantis/hmc/test/utils"
)

var _ = Describe("AWS Templates", Label("provider:cloud", "provider:aws"), Ordered, func() {
	var (
		kc                   *kubeclient.KubeClient
		standaloneClient     *kubeclient.KubeClient
		standaloneDeleteFunc func() error
		hostedDeleteFunc     func() error
		kubecfgDeleteFunc    func() error
		clusterName          string
	)

	BeforeAll(func() {
		By("providing cluster identity")
		kc = kubeclient.NewFromLocal(internalutils.DefaultSystemNamespace)
		ci := clusteridentity.New(kc, managedcluster.ProviderAWS)
		Expect(os.Setenv(managedcluster.EnvVarAWSClusterIdentity, ci.IdentityName)).Should(Succeed())
	})

	AfterAll(func() {
		// If we failed collect logs from each of the affiliated controllers
		// as well as the output of clusterctl to store as artifacts.
		if CurrentSpecReport().Failed() && !noCleanup() {
			if standaloneClient != nil {
				By("collecting failure logs from hosted controllers")
				collectLogArtifacts(standaloneClient, clusterName, managedcluster.ProviderAWS, managedcluster.ProviderCAPI)
			}
		}

		By("deleting resources")
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
	})

	It("should work with an AWS provider", func() {
		// Deploy a standalone cluster and verify it is running/ready.
		// Deploy standalone with an xlarge instance since it will also be
		// hosting the hosted cluster.
		GinkgoT().Setenv(managedcluster.EnvVarAWSInstanceType, "t3.xlarge")

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
		// TODO(#472): Ideally we shouldn't use Make here and should just
		// convert these Make targets into Go code, but this will require a
		// helmclient.
		var kubeCfgPath string
		kubeCfgPath, kubecfgDeleteFunc = kc.WriteKubeconfig(context.Background(), clusterName)

		GinkgoT().Setenv("KUBECONFIG", kubeCfgPath)
		cmd := exec.Command("make", "test-apply")
		_, err := utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred())
		Expect(os.Unsetenv("KUBECONFIG")).To(Succeed())

		templateBy(managedcluster.TemplateAWSHostedCP, "validating that the controller is ready")
		standaloneClient = kc.NewFromCluster(context.Background(), internalutils.DefaultSystemNamespace, clusterName)
		Eventually(func() error {
			err := verifyControllersUp(standaloneClient)
			if err != nil {
				_, _ = fmt.Fprintf(
					GinkgoWriter, "[%s] controller validation failed: %v\n",
					string(managedcluster.TemplateAWSHostedCP), err)
				return err
			}
			return nil
		}).WithTimeout(15 * time.Minute).WithPolling(10 * time.Second).Should(Succeed())

		// Ensure AWS credentials are set in the standalone cluster.
		clusteridentity.New(standaloneClient, managedcluster.ProviderAWS)

		// Populate the environment variables required for the hosted
		// cluster.
		aws.PopulateHostedTemplateVars(context.Background(), kc, clusterName)

		templateBy(managedcluster.TemplateAWSHostedCP, "creating a ManagedCluster")
		hd := managedcluster.GetUnstructured(managedcluster.TemplateAWSHostedCP)
		hdName := hd.GetName()

		// Deploy the hosted cluster on top of the standalone cluster.
		hostedDeleteFunc = standaloneClient.CreateManagedCluster(context.Background(), hd)

		templateBy(managedcluster.TemplateAWSHostedCP, "Patching AWSCluster to ready")
		managedcluster.PatchHostedClusterReady(standaloneClient, managedcluster.ProviderAWS, hdName)

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

	It("should work with an EKS provider", func() {
		// Deploy a standalone cluster and verify it is running/ready.
		GinkgoT().Setenv(managedcluster.EnvVarAWSInstanceType, "t3.small")

		cmd := exec.Command("kubectl", "get", "clustertemplates", "-n", "hmc-system", "-o", "yaml")
		output, err := utils.Run(cmd)
		_, _ = fmt.Fprintln(GinkgoWriter, string(output))
		Expect(err).NotTo(HaveOccurred())

		templateBy(managedcluster.TemplateEKSCP, "creating a ManagedCluster for EKS")
		sd := managedcluster.GetUnstructured(managedcluster.TemplateEKSCP)
		clusterName = sd.GetName()

		standaloneDeleteFunc = kc.CreateManagedCluster(context.Background(), sd)

		templateBy(managedcluster.TemplateEKSCP, "waiting for infrastructure to deploy successfully")
		deploymentValidator := managedcluster.NewProviderValidator(
			managedcluster.TemplateEKSCP,
			clusterName,
			managedcluster.ValidationActionDeploy,
		)

		Eventually(func() error {
			return deploymentValidator.Validate(context.Background(), kc)
		}).WithTimeout(60 * time.Minute).WithPolling(30 * time.Second).Should(Succeed())

		// --- clean up ---
		templateBy(managedcluster.TemplateAWSStandaloneCP, "deleting the ManagedCluster for EKS")
		Expect(standaloneDeleteFunc()).NotTo(HaveOccurred())

		deletionValidator := managedcluster.NewProviderValidator(
			managedcluster.TemplateAWSStandaloneCP,
			clusterName,
			managedcluster.ValidationActionDelete,
		)
		Eventually(func() error {
			return deletionValidator.Validate(context.Background(), kc)
		}).WithTimeout(15 * time.Minute).WithPolling(10 *
			time.Second).Should(Succeed())
	})
})
