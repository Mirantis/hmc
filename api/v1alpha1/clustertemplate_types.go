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

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=clustertmpl
// +kubebuilder:printcolumn:name="valid",type="boolean",JSONPath=".status.valid",description="Valid",priority=0
// +kubebuilder:printcolumn:name="validationError",type="string",JSONPath=".status.validationError",description="Validation Error",priority=1
// +kubebuilder:printcolumn:name="description",type="string",JSONPath=".status.description",description="Description",priority=1

// ClusterTemplate is the Schema for the cluster templates API
type ClusterTemplate struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="Spec is immutable"
	Spec   ClusterTemplateSpec   `json:"spec,omitempty"`
	Status ClusterTemplateStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ClusterTemplateList contains a list of ClusterTemplate
type ClusterTemplateList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ClusterTemplate `json:"items"`
}

// ClusterTemplateSpec defines the desired state of ClusterTemplate
type ClusterTemplateSpec struct {
	TemplateSpecMixin `json:",inline"`
}

// ClusterTemplateStatus defines the observed state of ClusterTemplate
type ClusterTemplateStatus struct {
	TemplateStatusMixin `json:",inline"`
}

func (t *ClusterTemplate) GetSpec() *TemplateSpecMixin {
	return &t.Spec.TemplateSpecMixin
}

func (t *ClusterTemplate) GetStatus() *TemplateStatusMixin {
	return &t.Status.TemplateStatusMixin
}
