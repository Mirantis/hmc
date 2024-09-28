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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type CredentialState string

const (
	CredentialReady     CredentialState = "Ready"
	CredentialNotFound  CredentialState = "Cluster Identity not found"
	CredentialWrongType CredentialState = "Mismatched type"
)

// CredentialSpec defines the desired state of Credential
type CredentialSpec struct {
	// Reference to the Credential Identity
	IdentityRef *corev1.ObjectReference `json:"identityRef"`
	// Description of the Credential object
	Description string `json:"description,omitempty"` // WARN: noop
}

// CredentialStatus defines the observed state of Credential
type CredentialStatus struct {
	State CredentialState `json:"state,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=cred
// +kubebuilder:printcolumn:name="State",type=string,JSONPath=`.status.state`
// +kubebuilder:printcolumn:name="Description",type=string,JSONPath=`.spec.description`

// Credential is the Schema for the credentials API
type Credential struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   CredentialSpec   `json:"spec,omitempty"`
	Status CredentialStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// CredentialList contains a list of Credential
type CredentialList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Credential `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Credential{}, &CredentialList{})
}
