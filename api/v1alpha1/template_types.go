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
	helmcontrollerv2 "github.com/fluxcd/helm-controller/api/v2"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
)

const (
	// ChartAnnotationType is an annotation containing the type of Template.
	ChartAnnotationType = "hmc.mirantis.com/type"
	// ChartAnnotationInfraProviders is an annotation containing the CAPI infrastructure providers associated with Template.
	ChartAnnotationInfraProviders = "hmc.mirantis.com/infrastructure-providers"
	// ChartAnnotationBootstrapProviders is an annotation containing the CAPI bootstrap providers associated with Template.
	ChartAnnotationBootstrapProviders = "hmc.mirantis.com/bootstrap-providers"
	// ChartAnnotationControlPlaneProviders is an annotation containing the CAPI control plane providers associated with Template.
	ChartAnnotationControlPlaneProviders = "hmc.mirantis.com/control-plane-providers"
)

// TemplateType specifies the type of template packaged as a helm chart.
// Should be provided in the chart Annotations.
type TemplateType string

const (
	// TemplateTypeDeployment is the type used for creating HMC ManagedCluster objects
	TemplateTypeDeployment TemplateType = "deployment"
	// TemplateTypeProvider is the type used for adding CAPI providers in the HMC Management object.
	TemplateTypeProvider TemplateType = "provider"
	// TemplateTypeCore is the type used for HMC and CAPI core components
	TemplateTypeCore TemplateType = "core"
)

// TemplateSpecMixin is a Template configuration common for all Template types
type TemplateSpecMixin struct {
	// Helm holds a reference to a Helm chart representing the HMC template
	// +kubebuilder:validation:Required
	Helm HelmSpec `json:"helm"`
	// Type specifies the type of the provided template.
	// Should be set if not present in the Helm chart metadata.
	// +kubebuilder:validation:Enum=deployment;provider;core
	Type TemplateType `json:"type,omitempty"`
	// Providers represent required/exposed CAPI providers depending on the template type.
	// Should be set if not present in the Helm chart metadata.
	Providers Providers `json:"providers,omitempty"`
}

// +kubebuilder:validation:XValidation:rule="(has(self.chartName) && !has(self.chartRef)) || (!has(self.chartName) && has(self.chartRef))", message="either chartName or chartRef must be set"

// HelmSpec references a Helm chart representing the HMC template
type HelmSpec struct {
	// ChartName is a name of a Helm chart representing the template in the HMC repository.
	// +optional
	ChartName string `json:"chartName,omitempty"`
	// ChartVersion is a version of a Helm chart representing the template in the HMC repository.
	// +optional
	ChartVersion string `json:"chartVersion,omitempty"`
	// ChartRef is a reference to a source controller resource containing the
	// Helm chart representing the template.
	// +optional
	ChartRef *helmcontrollerv2.CrossNamespaceSourceReference `json:"chartRef,omitempty"`
}

// TemplateStatusMixin defines the observed state of Template common for all Template types
type TemplateStatusMixin struct {
	TemplateValidationStatus `json:",inline"`
	// Description contains information about the template.
	// +optional
	Description string `json:"description,omitempty"`
	// Config demonstrates available parameters for template customization,
	// that can be used when creating ManagedCluster objects.
	// +optional
	Config *apiextensionsv1.JSON `json:"config,omitempty"`
	// ChartRef is a reference to a source controller resource containing the
	// Helm chart representing the template.
	// +optional
	ChartRef *helmcontrollerv2.CrossNamespaceSourceReference `json:"chartRef,omitempty"`
	// Type specifies the type of the provided template, as discovered from the Helm chart metadata.
	// +kubebuilder:validation:Enum=deployment;provider;core
	Type TemplateType `json:"type,omitempty"`
	// Providers represent required/exposed CAPI providers depending on the template type.
	Providers Providers `json:"providers,omitempty"`
	// ObservedGeneration is the last observed generation.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
}

type TemplateValidationStatus struct {
	// Valid indicates whether the template passed validation or not.
	Valid bool `json:"valid"`
	// ValidationError provides information regarding issues encountered during template validation.
	// +optional
	ValidationError string `json:"validationError,omitempty"`
}

func init() {
	SchemeBuilder.Register(&ClusterTemplate{}, &ClusterTemplateList{})
	SchemeBuilder.Register(&ServiceTemplate{}, &ServiceTemplateList{})
	SchemeBuilder.Register(&ProviderTemplate{}, &ProviderTemplateList{})
}
