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
	"github.com/Mirantis/hmc/test/objects/managedcluster"
	"github.com/Mirantis/hmc/test/objects/template"
	"github.com/Mirantis/hmc/test/scheme"
)

func TestClusterTemplateValidateDelete(t *testing.T) {
	g := NewWithT(t)
	ctx := context.Background()
	tpl := template.NewClusterTemplate(template.WithName("testTemplateFail"))
	tplTest := template.NewClusterTemplate(template.WithName("testTemplate"))

	tests := []struct {
		name            string
		template        *v1alpha1.ClusterTemplate
		existingObjects []runtime.Object
		err             string
		warnings        admission.Warnings
	}{
		{
			name:            "should fail if ManagedCluster objects exist",
			template:        tpl,
			existingObjects: []runtime.Object{managedcluster.NewManagedCluster(managedcluster.WithTemplate(tpl.Name))},
			warnings:        admission.Warnings{"The ClusterTemplate object can't be removed if ManagedCluster objects referencing it still exist"},
			err:             "template deletion is forbidden",
		},
		{
			name:            "should be OK because of a different cluster",
			template:        tpl,
			existingObjects: []runtime.Object{managedcluster.NewManagedCluster()},
		},
		{
			name:            "should succeed",
			template:        template.NewClusterTemplate(),
			existingObjects: []runtime.Object{managedcluster.NewManagedCluster(managedcluster.WithTemplate(tplTest.Name))},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := fake.NewClientBuilder().
				WithScheme(scheme.Scheme).
				WithRuntimeObjects(tt.existingObjects...).
				WithIndex(tt.existingObjects[0], TemplateKey, ExtractTemplateName).
				Build()
			validator := &ClusterTemplateValidator{Client: c}
			warn, err := validator.ValidateDelete(ctx, tt.template)
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
