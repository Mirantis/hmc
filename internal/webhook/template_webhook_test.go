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
	"github.com/Mirantis/hmc/test/objects/clusterdeployment"
	"github.com/Mirantis/hmc/test/objects/multiclusterservice"
	"github.com/Mirantis/hmc/test/objects/template"
	"github.com/Mirantis/hmc/test/scheme"
)

func TestClusterTemplateValidateDelete(t *testing.T) {
	ctx := context.Background()
	namespace := "test"
	tpl := template.NewClusterTemplate(template.WithName("testTemplateFail"), template.WithNamespace(namespace))
	tplTest := template.NewClusterTemplate(template.WithName("testTemplate"), template.WithNamespace(namespace))

	tests := []struct {
		name            string
		template        *v1alpha1.ClusterTemplate
		existingObjects []runtime.Object
		err             string
		warnings        admission.Warnings
	}{
		{
			name:     "should fail if ClusterDeployment objects exist in the same namespace",
			template: tpl,
			existingObjects: []runtime.Object{clusterdeployment.NewClusterDeployment(
				clusterdeployment.WithNamespace(namespace),
				clusterdeployment.WithClusterTemplate(tpl.Name),
			)},
			warnings: admission.Warnings{"The ClusterTemplate object can't be removed if ClusterDeployment objects referencing it still exist"},
			err:      "template deletion is forbidden",
		},
		{
			name:     "should succeed if some ClusterDeployment from another namespace references the template",
			template: tpl,
			existingObjects: []runtime.Object{clusterdeployment.NewClusterDeployment(
				clusterdeployment.WithNamespace("new"),
				clusterdeployment.WithClusterTemplate(tpl.Name),
			)},
		},
		{
			name:            "should be OK because of a different cluster",
			template:        tpl,
			existingObjects: []runtime.Object{clusterdeployment.NewClusterDeployment()},
		},
		{
			name:            "should succeed",
			template:        template.NewClusterTemplate(),
			existingObjects: []runtime.Object{clusterdeployment.NewClusterDeployment(clusterdeployment.WithClusterTemplate(tplTest.Name))},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			c := fake.NewClientBuilder().
				WithScheme(scheme.Scheme).
				WithRuntimeObjects(tt.existingObjects...).
				WithIndex(&v1alpha1.ClusterDeployment{}, v1alpha1.ClusterDeploymentTemplateIndexKey, v1alpha1.ExtractTemplateNameFromClusterDeployment).
				Build()
			validator := &ClusterTemplateValidator{Client: c}
			warn, err := validator.ValidateDelete(ctx, tt.template)
			if tt.err != "" {
				g.Expect(err).To(MatchError(tt.err))
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

func TestServiceTemplateValidateDelete(t *testing.T) {
	ctx := context.Background()
	tmpl := template.NewServiceTemplate(template.WithNamespace("mynamespace"), template.WithName("mytemplate"))

	tests := []struct {
		title           string
		template        *v1alpha1.ServiceTemplate
		existingObjects []runtime.Object
		warnings        admission.Warnings
		err             string
	}{
		{
			title:    "should fail if ClusterDeployment exists in same namespace",
			template: tmpl,
			existingObjects: []runtime.Object{
				clusterdeployment.NewClusterDeployment(
					clusterdeployment.WithNamespace(tmpl.Namespace),
					clusterdeployment.WithServiceTemplate(tmpl.Name),
				),
			},
			warnings: admission.Warnings{"The ServiceTemplate object can't be removed if ClusterDeployment objects referencing it still exist"},
			err:      errTemplateDeletionForbidden.Error(),
		},
		{
			title:    "should succeed if ClusterDeployment referencing ServiceTemplate is another namespace",
			template: tmpl,
			existingObjects: []runtime.Object{
				clusterdeployment.NewClusterDeployment(
					clusterdeployment.WithNamespace("someothernamespace"),
					clusterdeployment.WithServiceTemplate(tmpl.Name),
				),
			},
		},
		{
			title:           "should be OK because of a different cluster",
			template:        tmpl,
			existingObjects: []runtime.Object{clusterdeployment.NewClusterDeployment()},
		},
		{
			title:    "should fail if a MultiClusterService is referencing serviceTemplate in system namespace",
			template: template.NewServiceTemplate(template.WithNamespace(testSystemNamespace), template.WithName(tmpl.Name)),
			existingObjects: []runtime.Object{
				multiclusterservice.NewMultiClusterService(
					multiclusterservice.WithName("mymulticlusterservice"),
					multiclusterservice.WithServiceTemplate(tmpl.Name),
				),
			},
			warnings: admission.Warnings{"The ServiceTemplate object can't be removed if MultiClusterService objects referencing it still exist"},
			err:      errTemplateDeletionForbidden.Error(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.title, func(t *testing.T) {
			g := NewWithT(t)

			c := fake.
				NewClientBuilder().
				WithScheme(scheme.Scheme).
				WithRuntimeObjects(tt.existingObjects...).
				WithIndex(&v1alpha1.ClusterDeployment{}, v1alpha1.ClusterDeploymentServiceTemplatesIndexKey, v1alpha1.ExtractServiceTemplateNamesFromClusterDeployment).
				WithIndex(&v1alpha1.MultiClusterService{}, v1alpha1.MultiClusterServiceTemplatesIndexKey, v1alpha1.ExtractServiceTemplateNamesFromMultiClusterService).
				Build()
			validator := &ServiceTemplateValidator{Client: c, SystemNamespace: testSystemNamespace}
			warn, err := validator.ValidateDelete(ctx, tt.template)
			if tt.err != "" {
				g.Expect(err).To(MatchError(tt.err))
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
