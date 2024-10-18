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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// ChartAnnotationCAPIContractVersion is an annotation containing the expected core CAPI contract version (e.g. v1beta1) associated with a ProviderTemplate.
	ChartAnnotationCAPIContractVersion = "hmc.mirantis.com/capi-version"
)

// ProviderTemplateSpec defines the desired state of ProviderTemplate
type ProviderTemplateSpec struct {
	Helm HelmSpec `json:"helm,omitempty"`
	// CAPI [contract version] indicating compatibility with the core CAPI.
	// Currently supported versions: v1alpha3_v1alpha4_v1beta1.
	// The field is not applicable for the cluster-api ProviderTemplate.
	//
	// [contract version]: https://cluster-api.sigs.k8s.io/developer/providers/contracts
	CAPIContractVersion string `json:"capiContractVersion,omitempty"`
	// Providers represent exposed CAPI providers with supported contract versions.
	// Should be set if not present in the Helm chart metadata.
	// Supported contract versions are optional to be defined.
	Providers Providers `json:"providers,omitempty"`
}

// ProviderTemplateStatus defines the observed state of ProviderTemplate
type ProviderTemplateStatus struct {
	// CAPI [contract version] indicating compatibility with the core CAPI.
	// Currently supported versions: v1alpha3_v1alpha4_v1beta1.
	//
	// [contract version]: https://cluster-api.sigs.k8s.io/developer/providers/contracts
	CAPIContractVersion string `json:"capiContractVersion,omitempty"`
	// Providers represent exposed CAPI providers with supported contract versions
	// if the latter has been given.
	Providers Providers `json:"providers,omitempty"`

	TemplateStatusCommon `json:",inline"`
}

// FillStatusWithProviders sets the status of the template with providers
// either from the spec or from the given annotations.
func (t *ProviderTemplate) FillStatusWithProviders(annotations map[string]string) error {
	t.Status.Providers = parseProviders(t, annotations)

	if t.Name == CoreCAPIName {
		return nil
	}

	requiredCAPIContract := annotations[ChartAnnotationCAPIContractVersion]
	if t.Spec.CAPIContractVersion != "" {
		requiredCAPIContract = t.Spec.CAPIContractVersion
	}

	if requiredCAPIContract == "" {
		return nil
	}

	t.Status.CAPIContractVersion = requiredCAPIContract

	return nil
}

// GetSpecProviders returns .spec.providers of the Template.
func (t *ProviderTemplate) GetSpecProviders() Providers {
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
