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

package backup

import (
	"context"
	"fmt"

	certmanagerv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	velerov1api "github.com/zerospiel/velero/pkg/apis/velero/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterapiv1beta1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	hmcv1alpha1 "github.com/Mirantis/hmc/api/v1alpha1"
)

// TODO: label credentials' references, add permissions to the rbac (includeResources won't work as expected)

func (c *Config) getBackupTemplateSpec(ctx context.Context) (*velerov1api.BackupSpec, error) {
	bs := &velerov1api.BackupSpec{
		IncludedNamespaces: []string{"*"},
		OrLabelSelectors: []*metav1.LabelSelector{
			// fixed ones
			selector(hmcv1alpha1.GenericComponentLabelName, hmcv1alpha1.GenericComponentLabelValueHMC),
			selector(certmanagerv1.PartOfCertManagerControllerLabelKey, "true"),
			selector(hmcv1alpha1.FluxHelmChartNameKey, hmcv1alpha1.CoreHMCName),
			selector(clusterapiv1beta1.ProviderNameLabel, "cluster-api"),
		},
	}

	clusterTemplates := new(hmcv1alpha1.ClusterTemplateList)
	if err := c.cl.List(ctx, clusterTemplates); err != nil {
		return nil, fmt.Errorf("failed to list ClusterTemplates: %w", err)
	}

	if len(clusterTemplates.Items) == 0 { // just collect child clusters names
		cldSelectors, err := getClusterDeploymentsSelectors(ctx, c.cl, "")
		if err != nil {
			return nil, fmt.Errorf("failed to get selectors for all clusterdeployments: %w", err)
		}

		bs.OrLabelSelectors = append(bs.OrLabelSelectors, cldSelectors...)

		return bs, nil
	}

	for _, cltpl := range clusterTemplates.Items {
		for _, provider := range cltpl.Status.Providers {
			bs.OrLabelSelectors = append(bs.OrLabelSelectors, selector(clusterapiv1beta1.ProviderNameLabel, provider))
		}

		cldSelectors, err := getClusterDeploymentsSelectors(ctx, c.cl, cltpl.Name)
		if err != nil {
			return nil, fmt.Errorf("failed to get selectors for clusterdeployments referencing %s clustertemplate: %w", client.ObjectKeyFromObject(&cltpl), err)
		}

		bs.OrLabelSelectors = append(bs.OrLabelSelectors, cldSelectors...)
	}

	return bs, nil
}

func getClusterDeploymentsSelectors(ctx context.Context, cl client.Client, clusterTemplateRef string) ([]*metav1.LabelSelector, error) {
	cldeploys := new(hmcv1alpha1.ClusterDeploymentList)
	opts := []client.ListOption{}
	if clusterTemplateRef != "" {
		opts = append(opts, client.MatchingFields{hmcv1alpha1.ClusterDeploymentTemplateIndexKey: clusterTemplateRef})
	}

	if err := cl.List(ctx, cldeploys, opts...); err != nil {
		return nil, fmt.Errorf("failed to list ClusterDeployments: %w", err)
	}

	selectors := make([]*metav1.LabelSelector, len(cldeploys.Items)*2)
	for i, cldeploy := range cldeploys.Items {
		selectors[i] = selector(hmcv1alpha1.FluxHelmChartNameKey, cldeploy.Name)
		selectors[i+1] = selector(clusterapiv1beta1.ProviderNameLabel, cldeploy.Name)
	}

	return selectors, nil
}

func selector(k, v string) *metav1.LabelSelector {
	return &metav1.LabelSelector{
		MatchLabels: map[string]string{k: v},
	}
}
