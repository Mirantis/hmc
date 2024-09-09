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
// +kubebuilder:resource:shortName=svctmpl
// +kubebuilder:printcolumn:name="type",type="string",JSONPath=".status.type",description="Type",priority=0
// +kubebuilder:printcolumn:name="valid",type="boolean",JSONPath=".status.valid",description="Valid",priority=0
// +kubebuilder:printcolumn:name="validationError",type="string",JSONPath=".status.validationError",description="Validation Error",priority=1
// +kubebuilder:printcolumn:name="description",type="string",JSONPath=".status.description",description="Description",priority=1

// ServiceTemplate is the Schema for the service templates API
type ServiceTemplate struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ServiceTemplateSpec   `json:"spec,omitempty"`
	Status ServiceTemplateStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ServiceTemplateList contains a list of ServiceTemplate
type ServiceTemplateList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ServiceTemplate `json:"items"`
}

// ServiceTemplateSpec defines the desired state of ServiceTemplate
type ServiceTemplateSpec struct {
	TemplateSpecMixin `json:",inline"`
}

// ServiceTemplateStatus defines the observed state of ServiceTemplate
type ServiceTemplateStatus struct {
	TemplateStatusMixin `json:",inline"`
}

func (t *ServiceTemplate) GetSpec() *TemplateSpecMixin {
	return &t.Spec.TemplateSpecMixin
}

func (t *ServiceTemplate) GetStatus() *TemplateStatusMixin {
	return &t.Status.TemplateStatusMixin
}
