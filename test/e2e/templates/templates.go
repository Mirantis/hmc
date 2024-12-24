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
	"slices"
	"sort"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	crclient "sigs.k8s.io/controller-runtime/pkg/client"

	hmc "github.com/Mirantis/hmc/api/v1alpha1"
	internalutils "github.com/Mirantis/hmc/internal/utils"
)

type Type string

const (
	TemplateAWSStandaloneCP     Type = "aws-standalone-cp"
	TemplateAWSHostedCP         Type = "aws-hosted-cp"
	TemplateAzureHostedCP       Type = "azure-hosted-cp"
	TemplateAzureStandaloneCP   Type = "azure-standalone-cp"
	TemplateVSphereStandaloneCP Type = "vsphere-standalone-cp"
	TemplateVSphereHostedCP     Type = "vsphere-hosted-cp"
)

func ApplyClusterTemplateAccessRules(ctx context.Context, client crclient.Client, namespace string) map[string][]hmc.AvailableUpgrade {
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
				List: []string{namespace},
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
		clusterTemplateChains, err = checkClusterTemplateChains(ctx, client, namespace, chainNames)
		if err != nil {
			_, _ = fmt.Fprintf(GinkgoWriter, "Not all ClusterTemplateChains were created in the target namespace: %v\n", err)
		}
		return err
	}, 5*time.Minute, 10*time.Second).Should(Succeed())

	clusterTemplates := getClusterTemplates(clusterTemplateChains)
	Eventually(func() error {
		err := checkClusterTemplates(ctx, client, namespace, clusterTemplates)
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
			return fmt.Errorf("failed to get ClusterTemplate %s/%s: %w", namespace, templateName, err)
		}
	}
	return nil
}

func FindTemplate(clusterTemplates map[string][]hmc.AvailableUpgrade, templateType Type) (string, error) {
	templates := filterByType(clusterTemplates, templateType)
	if len(templates) == 0 {
		return "", fmt.Errorf("no Template of the %s type is supported", templateType)
	}
	return templates[0], nil
}

func FindTemplatesToUpgrade(
	clusterTemplates map[string][]hmc.AvailableUpgrade,
	templateType Type,
	sourceTemplate string,
) (template, upgradeTemplate string, err error) {
	templates := filterByType(clusterTemplates, templateType)
	if len(templates) == 0 {
		return "", "", fmt.Errorf("no Template of the %s type is supported", templateType)
	}
	if sourceTemplate != "" {
		// Template should be in the list of supported
		if !slices.Contains(templates, sourceTemplate) {
			return "", "", fmt.Errorf("invalid templates configuration. Template %s is not in the list of supported templates", sourceTemplate)
		}
		// Template should have available upgrades
		availableUpgrades := clusterTemplates[sourceTemplate]
		if len(availableUpgrades) == 0 {
			return "", "", fmt.Errorf("invalid templates configuration. No upgrades are available from the Template %s", sourceTemplate)
		}
		// Find latest available template for the upgrade
		sort.Slice(availableUpgrades, func(i, j int) bool {
			return availableUpgrades[i].Name < availableUpgrades[j].Name
		})
		return sourceTemplate, availableUpgrades[len(availableUpgrades)-1].Name, nil
	}

	// find template with available upgrades
	for _, templateName := range templates {
		template = templateName
		for _, au := range clusterTemplates[template] {
			if upgradeTemplate < au.Name {
				upgradeTemplate = au.Name
			}
		}
		if template != "" && upgradeTemplate != "" {
			return template, upgradeTemplate, nil
		}
	}
	if template == "" || upgradeTemplate == "" {
		return "", "", fmt.Errorf("invalid templates configuration. No %s templates are available for the upgrade", templateType)
	}
	return template, upgradeTemplate, nil
}

func ValidateTemplate(clusterTemplates map[string][]hmc.AvailableUpgrade, template string) error {
	if _, ok := clusterTemplates[template]; ok {
		return nil
	}
	return fmt.Errorf("template %s is not in the list of supported templates", template)
}

func ValidateUpgradeSequence(clusterTemplates map[string][]hmc.AvailableUpgrade, source, target string) error {
	availableUpgrades := clusterTemplates[source]
	if _, ok := clusterTemplates[source]; ok &&
		slices.Contains(availableUpgrades, hmc.AvailableUpgrade{Name: target}) {
		return nil
	}
	return fmt.Errorf("upgrade sequence %s -> %s is not supported", source, target)
}

func filterByType(clusterTemplates map[string][]hmc.AvailableUpgrade, templateType Type) []string {
	var templates []string
	for template := range clusterTemplates {
		if strings.HasPrefix(template, string(templateType)) {
			templates = append(templates, template)
		}
	}
	sort.Slice(templates, func(i, j int) bool {
		return templates[i] > templates[j]
	})
	return templates
}
