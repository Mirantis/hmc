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
	"github.com/Mirantis/hmc/test/e2e/managedcluster/azure"
	"github.com/Mirantis/hmc/test/e2e/managedcluster/clusteridentity"
	"github.com/Mirantis/hmc/test/e2e/templates"
	"github.com/Mirantis/hmc/test/utils"
)

var _ = Context("Azure Templates", Label("provider:cloud", "provider:azure"), Ordered, func() {
	ctx := context.Background()

	var (
		kc                      *kubeclient.KubeClient
		standaloneClient        *kubeclient.KubeClient
		standaloneDeleteFunc    func() error
		hostedDeleteFunc        func() error
		kubecfgDeleteFunc       func() error
		hostedKubecfgDeleteFunc func() error
		sdName                  string
	)

	BeforeAll(func() {
		By("ensuring Azure credentials are set")
		kc = kubeclient.NewFromLocal(internalutils.DefaultSystemNamespace)
		ci := clusteridentity.New(kc, managedcluster.ProviderAzure, managedcluster.Namespace)
		Expect(os.Setenv(managedcluster.EnvVarAzureClusterIdentity, ci.IdentityName)).Should(Succeed())
	})

	AfterEach(func() {
		// If we failed collect logs from each of the affiliated controllers
		// as well as the output of clusterctl to store as artifacts.
		if CurrentSpecReport().Failed() && !noCleanup() {
			By("collecting failure logs from controllers")
			if kc != nil {
				collectLogArtifacts(kc, sdName, managedcluster.ProviderAzure, managedcluster.ProviderCAPI)
			}
			if standaloneClient != nil {
				collectLogArtifacts(standaloneClient, sdName, managedcluster.ProviderAzure, managedcluster.ProviderCAPI)
			}
		}

		By("deleting resources")
		for _, deleteFunc := range []func() error{
			hostedKubecfgDeleteFunc,
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

	It("should work with an Azure provider", func() {
		templateBy(managedcluster.TemplateAzureStandaloneCP, "creating a ManagedCluster")
		sd := managedcluster.GetUnstructured(managedcluster.TemplateAzureStandaloneCP)
		sdName = sd.GetName()

		standaloneDeleteFunc := kc.CreateManagedCluster(context.Background(), sd, managedcluster.Namespace)

		// verify the standalone cluster is deployed correctly
		deploymentValidator := managedcluster.NewProviderValidator(
			managedcluster.TemplateAzureStandaloneCP,
			managedcluster.Namespace,
			sdName,
			managedcluster.ValidationActionDeploy,
		)

		templateBy(managedcluster.TemplateAzureStandaloneCP, "waiting for infrastructure provider to deploy successfully")
		Eventually(func() error {
			return deploymentValidator.Validate(context.Background(), kc)
		}).WithTimeout(90 * time.Minute).WithPolling(10 * time.Second).Should(Succeed())

		// setup environment variables for deploying the hosted template (subnet name, etc)
		azure.SetAzureEnvironmentVariables(sdName, kc)

		hd := managedcluster.GetUnstructured(managedcluster.TemplateAzureHostedCP)
		hdName := hd.GetName()

		var kubeCfgPath string
		kubeCfgPath, kubecfgDeleteFunc = kc.WriteKubeconfig(context.Background(), managedcluster.Namespace, sdName)

		By("Deploy onto standalone cluster")
		GinkgoT().Setenv("KUBECONFIG", kubeCfgPath)
		cmd := exec.Command("make", "test-apply")
		_, err := utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred())
		Expect(os.Unsetenv("KUBECONFIG")).To(Succeed())

		standaloneClient = kc.NewFromCluster(context.Background(), managedcluster.Namespace, sdName)
		// verify the cluster is ready prior to creating credentials
		Eventually(func() error {
			err := verifyControllersUp(standaloneClient)
			if err != nil {
				_, _ = fmt.Fprintf(GinkgoWriter, "Controller validation failed: %v\n", err)
				return err
			}
			return nil
		}).WithTimeout(15 * time.Minute).WithPolling(10 * time.Second).Should(Succeed())

		By(fmt.Sprintf("applying access rules for ClusterTemplates in %s namespace", managedcluster.Namespace))
		templates.ApplyClusterTemplateAccessRules(ctx, standaloneClient.CrClient)

		By("Create azure credential secret")
		clusteridentity.New(standaloneClient, managedcluster.ProviderAzure, managedcluster.Namespace)

		By("Create default storage class for azure-disk CSI driver")
		azure.CreateDefaultStorageClass(standaloneClient)

		templateBy(managedcluster.TemplateAzureHostedCP, "creating a ManagedCluster")
		hostedDeleteFunc = standaloneClient.CreateManagedCluster(context.Background(), hd, managedcluster.Namespace)

		templateBy(managedcluster.TemplateAzureHostedCP, "Patching AzureCluster to ready")
		managedcluster.PatchHostedClusterReady(standaloneClient, managedcluster.ProviderAzure, managedcluster.Namespace, hdName)

		templateBy(managedcluster.TemplateAzureHostedCP, "waiting for infrastructure to deploy successfully")
		deploymentValidator = managedcluster.NewProviderValidator(
			managedcluster.TemplateAzureHostedCP,
			managedcluster.Namespace,
			hdName,
			managedcluster.ValidationActionDeploy,
		)

		Eventually(func() error {
			return deploymentValidator.Validate(context.Background(), standaloneClient)
		}).WithTimeout(90 * time.Minute).WithPolling(10 * time.Second).Should(Succeed())

		By("verify the deployment deletes successfully")
		err = hostedDeleteFunc()
		Expect(err).NotTo(HaveOccurred())

		err = standaloneDeleteFunc()
		Expect(err).NotTo(HaveOccurred())

		deploymentValidator = managedcluster.NewProviderValidator(
			managedcluster.TemplateAzureHostedCP,
			managedcluster.Namespace,
			hdName,
			managedcluster.ValidationActionDelete,
		)

		Eventually(func() error {
			return deploymentValidator.Validate(context.Background(), standaloneClient)
		}).WithTimeout(10 * time.Minute).WithPolling(10 * time.Second).Should(Succeed())

		deploymentValidator = managedcluster.NewProviderValidator(
			managedcluster.TemplateAzureStandaloneCP,
			managedcluster.Namespace,
			hdName,
			managedcluster.ValidationActionDelete,
		)

		Eventually(func() error {
			return deploymentValidator.Validate(context.Background(), kc)
		}).WithTimeout(10 * time.Minute).WithPolling(10 * time.Second).Should(Succeed())
	})
})
