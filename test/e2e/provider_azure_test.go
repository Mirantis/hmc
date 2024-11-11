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
	"github.com/Mirantis/hmc/test/e2e/config"
	"github.com/Mirantis/hmc/test/e2e/kubeclient"
	"github.com/Mirantis/hmc/test/e2e/logs"
	"github.com/Mirantis/hmc/test/e2e/managedcluster"
	"github.com/Mirantis/hmc/test/e2e/managedcluster/azure"
	"github.com/Mirantis/hmc/test/e2e/managedcluster/clusteridentity"
	"github.com/Mirantis/hmc/test/e2e/templates"
	"github.com/Mirantis/hmc/test/utils"
)

var _ = Context("Azure Templates", Label("provider:cloud", "provider:azure"), Ordered, func() {
	var (
		kc                     *kubeclient.KubeClient
		standaloneClient       *kubeclient.KubeClient
		deleteFuncs            []func() error
		standaloneClusterNames []string
		hostedClusterNames     []string

		providerConfigs []config.ProviderTestingConfig
	)

	BeforeAll(func() {
		By("get testing configuration")
		providerConfigs = config.Config[config.TestingProviderAzure]

		if len(providerConfigs) == 0 {
			Skip("Azure ManagedCluster testing is skipped")
		}

		By("ensuring Azure credentials are set")
		kc = kubeclient.NewFromLocal(internalutils.DefaultSystemNamespace)
		ci := clusteridentity.New(kc, managedcluster.ProviderAzure)
		Expect(os.Setenv(managedcluster.EnvVarAzureClusterIdentity, ci.IdentityName)).Should(Succeed())
	})

	AfterAll(func() {
		// If we failed collect logs from each of the affiliated controllers
		// as well as the output of clusterctl to store as artifacts.
		if CurrentSpecReport().Failed() && !noCleanup() {
			if kc != nil {
				By("collecting failure logs from the management controllers")
				logs.Collector{
					Client:        kc,
					ProviderTypes: []managedcluster.ProviderType{managedcluster.ProviderAzure, managedcluster.ProviderCAPI},
					ClusterNames:  standaloneClusterNames,
				}.CollectAll()
			}
			if standaloneClient != nil {
				By("collecting failure logs from hosted controllers")
				logs.Collector{
					Client:        standaloneClient,
					ProviderTypes: []managedcluster.ProviderType{managedcluster.ProviderAzure, managedcluster.ProviderCAPI},
					ClusterNames:  hostedClusterNames,
				}.CollectAll()
			}
		}
		By("deleting resources")
		for _, deleteFunc := range deleteFuncs {
			if deleteFunc != nil {
				err := deleteFunc()
				Expect(err).NotTo(HaveOccurred())
			}
		}
	})

	for _, providerConfig := range providerConfigs {
		Context(fmt.Sprintf("Testing Azure ManagedCluster deployment with the following configuration: %s", providerConfig.String()), func() {
			It("should work with an Azure provider", func() {
				templateBy(templates.TemplateAzureStandaloneCP, "creating a ManagedCluster")
				sd := managedcluster.GetUnstructured(templates.TemplateAzureStandaloneCP, providerConfig.Standalone)
				sdName := sd.GetName()

				standaloneDeleteFunc := kc.CreateManagedCluster(context.Background(), sd)
				deleteFuncs = append(deleteFuncs, standaloneDeleteFunc)
				standaloneClusterNames = append(standaloneClusterNames, sdName)

				// verify the standalone cluster is deployed correctly
				deploymentValidator := managedcluster.NewProviderValidator(
					templates.TemplateAzureStandaloneCP,
					sdName,
					managedcluster.ValidationActionDeploy,
				)

				templateBy(templates.TemplateAzureStandaloneCP, "waiting for infrastructure provider to deploy successfully")
				Eventually(func() error {
					return deploymentValidator.Validate(context.Background(), kc)
				}).WithTimeout(90 * time.Minute).WithPolling(10 * time.Second).Should(Succeed())

				// setup environment variables for deploying the hosted template (subnet name, etc)
				azure.SetAzureEnvironmentVariables(sdName, kc)

				kubeCfgPath, kubecfgDeleteFunc := kc.WriteKubeconfig(context.Background(), sdName)
				deleteFuncs = append(deleteFuncs, kubecfgDeleteFunc)

				By("Deploy onto standalone cluster")
				GinkgoT().Setenv("KUBECONFIG", kubeCfgPath)
				cmd := exec.Command("make", "test-apply")
				_, err := utils.Run(cmd)
				Expect(err).NotTo(HaveOccurred())
				Expect(os.Unsetenv("KUBECONFIG")).To(Succeed())

				standaloneClient = kc.NewFromCluster(context.Background(), internalutils.DefaultSystemNamespace, sdName)
				// verify the cluster is ready prior to creating credentials
				Eventually(func() error {
					err := verifyControllersUp(standaloneClient)
					if err != nil {
						_, _ = fmt.Fprintf(GinkgoWriter, "Controller validation failed: %v\n", err)
						return err
					}
					return nil
				}).WithTimeout(15 * time.Minute).WithPolling(10 * time.Second).Should(Succeed())

				if providerConfig.Hosted == nil {
					Skip("Azure Hosted cluster deployment is skipped")
				}

				hd := managedcluster.GetUnstructured(templates.TemplateAzureHostedCP, providerConfig.Hosted)
				hdName := hd.GetName()

				By("Create azure credential secret")
				clusteridentity.New(standaloneClient, managedcluster.ProviderAzure)

				By("Create default storage class for azure-disk CSI driver")
				azure.CreateDefaultStorageClass(standaloneClient)

				templateBy(templates.TemplateAzureHostedCP, "creating a ManagedCluster")
				hostedDeleteFunc := standaloneClient.CreateManagedCluster(context.Background(), hd)

				templateBy(templates.TemplateAzureHostedCP, "Patching AzureCluster to ready")
				managedcluster.PatchHostedClusterReady(standaloneClient, managedcluster.ProviderAzure, hdName)

				templateBy(templates.TemplateAzureHostedCP, "waiting for infrastructure to deploy successfully")
				deploymentValidator = managedcluster.NewProviderValidator(
					templates.TemplateAzureHostedCP,
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
					templates.TemplateAzureHostedCP,
					hdName,
					managedcluster.ValidationActionDelete,
				)

				Eventually(func() error {
					return deploymentValidator.Validate(context.Background(), standaloneClient)
				}).WithTimeout(10 * time.Minute).WithPolling(10 * time.Second).Should(Succeed())

				deploymentValidator = managedcluster.NewProviderValidator(
					templates.TemplateAzureStandaloneCP,
					hdName,
					managedcluster.ValidationActionDelete,
				)

				Eventually(func() error {
					return deploymentValidator.Validate(context.Background(), kc)
				}).WithTimeout(10 * time.Minute).WithPolling(10 * time.Second).Should(Succeed())
			})
		})
	}
})
