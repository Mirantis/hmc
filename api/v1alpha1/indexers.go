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
	"errors"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func SetupIndexers(ctx context.Context, mgr ctrl.Manager) error {
	var merr error
	for _, f := range []func(context.Context, ctrl.Manager) error{
		setupManagedClusterIndexer,
		setupManagedClusterServicesIndexer,
		setupManagedClusterCredentialIndexer,
		setupReleaseVersionIndexer,
		setupReleaseTemplatesIndexer,
		setupClusterTemplateChainIndexer,
		setupServiceTemplateChainIndexer,
		setupClusterTemplateProvidersIndexer,
		setupMultiClusterServiceServicesIndexer,
		setupOwnerReferenceIndexers,
	} {
		merr = errors.Join(merr, f(ctx, mgr))
	}

	return merr
}

// managed cluster

// ManagedClusterTemplateIndexKey indexer field name to extract ClusterTemplate name reference from a ManagedCluster object.
const ManagedClusterTemplateIndexKey = ".spec.template"

func setupManagedClusterIndexer(ctx context.Context, mgr ctrl.Manager) error {
	return mgr.GetFieldIndexer().IndexField(ctx, &ManagedCluster{}, ManagedClusterTemplateIndexKey, ExtractTemplateNameFromManagedCluster)
}

// ExtractTemplateNameFromManagedCluster returns referenced ClusterTemplate name
// declared in a ManagedCluster object.
func ExtractTemplateNameFromManagedCluster(rawObj client.Object) []string {
	cluster, ok := rawObj.(*ManagedCluster)
	if !ok {
		return nil
	}

	return []string{cluster.Spec.Template}
}

// ManagedClusterServiceTemplatesIndexKey indexer field name to extract service templates names from a ManagedCluster object.
const ManagedClusterServiceTemplatesIndexKey = ".spec.services[].Template"

func setupManagedClusterServicesIndexer(ctx context.Context, mgr ctrl.Manager) error {
	return mgr.GetFieldIndexer().IndexField(ctx, &ManagedCluster{}, ManagedClusterServiceTemplatesIndexKey, ExtractServiceTemplateNamesFromManagedCluster)
}

// ExtractServiceTemplateNamesFromManagedCluster returns a list of service templates names
// declared in a ManagedCluster object.
func ExtractServiceTemplateNamesFromManagedCluster(rawObj client.Object) []string {
	cluster, ok := rawObj.(*ManagedCluster)
	if !ok {
		return nil
	}

	templates := []string{}
	for _, s := range cluster.Spec.ServiceSpec.Services {
		templates = append(templates, s.Template)
	}

	return templates
}

// ManagedClusterCredentialIndexKey indexer field name to extract Credential name reference from a ManagedCluster object.
const ManagedClusterCredentialIndexKey = ".spec.credential"

func setupManagedClusterCredentialIndexer(ctx context.Context, mgr ctrl.Manager) error {
	return mgr.GetFieldIndexer().IndexField(ctx, &ManagedCluster{}, ManagedClusterCredentialIndexKey, ExtractCredentialNameFromManagedCluster)
}

// ExtractCredentialNameFromManagedCluster returns referenced Credential name
// declared in a ManagedCluster object.
func ExtractCredentialNameFromManagedCluster(rawObj client.Object) []string {
	cluster, ok := rawObj.(*ManagedCluster)
	if !ok {
		return nil
	}

	return []string{cluster.Spec.Credential}
}

// release

// ReleaseVersionIndexKey indexer field name to extract release version from a Release object.
const ReleaseVersionIndexKey = ".spec.version"

func setupReleaseVersionIndexer(ctx context.Context, mgr ctrl.Manager) error {
	return mgr.GetFieldIndexer().IndexField(ctx, &Release{}, ReleaseVersionIndexKey, extractReleaseVersion)
}

func extractReleaseVersion(rawObj client.Object) []string {
	release, ok := rawObj.(*Release)
	if !ok {
		return nil
	}
	return []string{release.Spec.Version}
}

// ReleaseTemplatesIndexKey indexer field name to extract component template names from a Release object.
const ReleaseTemplatesIndexKey = "releaseTemplates"

func setupReleaseTemplatesIndexer(ctx context.Context, mgr ctrl.Manager) error {
	return mgr.GetFieldIndexer().IndexField(ctx, &Release{}, ReleaseTemplatesIndexKey, extractReleaseTemplates)
}

func extractReleaseTemplates(rawObj client.Object) []string {
	release, ok := rawObj.(*Release)
	if !ok {
		return nil
	}

	return release.Templates()
}

// template chains

// TemplateChainSupportedTemplatesIndexKey indexer field name to extract supported template names from an according TemplateChain object.
const TemplateChainSupportedTemplatesIndexKey = ".spec.supportedTemplates[].Name"

func setupClusterTemplateChainIndexer(ctx context.Context, mgr ctrl.Manager) error {
	return mgr.GetFieldIndexer().IndexField(ctx, &ClusterTemplateChain{}, TemplateChainSupportedTemplatesIndexKey, extractSupportedTemplatesNames)
}

func setupServiceTemplateChainIndexer(ctx context.Context, mgr ctrl.Manager) error {
	return mgr.GetFieldIndexer().IndexField(ctx, &ServiceTemplateChain{}, TemplateChainSupportedTemplatesIndexKey, extractSupportedTemplatesNames)
}

func extractSupportedTemplatesNames(rawObj client.Object) []string {
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

// cluster template

// ClusterTemplateProvidersIndexKey indexer field name to extract provider names from a ClusterTemplate object.
const ClusterTemplateProvidersIndexKey = "clusterTemplateProviders"

func setupClusterTemplateProvidersIndexer(ctx context.Context, mgr ctrl.Manager) error {
	return mgr.GetFieldIndexer().IndexField(ctx, &ClusterTemplate{}, ClusterTemplateProvidersIndexKey, ExtractProvidersFromClusterTemplate)
}

// ExtractProvidersFromClusterTemplate returns provider names from a ClusterTemplate object.
func ExtractProvidersFromClusterTemplate(o client.Object) []string {
	ct, ok := o.(*ClusterTemplate)
	if !ok {
		return nil
	}

	return ct.Status.Providers
}

// multicluster service

// MultiClusterServiceTemplatesIndexKey indexer field name to extract service templates names from a MultiClusterService object.
const MultiClusterServiceTemplatesIndexKey = "serviceTemplates"

func setupMultiClusterServiceServicesIndexer(ctx context.Context, mgr ctrl.Manager) error {
	return mgr.GetFieldIndexer().IndexField(ctx, &MultiClusterService{}, MultiClusterServiceTemplatesIndexKey, ExtractServiceTemplateNamesFromMultiClusterService)
}

// ExtractServiceTemplateNamesFromMultiClusterService returns a list of service templates names
// declared in a MultiClusterService object.
func ExtractServiceTemplateNamesFromMultiClusterService(rawObj client.Object) []string {
	mcs, ok := rawObj.(*MultiClusterService)
	if !ok {
		return nil
	}

	templates := make([]string, len(mcs.Spec.ServiceSpec.Services))
	for i, s := range mcs.Spec.ServiceSpec.Services {
		templates[i] = s.Template
	}

	return templates
}

// ownerref indexers

// OwnerRefIndexKey indexer field name to extract ownerReference names from objects
const OwnerRefIndexKey = ".metadata.ownerReferences"

func setupOwnerReferenceIndexers(ctx context.Context, mgr ctrl.Manager) error {
	var merr error
	for _, obj := range []client.Object{
		&ProviderTemplate{},
	} {
		merr = errors.Join(merr, mgr.GetFieldIndexer().IndexField(ctx, obj, OwnerRefIndexKey, extractOwnerReferences))
	}

	return merr
}

// extractOwnerReferences returns a list of ownerReference names
func extractOwnerReferences(rawObj client.Object) []string {
	ownerRefs := rawObj.GetOwnerReferences()
	owners := make([]string, 0, len(ownerRefs))
	for _, ref := range ownerRefs {
		owners = append(owners, ref.Name)
	}
	return owners
}
