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
	"slices"
	"strings"

	helmcontrollerv2 "github.com/fluxcd/helm-controller/api/v2"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
)

const (
	// ChartAnnotationProviderName is the annotation set on components in a Template.
	// This annotations allows to identify all the components belonging to a provider.
	ChartAnnotationProviderName = "cluster.x-k8s.io/provider"

	chartAnnoCAPIPrefix = "cluster.x-k8s.io/"
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

func getProvidersList(providersGetter interface{ GetSpecProviders() Providers }, annotations map[string]string) Providers {
	const multiProviderSeparator = ","

	if spec := providersGetter.GetSpecProviders(); len(spec) > 0 {
		slices.Sort(spec)
		return slices.Compact(spec)
	}

	providers := annotations[ChartAnnotationProviderName]
	if len(providers) == 0 {
		return Providers{}
	}

	var (
		splitted = strings.Split(providers, multiProviderSeparator)
		pstatus  = make([]string, 0, len(splitted))
	)
	for _, v := range splitted {
		if c := strings.TrimSpace(v); c != "" {
			pstatus = append(pstatus, c)
		}
	}

	slices.Sort(pstatus)
	return slices.Compact(pstatus)
}

func getCAPIContracts(contractsGetter interface{ GetContracts() CompatibilityContracts }, annotations map[string]string) (_ CompatibilityContracts, merr error) {
	contractsStatus := make(map[string]string)

	// spec preceding the annos
	if contracts := contractsGetter.GetContracts(); len(contracts) > 0 {
		for capiContract, providerContract := range contracts {
			if !isCAPIContractSingleVersion(capiContract) {
				merr = errors.Join(merr, fmt.Errorf("incorrect CAPI contract version %s in the spec", capiContract))
				continue
			}

			if providerContract != "" && !isCAPIContractVersion(providerContract) { // special case for either CAPI or deliberately set empty
				merr = errors.Join(merr, fmt.Errorf("incorrect provider contract version %s in the spec for the %s CAPI contract version", providerContract, capiContract))
				continue
			}

			contractsStatus[capiContract] = providerContract
		}

		return contractsStatus, merr
	}

	for k, providerContract := range annotations {
		idx := strings.Index(k, chartAnnoCAPIPrefix)
		if idx < 0 {
			continue
		}

		capiContract := k[idx+len(chartAnnoCAPIPrefix):]
		if isCAPIContractSingleVersion(capiContract) {
			if providerContract == "" { // special case for either CAPI or deliberately set empty
				contractsStatus[capiContract] = ""
				continue
			}

			if isCAPIContractVersion(providerContract) {
				contractsStatus[capiContract] = providerContract
			} else {
				// since we parsed capi contract version,
				// then treat the provider's invalid version as an error
				merr = errors.Join(merr, fmt.Errorf("incorrect provider contract version %s given for the %s CAPI contract version annotation", providerContract, k))
			}
		}
	}

	return contractsStatus, merr
}
