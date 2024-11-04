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

package templates

import (
	"context"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	crclient "sigs.k8s.io/controller-runtime/pkg/client"

	hmc "github.com/Mirantis/hmc/api/v1alpha1"
	internalutils "github.com/Mirantis/hmc/internal/utils"
	"github.com/Mirantis/hmc/test/e2e/managedcluster"
)

func ApplyClusterTemplateAccessRules(ctx context.Context, client crclient.Client) map[string][]hmc.AvailableUpgrade {
	ctChains := &metav1.PartialObjectMetadataList{}
	gvk := hmc.GroupVersion.WithKind(hmc.ClusterTemplateChainKind)
	ctChains.SetGroupVersionKind(gvk)

	err := client.List(ctx, ctChains, crclient.InNamespace(internalutils.DefaultSystemNamespace))
	Expect(err).NotTo(HaveOccurred())
	Expect(ctChains.Items).NotTo(BeEmpty())

	chainNames := make([]string, 0, len(ctChains.Items))
	for _, chain := range ctChains.Items {
		chainNames = append(chainNames, chain.Name)
	}

	tm := &hmc.AccessManagement{
		ObjectMeta: metav1.ObjectMeta{
			Name: hmc.AccessManagementName,
		},
	}
	accessRules := []hmc.AccessRule{
		{
			TargetNamespaces: hmc.TargetNamespaces{
				List: []string{managedcluster.Namespace},
			},
			ClusterTemplateChains: chainNames,
		},
	}

	_, err = ctrl.CreateOrUpdate(ctx, client, tm, func() error {
		tm.Spec.AccessRules = accessRules
		return nil
	})
	Expect(err).NotTo(HaveOccurred())

	clusterTemplateChains := make([]*hmc.ClusterTemplateChain, 0, len(chainNames))
	Eventually(func() error {
		var err error
		clusterTemplateChains, err = checkClusterTemplateChains(ctx, client, managedcluster.Namespace, chainNames)
		if err != nil {
			_, _ = fmt.Fprintf(GinkgoWriter, "Not all ClusterTemplateChains were created in the target namespace: %v\n", err)
		}
		return err
	}, 5*time.Minute, 10*time.Second).Should(Succeed())

	clusterTemplates := getClusterTemplates(clusterTemplateChains)
	Eventually(func() error {
		err := checkClusterTemplates(ctx, client, managedcluster.Namespace, clusterTemplates)
		if err != nil {
			_, _ = fmt.Fprintf(GinkgoWriter, "Not all ClusterTemplates were created in the target namespace: %v\n", err)
		}
		return err
	}, 15*time.Minute, 10*time.Second).Should(Succeed())
	return clusterTemplates
}

func checkClusterTemplateChains(ctx context.Context, client crclient.Client, namespace string, chainNames []string) ([]*hmc.ClusterTemplateChain, error) {
	chains := make([]*hmc.ClusterTemplateChain, 0, len(chainNames))
	for _, chainName := range chainNames {
		chain := &hmc.ClusterTemplateChain{}
		if err := client.Get(ctx, types.NamespacedName{Namespace: namespace, Name: chainName}, chain); err != nil {
			return nil, fmt.Errorf("failed to get ClusterTemplateChain %s/%s: %w", namespace, chainName, err)
		}
		chains = append(chains, chain)
	}
	return chains, nil
}

func getClusterTemplates(chains []*hmc.ClusterTemplateChain) map[string][]hmc.AvailableUpgrade {
	templates := make(map[string][]hmc.AvailableUpgrade)
	for _, chain := range chains {
		for _, supportedTemplate := range chain.Spec.SupportedTemplates {
			templates[supportedTemplate.Name] = append(templates[supportedTemplate.Name], supportedTemplate.AvailableUpgrades...)
		}
	}
	return templates
}

func checkClusterTemplates(ctx context.Context, client crclient.Client, namespace string, clusterTemplates map[string][]hmc.AvailableUpgrade) error {
	for templateName := range clusterTemplates {
		template := &metav1.PartialObjectMetadata{}
		gvk := hmc.GroupVersion.WithKind(hmc.ClusterTemplateKind)
		template.SetGroupVersionKind(gvk)
		if err := client.Get(ctx, types.NamespacedName{Namespace: namespace, Name: templateName}, template); err != nil {
			return fmt.Errorf("failed to get ClusterTemplate %s/%s: %w", namespace, template.Name, err)
		}
	}
	return nil
}
