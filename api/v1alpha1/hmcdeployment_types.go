/*
Copyright 2024.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

import (
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// DeploymentSpec defines the desired state of Deployment
type DeploymentSpec struct {
	// DryRun specifies whether the template should be applied after validation or only validated.
	// +kubebuilder:validation:Optional
	DryRun bool `json:"dryRun"`
	// Template is a reference to a Template object located in the same namespace.
	// +kubebuilder:validation:Required
	Template string `json:"template"`
	// Configuration allows to provide parameters for template customization.
	// If no Configuration provided, the field will be populated with the default values for
	// the template and DryRun will be enabled.
	// +kubebuilder:validation:Optional
	Configuration apiextensionsv1.JSON `json:"configuration"`
}

// DeploymentStatus defines the observed state of Deployment
type DeploymentStatus struct {
	TemplateValidationStatus `json:",inline"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// Deployment is the Schema for the deployments API
type Deployment struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   DeploymentSpec   `json:"spec,omitempty"`
	Status DeploymentStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// DeploymentList contains a list of Deployment
type DeploymentList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Deployment `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Deployment{}, &DeploymentList{})
}
