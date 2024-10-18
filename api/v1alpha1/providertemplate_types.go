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

const (
	// ChartAnnotationCAPIVersion is an annotation containing the CAPI exact version in the SemVer format associated with a ProviderTemplate.
	ChartAnnotationCAPIVersion = "hmc.mirantis.com/capi-version"
	// ChartAnnotationCAPIVersionConstraint is an annotation containing the CAPI version constraint in the SemVer format associated with a ProviderTemplate.
	ChartAnnotationCAPIVersionConstraint = "hmc.mirantis.com/capi-version-constraint"
)

// +kubebuilder:validation:XValidation:rule="!(has(self.capiVersion) && has(self.capiVersionConstraint))", message="Either capiVersion or capiVersionConstraint may be set, but not both"

// ProviderTemplateSpec defines the desired state of ProviderTemplate
type ProviderTemplateSpec struct {
	Helm HelmSpec `json:"helm,omitempty"`
	// CAPI exact version in the SemVer format.
	// Applicable only for the cluster-api ProviderTemplate itself.
	CAPIVersion string `json:"capiVersion,omitempty"`
	// CAPI version constraint in the SemVer format indicating compatibility with the core CAPI.
	// Not applicable for the cluster-api ProviderTemplate.
	CAPIVersionConstraint string `json:"capiVersionConstraint,omitempty"`
	// Providers represent exposed CAPI providers with exact compatibility versions set.
	// Should be set if not present in the Helm chart metadata.
	// Compatibility attributes are optional to be defined.
	Providers ProvidersTupled `json:"providers,omitempty"`
}

// ProviderTemplateStatus defines the observed state of ProviderTemplate
type ProviderTemplateStatus struct {
	// CAPI exact version in the SemVer format.
	// Applicable only for the capi Template itself.
	CAPIVersion string `json:"capiVersion,omitempty"`
	// CAPI version constraint in the SemVer format indicating compatibility with the core CAPI.
	CAPIVersionConstraint string `json:"capiVersionConstraint,omitempty"`
	// Providers represent exposed CAPI providers with exact compatibility versions set
	// if the latter has been given.
	Providers ProvidersTupled `json:"providers,omitempty"`

	TemplateStatusCommon `json:",inline"`
}

// FillStatusWithProviders sets the status of the template with providers
// either from the spec or from the given annotations.
func (t *ProviderTemplate) FillStatusWithProviders(annotations map[string]string) error {
	var err error
	t.Status.Providers, err = parseProviders(t, annotations, semver.NewVersion)
	if err != nil {
		return fmt.Errorf("failed to parse ProviderTemplate providers: %v", err)
	}

	if t.Name == CoreCAPIName {
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
	} else {
		capiConstraint := annotations[ChartAnnotationCAPIVersionConstraint]
		if t.Spec.CAPIVersionConstraint != "" {
			capiConstraint = t.Spec.CAPIVersionConstraint
		}
		if capiConstraint == "" {
			return nil
		}

		if _, err := semver.NewConstraint(capiConstraint); err != nil {
			return fmt.Errorf("failed to parse CAPI version constraint %s: %w", capiConstraint, err)
		}

		t.Status.CAPIVersionConstraint = capiConstraint
	}

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
