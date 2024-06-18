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
	"k8s.io/apimachinery/pkg/util/yaml"
)

const (
	ManagementName      = "hmc"
	ManagementNamespace = "hmc-system"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// ManagementSpec defines the desired state of Management
type ManagementSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Components is the list of supported management components
	Components []Component `json:"components,omitempty"`
}

// Component represents HMC management component
type Component struct {
	// Template is the name of the Template associated with this component
	Template string `json:"template"`
	// Config allows to provide parameters for management component customization.
	// If no Config provided, the field will be populated with the default
	// values for the template.
	// +optional
	Config *apiextensionsv1.JSON `json:"config,omitempty"`
}

func (in *Component) HelmValues() (values map[string]interface{}, err error) {
	if in.Config != nil {
		err = yaml.Unmarshal(in.Config.Raw, &values)
	}
	return values, err
}

func (m ManagementSpec) SetDefaults() {
	// TODO: Uncomment when Templates will be ready
	/*
		m.Components = []Component{
			{
				Template: "cluster-api",
			},
			{
				Template: "k0smotron",
			},
			{
				Template: "cluster-api-provider-aws",
			},
		}
	*/
}

// ManagementStatus defines the observed state of Management
type ManagementStatus struct {
	// ObservedGeneration is the last observed generation.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
	// Providers is the list of discovered supported providers
	Providers ProvidersStatus `json:"providers,omitempty"`
	// Components contains the map with the status of Management components installation
	Components map[string]ComponentStatus `json:"components,omitempty"`
}

// ComponentStatus is the status of Management component installation
type ComponentStatus struct {
	// Success represents if a component installation was successful
	Success bool `json:"success,omitempty"`
	// Error stores as error message in case of failed installation
	Error string `json:"error,omitempty"`
}

// ProvidersStatus is the list of discovered supported providers
type ProvidersStatus struct {
	// InfrastructureProviders is the list of discovered infrastructure providers
	InfrastructureProviders []string `json:"infrastructure,omitempty"`
	// BootstrapProviders is the list of discovered bootstrap providers
	BootstrapProviders []string `json:"bootstrap,omitempty"`
	// ControlPlaneProviders is the list of discovered control plane providers
	ControlPlaneProviders []string `json:"controlPlane,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
// +kubebuilder:resource:shortName=hmc-mgmt;mgmt

// Management is the Schema for the managements API
type Management struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ManagementSpec   `json:"spec,omitempty"`
	Status ManagementStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// ManagementList contains a list of Management
type ManagementList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Management `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Management{}, &ManagementList{})
}
