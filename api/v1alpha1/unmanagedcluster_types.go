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

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

const (
	UnmanagedClusterKind      = "UnmanagedCluster"
	UnmanagedClusterFinalizer = "hmc.mirantis.com/unmanage-dcluster"
	AllNodesCondition         = "AllNodesCondition"
	NodeCondition             = "NodeCondition"
	HelmChart                 = "HelmChart"
)

// UnmanagedClusterSpec defines the desired state of UnmanagedCluster
type UnmanagedClusterSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	Name string `json:"name,omitempty"`
	// Services is a list of services created via ServiceTemplates
	// that could be installed on the target cluster.
	Services []ServiceSpec `json:"services,omitempty"`

	// ServicesPriority sets the priority for the services defined in this spec.
	// Higher value means higher priority and lower means lower.
	// In case of conflict with another object managing the service,
	// the one with higher priority will get to deploy its services.
	ServicesPriority int32 `json:"servicesPriority,omitempty"`
	// DryRun specifies whether the template should be applied after validation or only validated.
	// DryRun bool `json:"dryRun,omitempty"`

	// +kubebuilder:default:=false

	// StopOnConflict specifies what to do in case of a conflict.
	// E.g. If another object is already managing a service.
	// By default the remaining services will be deployed even if conflict is detected.
	// If set to true, the deployment will stop after encountering the first conflict.
	StopOnConflict bool `json:"stopOnConflict,omitempty"`
}

// UnmanagedClusterStatus defines the observed state of UnmanagedCluster
type UnmanagedClusterStatus struct {
	// Flag indicating whether the unmanaged cluster is in the ready state or not
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
