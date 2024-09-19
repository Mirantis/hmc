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

package webhook

import (
	"context"
	"testing"

	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/Mirantis/hmc/api/v1alpha1"
	tc "github.com/Mirantis/hmc/test/objects/templatechain"
	"github.com/Mirantis/hmc/test/scheme"
)

func TestClusterTemplateChainValidateCreate(t *testing.T) {
	g := NewWithT(t)

	ctx := context.Background()

	upgradeFromTemplateName := "template-1-0-1"
	upgradeToTemplateName := "template-1-0-2"
	supportedTemplates := []v1alpha1.SupportedTemplate{
		{
			Name: upgradeFromTemplateName,
			AvailableUpgrades: []v1alpha1.AvailableUpgrade{
				{
					Name: upgradeToTemplateName,
				},
			},
		},
	}

	tests := []struct {
		name            string
		chain           *v1alpha1.ClusterTemplateChain
		existingObjects []runtime.Object
		err             string
		warnings        admission.Warnings
	}{
		{
			name:  "should fail if spec is invalid: incorrect supported templates",
			chain: tc.NewClusterTemplateChain(tc.WithName("test"), tc.WithSupportedTemplates(supportedTemplates)),
			warnings: admission.Warnings{
				"template template-1-0-2 is allowed for upgrade but is not present in the list of spec.SupportedTemplates",
			},
			err: "the template chain spec is invalid",
		},
		{
			name:  "should succeed",
			chain: tc.NewClusterTemplateChain(tc.WithName("test"), tc.WithSupportedTemplates(append(supportedTemplates, v1alpha1.SupportedTemplate{Name: upgradeToTemplateName}))),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := fake.NewClientBuilder().WithScheme(scheme.Scheme).WithRuntimeObjects(tt.existingObjects...).Build()
			validator := &ClusterTemplateChainValidator{Client: c}
			warn, err := validator.ValidateCreate(ctx, tt.chain)
			if tt.err != "" {
				g.Expect(err).To(HaveOccurred())
				if err.Error() != tt.err {
					t.Fatalf("expected error '%s', got error: %s", tt.err, err.Error())
				}
			} else {
				g.Expect(err).To(Succeed())
			}
			if len(tt.warnings) > 0 {
				g.Expect(warn).To(Equal(tt.warnings))
			} else {
				g.Expect(warn).To(BeEmpty())
			}
		})
	}
}
