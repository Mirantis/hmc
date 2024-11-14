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

package scenarios

import (
	"context"
	"fmt"
	"os"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	internalutils "github.com/Mirantis/hmc/internal/utils"
	"github.com/Mirantis/hmc/test/e2e/config"
	"github.com/Mirantis/hmc/test/e2e/kubeclient"
	"github.com/Mirantis/hmc/test/e2e/managedcluster"
	"github.com/Mirantis/hmc/test/e2e/managedcluster/clusteridentity"
	"github.com/Mirantis/hmc/test/e2e/managedcluster/vsphere"
	"github.com/Mirantis/hmc/test/e2e/templates"
)

var _ = Context("vSphere Templates", Label("provider:onprem", "provider:vsphere"), Ordered, func() {
	ctx := context.Background()

	var (
		kc          *kubeclient.KubeClient
		deleteFunc  func() error
		clusterName string
		err         error

		testingConfig config.ProviderTestingConfig
	)

	BeforeAll(func() {
		By("get testing configuration")
		testingConfig = config.Config[config.TestingProviderVsphere]

		By("set defaults and validate testing configuration")
		err := testingConfig.Standalone.SetDefaults(clusterTemplates, templates.TemplateVSphereStandaloneCP)
		Expect(err).NotTo(HaveOccurred())

		err = testingConfig.Hosted.SetDefaults(clusterTemplates, templates.TemplateVSphereHostedCP)
		Expect(err).NotTo(HaveOccurred())

		_, _ = fmt.Fprintf(GinkgoWriter, "Final Vsphere testing configuration:\n%s\n", testingConfig.String())

		By("ensuring that env vars are set correctly")
		vsphere.CheckEnv()
		By("creating kube client")
		kc = kubeclient.NewFromLocal(internalutils.DefaultSystemNamespace)
		By("providing cluster identity")
		ci := clusteridentity.New(kc, managedcluster.ProviderVSphere, managedcluster.Namespace)
		By("setting VSPHERE_CLUSTER_IDENTITY env variable")
		Expect(os.Setenv(managedcluster.EnvVarVSphereClusterIdentity, ci.IdentityName)).Should(Succeed())
	})

	AfterEach(func() {
		// If we failed collect logs from each of the affiliated controllers
		// as well as the output of clusterctl to store as artifacts.
		if CurrentSpecReport().Failed() {
			By("collecting failure logs from controllers")
			collectLogArtifacts(kc, clusterName, managedcluster.ProviderVSphere, managedcluster.ProviderCAPI)
		}

		// Run the deletion as part of the cleanup and validate it here.
		// VSphere doesn't have any form of cleanup outside of reconciling a
		// cluster deletion so we need to keep the test active while we wait
		// for CAPV to clean up the resources.
		// TODO(#473) Add an exterior cleanup mechanism for VSphere like
		// 'dev-aws-nuke' to clean up resources in the event that the test
		// fails to do so.
		if deleteFunc != nil && !noCleanup() {
			deletionValidator := managedcluster.NewProviderValidator(
				templates.TemplateVSphereStandaloneCP,
				managedcluster.Namespace,
				clusterName,
				managedcluster.ValidationActionDelete,
			)

			err = deleteFunc()
			Expect(err).NotTo(HaveOccurred())
			Eventually(func() error {
				return deletionValidator.Validate(context.Background(), kc)
			}).WithTimeout(10 * time.Minute).WithPolling(10 * time.Second).Should(Succeed())
		}
	})

	It("should deploy standalone managed cluster", func() {
		By(fmt.Sprintf("creating a managed cluster with %s template", testingConfig.Standalone.Template))
		d := managedcluster.GetUnstructured(templates.TemplateVSphereStandaloneCP, testingConfig.Standalone.Template)
		clusterName = d.GetName()

		deleteFunc = kc.CreateManagedCluster(context.Background(), d, managedcluster.Namespace)

		By("waiting for infrastructure providers to deploy successfully")
		deploymentValidator := managedcluster.NewProviderValidator(
			templates.TemplateVSphereStandaloneCP,
			managedcluster.Namespace,
			clusterName,
			managedcluster.ValidationActionDeploy,
		)
		Eventually(func() error {
			return deploymentValidator.Validate(context.Background(), kc)
		}).WithTimeout(30 * time.Minute).WithPolling(10 * time.Second).Should(Succeed())

		if testingConfig.Standalone.Upgrade {
			managedcluster.Upgrade(ctx, kc.CrClient, managedcluster.Namespace, clusterName, testingConfig.Standalone.UpgradeTemplate)
			Eventually(func() error {
				return deploymentValidator.Validate(context.Background(), kc)
			}).WithTimeout(30 * time.Minute).WithPolling(10 * time.Second).Should(Succeed())
		}
	})
})