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
	UnmanagedClusterKind      = "UnmanagedCluster"
	UnmanagedClusterFinalizer = "hmc.mirantis.com/unmanaged-cluster"
	AllNodesCondition         = "AllNodesCondition"
	NodeCondition             = "NodeCondition"
	HelmChart                 = "HelmChart"
)

// UnmanagedClusterSpec defines the desired state of UnmanagedCluster
type UnmanagedClusterSpec struct {
	ServicesType `json:",inline"`
}

// UnmanagedClusterStatus defines the observed state of UnmanagedCluster
type UnmanagedClusterStatus struct {
	// Flag indicating whether the unmanaged cluster is in the ready state or not
	// +kubebuilder:default:=false
	Ready bool `json:"ready"`

	// Conditions contains details for the current state of the ManagedCluster.
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:metadata:labels=cluster.x-k8s.io/v1beta1=v1alpha1
// UnmanagedCluster is the Schema for the unmanagedclusters API
type UnmanagedCluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   UnmanagedClusterSpec   `json:"spec,omitempty"`
	Status UnmanagedClusterStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// UnmanagedClusterList contains a list of UnmanagedCluster
type UnmanagedClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []UnmanagedCluster `json:"items"`
}

func init() {
	SchemeBuilder.Register(&UnmanagedCluster{}, &UnmanagedClusterList{})
}

func (in *UnmanagedCluster) GetConditions() *[]metav1.Condition {
	return &in.Status.Conditions
}
