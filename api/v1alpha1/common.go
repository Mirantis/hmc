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

package v1alpha1

import (
	"context"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// SucceededReason indicates a condition or event observed a success, for example when declared desired state
	// matches actual state, or a performed action succeeded.
	SucceededReason string = "Succeeded"

	// FailedReason indicates a condition or event observed a failure, for example when declared state does not match
	// actual state, or a performed action failed.
	FailedReason string = "Failed"

	// ProgressingReason indicates a condition or event observed progression, for example when the reconciliation of a
	// resource or an action has started.
	ProgressingReason string = "Progressing"
)

type (
	// Holds different types of CAPI providers.
	Providers []string

	// Holds key-value pairs with compatibility [contract versions],
	// where the key is the core CAPI contract version,
	// and the value is an underscore-delimited (_) list of provider contract versions
	// supported by the core CAPI.
	//
	// [contract versions]: https://cluster-api.sigs.k8s.io/developer/providers/contracts
	CompatibilityContracts map[string]string
)

const (
	// Provider CAPA
	ProviderCAPAName = "cluster-api-provider-aws"
	// Provider Azure
	ProviderAzureName   = "cluster-api-provider-azure"
	ProviderVSphereName = "cluster-api-provider-vsphere"
	// Provider K0smotron
	ProviderK0smotronName = "k0smotron"
	// Provider Sveltos
	ProviderSveltosName            = "projectsveltos"
	ProviderSveltosTargetNamespace = "projectsveltos"
	ProviderSveltosCreateNamespace = true
)

func SetupIndexers(ctx context.Context, mgr ctrl.Manager) error {
	if err := SetupManagedClusterIndexer(ctx, mgr); err != nil {
		return err
	}

	if err := SetupReleaseVersionIndexer(ctx, mgr); err != nil {
		return err
	}

	if err := SetupReleaseTemplatesIndexer(ctx, mgr); err != nil {
		return err
	}

	if err := SetupManagedClusterServicesIndexer(ctx, mgr); err != nil {
		return err
	}

	if err := SetupMultiClusterServiceServicesIndexer(ctx, mgr); err != nil {
		return err
	}

	if err := SetupClusterTemplateChainIndexer(ctx, mgr); err != nil {
		return err
	}

	return SetupServiceTemplateChainIndexer(ctx, mgr)
}

const TemplateKey = ".spec.template"

func SetupManagedClusterIndexer(ctx context.Context, mgr ctrl.Manager) error {
	return mgr.GetFieldIndexer().IndexField(ctx, &ManagedCluster{}, TemplateKey, ExtractTemplateName)
}

func ExtractTemplateName(rawObj client.Object) []string {
	cluster, ok := rawObj.(*ManagedCluster)
	if !ok {
		return nil
	}
	return []string{cluster.Spec.Template}
}

const ReleaseVersionKey = ".spec.version"

func SetupReleaseVersionIndexer(ctx context.Context, mgr ctrl.Manager) error {
	return mgr.GetFieldIndexer().IndexField(ctx, &Release{}, ReleaseVersionKey, ExtractReleaseVersion)
}

func ExtractReleaseVersion(rawObj client.Object) []string {
	release, ok := rawObj.(*Release)
	if !ok {
		return nil
	}
	return []string{release.Spec.Version}
}

const ReleaseTemplatesKey = "releaseTemplates"

func SetupReleaseTemplatesIndexer(ctx context.Context, mgr ctrl.Manager) error {
	return mgr.GetFieldIndexer().IndexField(ctx, &Release{}, ReleaseTemplatesKey, ExtractReleaseTemplates)
}

func ExtractReleaseTemplates(rawObj client.Object) []string {
	release, ok := rawObj.(*Release)
	if !ok {
		return nil
	}
	return release.Templates()
}

const ServicesTemplateKey = ".spec.services[].Template"

func SetupManagedClusterServicesIndexer(ctx context.Context, mgr ctrl.Manager) error {
	return mgr.GetFieldIndexer().IndexField(ctx, &ManagedCluster{}, ServicesTemplateKey, ExtractServiceTemplateFromManagedCluster)
}

func ExtractServiceTemplateFromManagedCluster(rawObj client.Object) []string {
	cluster, ok := rawObj.(*ManagedCluster)
	if !ok {
		return nil
	}

	templates := []string{}
	for _, s := range cluster.Spec.Services {
		templates = append(templates, s.Template)
	}

	return templates
}

func SetupMultiClusterServiceServicesIndexer(ctx context.Context, mgr ctrl.Manager) error {
	return mgr.GetFieldIndexer().IndexField(ctx, &MultiClusterService{}, ServicesTemplateKey, ExtractServiceTemplateFromMultiClusterService)
}

func ExtractServiceTemplateFromMultiClusterService(rawObj client.Object) []string {
	cluster, ok := rawObj.(*MultiClusterService)
	if !ok {
		return nil
	}

	templates := []string{}
	for _, s := range cluster.Spec.Services {
		templates = append(templates, s.Template)
	}

	return templates
}

const SupportedTemplateKey = ".spec.supportedTemplates[].Name"

func SetupClusterTemplateChainIndexer(ctx context.Context, mgr ctrl.Manager) error {
	return mgr.GetFieldIndexer().IndexField(ctx, &ClusterTemplateChain{}, SupportedTemplateKey, ExtractSupportedTemplatesNames)
}

func SetupServiceTemplateChainIndexer(ctx context.Context, mgr ctrl.Manager) error {
	return mgr.GetFieldIndexer().IndexField(ctx, &ServiceTemplateChain{}, SupportedTemplateKey, ExtractSupportedTemplatesNames)
}

func ExtractSupportedTemplatesNames(rawObj client.Object) []string {
	chainSpec := TemplateChainSpec{}
	switch chain := rawObj.(type) {
	case *ClusterTemplateChain:
		chainSpec = chain.Spec
	case *ServiceTemplateChain:
		chainSpec = chain.Spec
	default:
		return nil
	}

	supportedTemplates := make([]string, 0, len(chainSpec.SupportedTemplates))
	for _, t := range chainSpec.SupportedTemplates {
		supportedTemplates = append(supportedTemplates, t.Name)
	}

	return supportedTemplates
}
