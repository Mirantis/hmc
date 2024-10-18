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
	"errors"
	"fmt"
	"strings"

	helmcontrollerv2 "github.com/fluxcd/helm-controller/api/v2"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
)

const (
	// ChartAnnotationProviderName is the annotation set on components in a Template.
	// This annotations allows to identify all the components belonging to a provider.
	ChartAnnotationProviderName = "cluster.x-k8s.io/provider"
)

// +kubebuilder:validation:XValidation:rule="(has(self.chartName) && !has(self.chartRef)) || (!has(self.chartName) && has(self.chartRef))", message="either chartName or chartRef must be set"

// HelmSpec references a Helm chart representing the HMC template
type HelmSpec struct {
	// ChartRef is a reference to a source controller resource containing the
	// Helm chart representing the template.
	ChartRef *helmcontrollerv2.CrossNamespaceSourceReference `json:"chartRef,omitempty"`
	// ChartName is a name of a Helm chart representing the template in the HMC repository.
	ChartName string `json:"chartName,omitempty"`
	// ChartVersion is a version of a Helm chart representing the template in the HMC repository.
	ChartVersion string `json:"chartVersion,omitempty"`
}

func (s *HelmSpec) String() string {
	if s.ChartRef != nil {
		return s.ChartRef.Namespace + "/" + s.ChartRef.Name + ", Kind=" + s.ChartRef.Kind
	}

	return s.ChartName + ": " + s.ChartVersion
}

// TemplateStatusCommon defines the observed state of Template common for all Template types
type TemplateStatusCommon struct {
	// Config demonstrates available parameters for template customization,
	// that can be used when creating ManagedCluster objects.
	Config *apiextensionsv1.JSON `json:"config,omitempty"`
	// ChartRef is a reference to a source controller resource containing the
	// Helm chart representing the template.
	ChartRef *helmcontrollerv2.CrossNamespaceSourceReference `json:"chartRef,omitempty"`
	// Description contains information about the template.
	Description string `json:"description,omitempty"`

	TemplateValidationStatus `json:",inline"`

	// ObservedGeneration is the last observed generation.
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
}

type TemplateValidationStatus struct {
	// ValidationError provides information regarding issues encountered during template validation.
	ValidationError string `json:"validationError,omitempty"`
	// Valid indicates whether the template passed validation or not.
	Valid bool `json:"valid"`
}

// TODO (zerospiel): change to comma as part of the [Contracts support].
//
// [Contracts support]: https://github.com/Mirantis/hmc/issues/496
const multiProviderSeparator = ";"

// TODO (zerospiel): move to the template-ctrl?
func parseProviders[T any](providersGetter interface{ GetSpecProviders() ProvidersTupled }, annotations map[string]string, validationFn func(string) (T, error)) ([]ProviderTuple, error) {
	providers := annotations[ChartAnnotationProviderName]
	if len(providers) == 0 {
		return providersGetter.GetSpecProviders(), nil
	}

	var (
		ps = providersGetter.GetSpecProviders()

		splitted = strings.Split(providers, multiProviderSeparator)
		pstatus  = make([]ProviderTuple, 0, len(splitted)+len(ps))
		merr     error
	)
	pstatus = append(pstatus, ps...)

	for _, v := range splitted {
		v = strings.TrimSpace(v)
		nVerOrC := strings.SplitN(v, " ", 2)
		if len(nVerOrC) == 0 { // BCE (bound check elimination)
			continue
		}

		n := ProviderTuple{Name: nVerOrC[0]}
		if len(nVerOrC) < 2 {
			pstatus = append(pstatus, n)
			continue
		}

		ver := strings.TrimSpace(nVerOrC[1])
		if _, err := validationFn(ver); err != nil { // validation
			merr = errors.Join(merr, fmt.Errorf("failed to parse %s in the %s: %v", ver, v, err))
			continue
		}

		n.VersionOrConstraint = ver
		pstatus = append(pstatus, n)
	}

	return pstatus, merr
}
