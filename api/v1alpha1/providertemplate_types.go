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

// ProviderTemplateSpec defines the desired state of ProviderTemplate
type ProviderTemplateSpec struct {
	TemplateSpecCommon `json:",inline"`
}

// ProviderTemplateStatus defines the observed state of ProviderTemplate
type ProviderTemplateStatus struct {
	TemplateStatusCommon `json:",inline"`
}

func (t *ProviderTemplate) GetSpec() *TemplateSpecCommon {
	return &t.Spec.TemplateSpecCommon
}

func (t *ProviderTemplate) GetStatus() *TemplateStatusCommon {
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
