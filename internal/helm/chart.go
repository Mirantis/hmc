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

package helm

import (
	"fmt"

	sourcev1 "github.com/fluxcd/source-controller/api/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func ArtifactReady(chart *sourcev1.HelmChart) (err error, reportStatus bool) {
	for _, c := range chart.Status.Conditions {
		if c.Type == "Ready" {
			if chart.Generation != c.ObservedGeneration {
				return fmt.Errorf("HelmChart was not reconciled yet, retrying"), false
			}
			if c.Status != metav1.ConditionTrue {
				return fmt.Errorf("failed to download helm chart artifact: %s", c.Message), true
			}
		}
	}
	if chart.Status.Artifact == nil || chart.Status.URL == "" {
		return fmt.Errorf("helm chart artifact is not ready yet"), false
	}
	return nil, false
}
