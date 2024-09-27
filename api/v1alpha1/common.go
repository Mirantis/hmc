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

type (
	// Providers hold different types of CAPI providers.
	Providers struct {
		// InfrastructureProviders is the list of CAPI infrastructure providers
		InfrastructureProviders []string `json:"infrastructure,omitempty"`
		// BootstrapProviders is the list of CAPI bootstrap providers
		BootstrapProviders []string `json:"bootstrap,omitempty"`
		// ControlPlaneProviders is the list of CAPI control plane providers
		ControlPlaneProviders []string `json:"controlPlane,omitempty"`
	}

	// Holds different types of CAPI providers with either
	// an exact or constrainted version in the SemVer format. The requirement
	// is determined by a consumer this type.
	ProvidersTupled struct {
		// List of CAPI infrastructure providers with either an exact or constrainted version in the SemVer format.
		InfrastructureProviders []ProviderTuple `json:"infrastructure,omitempty"`
		// List of CAPI bootstrap providers with either an exact or constrainted version in the SemVer format.
		BootstrapProviders []ProviderTuple `json:"bootstrap,omitempty"`
		// List of CAPI control plane providers with either an exact or constrainted version in the SemVer format.
		ControlPlaneProviders []ProviderTuple `json:"controlPlane,omitempty"`
	}

	// Represents name of the provider with either an exact or constrainted version in the SemVer format.
	ProviderTuple struct {
		// Name of the provider.
		Name string `json:"name,omitempty"`
		// Compatibility restriction in the SemVer format (exact or constrainted version)
		VersionOrContraint string `json:"versionOrContraint,omitempty"`
	}
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

	if err := SetupReleaseIndexer(ctx, mgr); err != nil {
		return err
	}

	return SetupManagedClusterServicesIndexer(ctx, mgr)
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

const VersionKey = ".spec.version"

func SetupReleaseIndexer(ctx context.Context, mgr ctrl.Manager) error {
	return mgr.GetFieldIndexer().IndexField(ctx, &Release{}, VersionKey, ExtractReleaseVersion)
}

func ExtractReleaseVersion(rawObj client.Object) []string {
	release, ok := rawObj.(*Release)
	if !ok {
		return nil
	}
	return []string{release.Spec.Version}
}

const ServicesTemplateKey = ".spec.services[].Template"

func SetupManagedClusterServicesIndexer(ctx context.Context, mgr ctrl.Manager) error {
	return mgr.GetFieldIndexer().IndexField(ctx, &ManagedCluster{}, ServicesTemplateKey, ExtractServiceTemplateName)
}

func ExtractServiceTemplateName(rawObj client.Object) []string {
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

func (c ProvidersTupled) BootstrapProvidersNames() []string {
	return c.names(bootstrapProvidersType)
}

func (c ProvidersTupled) ControlPlaneProvidersNames() []string {
	return c.names(bootstrapProvidersType)
}

func (c ProvidersTupled) InfrastructureProvidersNames() []string {
	return c.names(bootstrapProvidersType)
}

func (c ProvidersTupled) names(typ providersType) []string {
	f := func(nn []ProviderTuple) []string {
		res := make([]string, len(nn))
		for i, v := range nn {
			res[i] = v.Name
		}
		return res
	}

	switch typ {
	case bootstrapProvidersType:
		return f(c.BootstrapProviders)
	case controlPlaneProvidersType:
		return f(c.ControlPlaneProviders)
	case infrastructureProvidersType:
		return f(c.InfrastructureProviders)
	default:
		return []string{}
	}
}
