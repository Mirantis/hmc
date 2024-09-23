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

// Providers is a structure holding different types of CAPI providers
type Providers struct {
	// InfrastructureProviders is the list of CAPI infrastructure providers
	InfrastructureProviders []string `json:"infrastructure,omitempty"`
	// BootstrapProviders is the list of CAPI bootstrap providers
	BootstrapProviders []string `json:"bootstrap,omitempty"`
	// ControlPlaneProviders is the list of CAPI control plane providers
	ControlPlaneProviders []string `json:"controlPlane,omitempty"`
}

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

	return nil
}

const TemplateKey = ".spec.template"

func SetupManagedClusterIndexer(ctx context.Context, mgr ctrl.Manager) error {
	if err := mgr.GetFieldIndexer().
		IndexField(ctx, &ManagedCluster{}, TemplateKey, ExtractTemplateName); err != nil {
		return err
	}

	return nil
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
	if err := mgr.GetFieldIndexer().
		IndexField(ctx, &Release{}, VersionKey, ExtractReleaseVersion); err != nil {
		return err
	}

	return nil
}

func ExtractReleaseVersion(rawObj client.Object) []string {
	release, ok := rawObj.(*Release)
	if !ok {
		return nil
	}
	return []string{release.Spec.Version}
}
