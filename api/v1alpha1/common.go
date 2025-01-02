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

const (
	// SucceededReason indicates a condition or event observed a success, for example when declared desired state
	// matches actual state, or a performed action succeeded.
	SucceededReason string = "Succeeded"

	// FailedReason indicates a condition or event observed a failure, for example when declared state does not match
	// actual state, or a performed action failed.
	FailedReason string = "Failed"

	// ProgressingReason indicates a condition or event observed progression, for example when the reconciliation of a
	// resource or an action has started.
	ProgressingReason string = "Progressing"
)

type (
	// Holds different types of CAPI providers.
	Providers []string

	// Holds key-value pairs with compatibility [contract versions],
	// where the key is the core CAPI contract version,
	// and the value is an underscore-delimited (_) list of provider contract versions
	// supported by the core CAPI.
	//
	// [contract versions]: https://cluster-api.sigs.k8s.io/developer/providers/contracts
	CompatibilityContracts map[string]string
)

const (
	// Provider AWS
	ProviderAWSName = "cluster-api-provider-aws"
	// Provider vSphere
	ProviderVSphereName = "cluster-api-provider-vsphere"
	// Provider OpenStack
	ProviderOpenStackName = "cluster-api-provider-openstack"
	// Provider K0smotron
	ProviderK0smotronName = "k0smotron"
	// Provider Sveltos
	ProviderSveltosName = "projectsveltos"
)
