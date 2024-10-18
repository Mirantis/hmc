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
	// Holds different types of CAPI providers with [compatible contract versions].
	//
	// [compatible contract versions]: https://cluster-api.sigs.k8s.io/developer/providers/contracts
	Providers []NameContract

	// Represents name of the provider with either an exact or constrained version in the SemVer format.
	NameContract struct {
		// Name of the provider.
		Name string `json:"name,omitempty"`
		// Compatibility restriction in the [CAPI provider format]. The value is an underscore-delimited (_) list of versions.
		// Optional to be defined.
		//
		// [CAPI provider format]: https://cluster-api.sigs.k8s.io/developer/providers/contracts#api-version-labels
		ContractVersion string `json:"contractVersion,omitempty"`
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

// Names flattens the underlaying slice to provider names slice and returns it.
func (c Providers) Names() []string {
	nn := make([]string, len(c))
	for i, v := range c {
		nn[i] = v.Name
	}
	return nn
}
