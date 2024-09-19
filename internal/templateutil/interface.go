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

package templateutil

import (
	"sigs.k8s.io/controller-runtime/pkg/client"

	hmc "github.com/Mirantis/hmc/api/v1alpha1"
)

// Template is the interface defining a list of methods to interact with templates
type Template interface {
	client.Object
	GetSpec() *hmc.TemplateSpecCommon
	GetStatus() *hmc.TemplateStatusCommon
}

func IsManagedByHMC(template Template) bool {
	return template.GetLabels()[hmc.HMCManagedLabelKey] == hmc.HMCManagedLabelValue
}
