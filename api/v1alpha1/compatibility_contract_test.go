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

import "testing"

func Test_isCAPIContractVersion(t *testing.T) {
	tests := []struct {
		version string
		isValid bool
	}{
		{"v1", true},
		{"v1alpha1", true},
		{"v1beta1", true},
		{"v2", true},
		{"v3alpha2", true},
		{"v33beta22", true},
		{"v1alpha1_v1beta1", true},
		{"v1alpha1v1alha2_v1beta1", false},
		{"v4beta1", true},
		{"invalid", false},
		{"v1alpha", false},
		{"v1beta", false},
		{"v1alpha1beta1", false},
	}

	for _, test := range tests {
		result := isCAPIContractVersion(test.version)
		if result != test.isValid {
			t.Errorf("isValidVersion(%q) = %v, want %v", test.version, result, test.isValid)
		}
	}
}

func Test_isCAPIContractSingleVersion(t *testing.T) {
	tests := []struct {
		version string
		isValid bool
	}{
		{"v1", true},
		{"v1alpha1", true},
		{"v1beta1", true},
		{"v2", true},
		{"v3alpha2", true},
		{"v33beta22", true},
		{"v4beta1", true},
		{"invalid", false},
		{"v1alpha", false},
		{"v1beta", false},
		{"v1alpha1beta1", false},
		{"v1alpha1_v1beta1", false},
	}

	for _, test := range tests {
		result := isCAPIContractSingleVersion(test.version)
		if result != test.isValid {
			t.Errorf("isValidVersion(%q) = %v, want %v", test.version, result, test.isValid)
		}
	}
}
