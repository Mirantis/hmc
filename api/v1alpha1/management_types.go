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
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/yaml"
)

const (
	CoreHMCName = "hmc"

	CoreCAPIName = "capi"

	ManagementKind      = "Management"
	ManagementName      = "hmc"
	ManagementFinalizer = "hmc.mirantis.com/management"
)

// ManagementSpec defines the desired state of Management
type ManagementSpec struct {
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=253

	// Release references the Release object.
	Release string `json:"release"`
	// Core holds the core Management components that are mandatory.
	// If not specified, will be populated with the default values.
	Core *Core `json:"core,omitempty"`

	// Providers is the list of supported CAPI providers.
	Providers []Provider `json:"providers,omitempty"`

	Backup ManagementBackup `json:"backup,omitempty"`
}

// Core represents a structure describing core Management components.
type Core struct {
	// HMC represents the core HMC component and references the HMC template.
	HMC Component `json:"hmc,omitempty"`
	// CAPI represents the core Cluster API component and references the Cluster API template.
	CAPI Component `json:"capi,omitempty"`
}

// ManagementBackup enables a feature to backup HMC objects into a cloud.
type ManagementBackup struct {
	// +kubebuilder:default="0 */6 * * *"

	// Schedule is a Cron expression defining when to run the scheduled Backup.
	// Default value is to backup every 6 hours.
	Schedule string `json:"schedule,omitempty"`

	// +kubebuilder:default=false

	// Flag to indicate whether the backup feature is enabled.
	// If set to true, [Velero] platform will be installed.
	// If set to false, creation or modification of Backups/Restores will be blocked.
	//
	// [Velero]: https://velero.io
	Enabled bool `json:"enabled"`
}

// Component represents HMC management component
type Component struct {
	// Config allows to provide parameters for management component customization.
	// If no Config provided, the field will be populated with the default
	// values for the template.
	Config *apiextensionsv1.JSON `json:"config,omitempty"`
	// Template is the name of the Template associated with this component.
	// If not specified, will be taken from the Release object.
	Template string `json:"template,omitempty"`
}

type Provider struct {
	Component `json:",inline"`
	// Name of the provider.
	Name string `json:"name"`
}

func (p Provider) String() string {
	return p.Name
}

func (in *Component) HelmValues() (values map[string]any, err error) {
	if in.Config != nil {
		err = yaml.Unmarshal(in.Config.Raw, &values)
	}
	return values, err
}

// Templates returns a list of provider templates explicitly defined in the Management object
func (in *Management) Templates() []string {
	templates := []string{}
	if in.Spec.Core != nil {
		if in.Spec.Core.CAPI.Template != "" {
			templates = append(templates, in.Spec.Core.CAPI.Template)
		}
		if in.Spec.Core.HMC.Template != "" {
			templates = append(templates, in.Spec.Core.HMC.Template)
		}
	}
	for _, p := range in.Spec.Providers {
		if p.Template != "" {
			templates = append(templates, p.Template)
		}
	}
	return templates
}

// ManagementStatus defines the observed state of Management
type ManagementStatus struct {
	// For each CAPI provider name holds its compatibility [contract versions]
	// in a key-value pairs, where the key is the core CAPI contract version,
	// and the value is an underscore-delimited (_) list of provider contract versions
	// supported by the core CAPI.
	//
	// [contract versions]: https://cluster-api.sigs.k8s.io/developer/providers/contracts
	CAPIContracts map[string]CompatibilityContracts `json:"capiContracts,omitempty"`
	// Components indicates the status of installed HMC components and CAPI providers.
	Components map[string]ComponentStatus `json:"components,omitempty"`
	// BackupName is a name of the management cluster scheduled backup.
	BackupName string `json:"backupName,omitempty"`
	// Release indicates the current Release object.
	Release string `json:"release,omitempty"`
	// AvailableProviders holds all available CAPI providers.
	AvailableProviders Providers `json:"availableProviders,omitempty"`
	// ObservedGeneration is the last observed generation.
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
}

// ComponentStatus is the status of Management component installation
type ComponentStatus struct {
	// Template is the name of the Template associated with this component.
	Template string `json:"template,omitempty"`
	// Error stores as error message in case of failed installation
	Error string `json:"error,omitempty"`
	// Success represents if a component installation was successful
	Success bool `json:"success,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=hmc-mgmt;mgmt,scope=Cluster

// Management is the Schema for the managements API
type Management struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ManagementSpec   `json:"spec,omitempty"`
	Status ManagementStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ManagementList contains a list of Management
type ManagementList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Management `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Management{}, &ManagementList{})
}
