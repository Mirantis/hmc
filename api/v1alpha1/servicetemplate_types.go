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
	"fmt"
	"strings"

	"github.com/Masterminds/semver/v3"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// Denotes the servicetemplate resource Kind.
	ServiceTemplateKind = "ServiceTemplate"
	// ChartAnnotationKubernetesConstraint is an annotation containing the Kubernetes constrainted version in the SemVer format associated with a ServiceTemplate.
	ChartAnnotationKubernetesConstraint = "hmc.mirantis.com/k8s-version-constraint"
)

// ServiceTemplateSpec defines the desired state of ServiceTemplate
type ServiceTemplateSpec struct {
	Helm HelmSpec `json:"helm"`
	// Constraint describing compatible K8S versions of the cluster set in the SemVer format.
	KubertenesConstraint string `json:"k8sConstraint,omitempty"`
	// Represents required CAPI providers. Should be set if not present in the Helm chart metadata.
	Providers Providers `json:"providers,omitempty"`
}

// ServiceTemplateStatus defines the observed state of ServiceTemplate
type ServiceTemplateStatus struct {
	// Constraint describing compatible K8S versions of the cluster set in the SemVer format.
	KubertenesConstraint string `json:"k8sConstraint,omitempty"`
	// Represents exposed CAPI providers.
	Providers Providers `json:"providers,omitempty"`

	TemplateStatusCommon `json:",inline"`
}

// FillStatusWithProviders sets the status of the template with providers
// either from the spec or from the given annotations.
func (t *ServiceTemplate) FillStatusWithProviders(annotations map[string]string) error {
	parseProviders := func(typ providersType) []string {
		var (
			pspec []string
			anno  string
		)
		switch typ {
		case bootstrapProvidersType:
			pspec, anno = t.Spec.Providers.BootstrapProviders, ChartAnnotationBootstrapProviders
		case controlPlaneProvidersType:
			pspec, anno = t.Spec.Providers.ControlPlaneProviders, ChartAnnotationControlPlaneProviders
		case infrastructureProvidersType:
			pspec, anno = t.Spec.Providers.InfrastructureProviders, ChartAnnotationInfraProviders
		}

		if len(pspec) > 0 {
			return pspec
		}

		providers := annotations[anno]
		if len(providers) == 0 {
			return []string{}
		}

		splitted := strings.Split(providers, ",")
		result := make([]string, 0, len(splitted))
		for _, v := range splitted {
			if c := strings.TrimSpace(v); c != "" {
				result = append(result, c)
			}
		}

		return result
	}

	t.Status.Providers.BootstrapProviders = parseProviders(bootstrapProvidersType)
	t.Status.Providers.ControlPlaneProviders = parseProviders(controlPlaneProvidersType)
	t.Status.Providers.InfrastructureProviders = parseProviders(infrastructureProvidersType)

	kconstraint := annotations[ChartAnnotationKubernetesConstraint]
	if t.Spec.KubertenesConstraint != "" {
		kconstraint = t.Spec.KubertenesConstraint
	}
	if kconstraint == "" {
		return nil
	}

	if _, err := semver.NewConstraint(kconstraint); err != nil {
		return fmt.Errorf("failed to parse kubernetes constraint %s: %w", kconstraint, err)
	}

	t.Status.KubertenesConstraint = kconstraint

	return nil
}

// GetHelmSpec returns .spec.helm of the Template.
func (t *ServiceTemplate) GetHelmSpec() *HelmSpec {
	return &t.Spec.Helm
}

// GetCommonStatus returns common status of the Template.
func (t *ServiceTemplate) GetCommonStatus() *TemplateStatusCommon {
	return &t.Status.TemplateStatusCommon
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=svctmpl
// +kubebuilder:printcolumn:name="valid",type="boolean",JSONPath=".status.valid",description="Valid",priority=0
// +kubebuilder:printcolumn:name="validationError",type="string",JSONPath=".status.validationError",description="Validation Error",priority=1
// +kubebuilder:printcolumn:name="description",type="string",JSONPath=".status.description",description="Description",priority=1

// ServiceTemplate is the Schema for the servicetemplates API
type ServiceTemplate struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="Spec is immutable"

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

func init() {
	SchemeBuilder.Register(&ServiceTemplate{}, &ServiceTemplateList{})
}
