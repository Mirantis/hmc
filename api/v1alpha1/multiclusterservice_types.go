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
	sveltosv1beta1 "github.com/projectsveltos/addon-controller/api/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// MultiClusterServiceFinalizer is finalizer applied to MultiClusterService objects.
	MultiClusterServiceFinalizer = "hmc.mirantis.com/multicluster-service"
	// MultiClusterServiceKind is the string representation of a MultiClusterServiceKind.
	MultiClusterServiceKind = "MultiClusterService"

	// SveltosProfileReadyCondition indicates if the Sveltos Profile is ready.
	SveltosProfileReadyCondition = "SveltosProfileReady"
	// SveltosClusterProfileReadyCondition indicates if the Sveltos ClusterProfile is ready.
	SveltosClusterProfileReadyCondition = "SveltosClusterProfileReady"
	// SveltosHelmReleaseReadyCondition indicates if the HelmRelease
	// managed by a Sveltos Profile/ClusterProfile is ready.
	SveltosHelmReleaseReadyCondition = "SveltosHelmReleaseReady"

	// FetchServicesStatusSuccessCondition indicates if status
	// for the deployed services have been fetched successfully.
	FetchServicesStatusSuccessCondition = "FetchServicesStatusSuccess"
)

// Service represents a Service to be deployed.
type Service struct {
	// Values is the helm values to be passed to the chart used by the template.
	// The string type is used in order to allow for templating.
	Values string `json:"values,omitempty"`

	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=253

	// Template is a reference to a Template object located in the same namespace.
	Template string `json:"template"`

	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=253

	// Name is the chart release.
	Name string `json:"name"`
	// Namespace is the namespace the release will be installed in.
	// It will default to Name if not provided.
	Namespace string `json:"namespace,omitempty"`
	// ValuesFrom can reference a ConfigMap or Secret containing helm values.
	ValuesFrom []sveltosv1beta1.ValueFrom `json:"valuesFrom,omitempty"`
	// Disable can be set to disable handling of this service.
	Disable bool `json:"disable,omitempty"`
}

// ServiceSpec contains all the spec related to deployment of services.
type ServiceSpec struct {
	// Services is a list of services created via ServiceTemplates
	// that could be installed on the target cluster.
	Services []Service `json:"services,omitempty"`
	// TemplateResourceRefs is a list of resources to collect from the management cluster,
	// the values from which can be used in templates.
	TemplateResourceRefs []sveltosv1beta1.TemplateResourceRef `json:"templateResourceRefs,omitempty"`
	// PolicyRefs is a list of kubernetes resources that need to be deployed in matching clusters.
	// These resources may contain static values or leverage Go templates for dynamic customization.
	// When expressed as templates, the values are rendered using data from resources
	// (like Cluster & TemplateResourceRefs) within the management cluster before deployement.
	PolicyRefs []sveltosv1beta1.PolicyRef `json:"policyRefs,omitempty"`

	// +kubebuilder:default:=100
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=2147483646

	// Priority sets the priority for the services defined in this spec.
	// Higher value means higher priority and lower means lower.
	// In case of conflict with another object managing the service,
	// the one with higher priority will get to deploy its services.
	Priority int32 `json:"priority,omitempty"`

	// +kubebuilder:default:=false

	// StopOnConflict specifies what to do in case of a conflict.
	// E.g. If another object is already managing a service.
	// By default the remaining services will be deployed even if conflict is detected.
	// If set to true, the deployment will stop after encountering the first conflict.
	StopOnConflict bool `json:"stopOnConflict,omitempty"`
	// Reload instances via rolling upgrade when a ConfigMap/Secret mounted as volume is modified.
	Reload bool `json:"reload,omitempty"`
}

// MultiClusterServiceSpec defines the desired state of MultiClusterService
type MultiClusterServiceSpec struct {
	// ClusterSelector identifies target clusters to manage services on.
	ClusterSelector metav1.LabelSelector `json:"clusterSelector,omitempty"`
	// ServiceSpec is spec related to deployment of services.
	ServiceSpec ServiceSpec `json:"serviceSpec,omitempty"`
}

// ServiceStatus contains details for the state of services.
type ServiceStatus struct {
	// ClusterName is the name of the associated cluster.
	ClusterName string `json:"clusterName"`
	// ClusterNamespace is the namespace of the associated cluster.
	ClusterNamespace string `json:"clusterNamespace,omitempty"`
	// Conditions contains details for the current state of managed services.
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// MultiClusterServiceStatus defines the observed state of MultiClusterService.
type MultiClusterServiceStatus struct {
	// Services contains details for the state of services.
	Services []ServiceStatus `json:"services,omitempty"`
	// Conditions contains details for the current state of the MultiClusterService.
	Conditions []metav1.Condition `json:"conditions,omitempty"`
	// ObservedGeneration is the last observed generation.
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster

// MultiClusterService is the Schema for the multiclusterservices API
type MultiClusterService struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   MultiClusterServiceSpec   `json:"spec,omitempty"`
	Status MultiClusterServiceStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// MultiClusterServiceList contains a list of MultiClusterService
type MultiClusterServiceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []MultiClusterService `json:"items"`
}

func init() {
	SchemeBuilder.Register(&MultiClusterService{}, &MultiClusterServiceList{})
}
