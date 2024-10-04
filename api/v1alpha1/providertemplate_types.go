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
	"fmt"

	"github.com/Masterminds/semver/v3"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ChartAnnotationCAPIVersion is an annotation containing the CAPI exact version in the SemVer format associated with a ProviderTemplate.
const ChartAnnotationCAPIVersion = "hmc.mirantis.com/capi-version"

// ProviderTemplateSpec defines the desired state of ProviderTemplate
type ProviderTemplateSpec struct {
	Helm HelmSpec `json:"helm"`
	// Compatible CAPI provider version set in the SemVer format.
	CAPIVersion string `json:"capiVersion,omitempty"`
	// Represents required CAPI providers with exact compatibility versions set. Should be set if not present in the Helm chart metadata.
	Providers ProvidersTupled `json:"providers,omitempty"`
}

// ProviderTemplateStatus defines the observed state of ProviderTemplate
type ProviderTemplateStatus struct {
	// Compatible CAPI provider version in the SemVer format.
	CAPIVersion string `json:"capiVersion,omitempty"`
	// Providers represent exposed CAPI providers with exact compatibility versions set.
	Providers ProvidersTupled `json:"providers,omitempty"`

	TemplateStatusCommon `json:",inline"`
}

// FillStatusWithProviders sets the status of the template with providers
// either from the spec or from the given annotations.
func (t *ProviderTemplate) FillStatusWithProviders(annotations map[string]string) error {
	var err error
	t.Status.Providers.BootstrapProviders, err = parseProviders(t, bootstrapProvidersType, annotations, semver.NewVersion)
	if err != nil {
		return fmt.Errorf("failed to parse ProviderTemplate bootstrap providers: %v", err)
	}

	t.Status.Providers.ControlPlaneProviders, err = parseProviders(t, controlPlaneProvidersType, annotations, semver.NewVersion)
	if err != nil {
		return fmt.Errorf("failed to parse ProviderTemplate controlPlane providers: %v", err)
	}

	t.Status.Providers.InfrastructureProviders, err = parseProviders(t, infrastructureProvidersType, annotations, semver.NewVersion)
	if err != nil {
		return fmt.Errorf("failed to parse ProviderTemplate infrastructure providers: %v", err)
	}

	capiVersion := annotations[ChartAnnotationCAPIVersion]
	if t.Spec.CAPIVersion != "" {
		capiVersion = t.Spec.CAPIVersion
	}
	if capiVersion == "" {
		return nil
	}

	if _, err := semver.NewVersion(capiVersion); err != nil {
		return fmt.Errorf("failed to parse CAPI version %s: %w", capiVersion, err)
	}

	t.Status.CAPIVersion = capiVersion

	return nil
}

// GetSpecProviders returns .spec.providers of the Template.
func (t *ProviderTemplate) GetSpecProviders() ProvidersTupled {
	return t.Spec.Providers
}

// GetHelmSpec returns .spec.helm of the Template.
func (t *ProviderTemplate) GetHelmSpec() *HelmSpec {
	return &t.Spec.Helm
}

// GetCommonStatus returns common status of the Template.
func (t *ProviderTemplate) GetCommonStatus() *TemplateStatusCommon {
	return &t.Status.TemplateStatusCommon
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=providertmpl,scope=Cluster
// +kubebuilder:printcolumn:name="valid",type="boolean",JSONPath=".status.valid",description="Valid",priority=0
// +kubebuilder:printcolumn:name="validationError",type="string",JSONPath=".status.validationError",description="Validation Error",priority=1
// +kubebuilder:printcolumn:name="description",type="string",JSONPath=".status.description",description="Description",priority=1

// ProviderTemplate is the Schema for the providertemplates API
type ProviderTemplate struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="Spec is immutable"

	Spec   ProviderTemplateSpec   `json:"spec,omitempty"`
	Status ProviderTemplateStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ProviderTemplateList contains a list of ProviderTemplate
type ProviderTemplateList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ProviderTemplate `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ProviderTemplate{}, &ProviderTemplateList{})
}
