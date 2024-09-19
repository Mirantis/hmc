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
	"fmt"
	"testing"

	. "github.com/onsi/gomega"
	admissionv1 "k8s.io/api/admission/v1"
	authenticationv1 "k8s.io/api/authentication/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/Mirantis/hmc/api/v1alpha1"
	"github.com/Mirantis/hmc/test/objects/managedcluster"
	"github.com/Mirantis/hmc/test/objects/template"
	tm "github.com/Mirantis/hmc/test/objects/templatemanagement"
	"github.com/Mirantis/hmc/test/scheme"
)

const (
	namespace             = "test-ns"
	systemNamespace       = "hmc"
	tmName                = "test-tm"
	hmcServiceAccountName = "hmc-controller-manager"
)

func TestClusterTemplateValidateDelete(t *testing.T) {
	ctx := context.Background()

	tpl := template.NewClusterTemplate(template.WithName("testTemplateFail"), template.WithNamespace(namespace))

	tests := []struct {
		name            string
		template        *v1alpha1.ClusterTemplate
		existingObjects []runtime.Object
		userInfo        authenticationv1.UserInfo
		err             string
		warnings        admission.Warnings
	}{
		{
			name:            "should fail if the template is managed by HMC and the user triggered the deletion",
			template:        template.NewClusterTemplate(template.ManagedByHMC()),
			existingObjects: []runtime.Object{tm.NewTemplateManagement(tm.WithName(tmName))},
			err:             "template deletion is forbidden",
		},
		{
			name:            "should fail if the template is in the system namespace",
			template:        template.NewClusterTemplate(template.WithNamespace(systemNamespace)),
			existingObjects: []runtime.Object{},
			err:             "template deletion is forbidden",
		},
		{
			name:     "should fail if ManagedCluster object referencing the template exists in the same namespace",
			template: tpl,
			existingObjects: []runtime.Object{managedcluster.NewManagedCluster(
				managedcluster.WithNamespace(namespace),
				managedcluster.WithClusterTemplate(tpl.Name),
			)},
			warnings: admission.Warnings{"The ClusterTemplate object can't be removed if ManagedCluster objects referencing it still exist"},
			err:      "template deletion is forbidden",
		},
		{
			name:     "should succeed if some ManagedCluster from another namespace references the template",
			template: tpl,
			existingObjects: []runtime.Object{managedcluster.NewManagedCluster(
				managedcluster.WithNamespace("new"),
				managedcluster.WithClusterTemplate(tpl.Name),
			)},
		},
		{
			name:            "should succeed if the template is managed by HMC and the controller triggered the deletion",
			template:        template.NewClusterTemplate(template.ManagedByHMC()),
			userInfo:        authenticationv1.UserInfo{Username: fmt.Sprintf("system:serviceaccount:hmc-system:%s", hmcServiceAccountName)},
			existingObjects: []runtime.Object{tm.NewTemplateManagement(tm.WithName(tmName))},
		},
		{
			name:            "should succeed if the template is not managed by HMC",
			template:        tpl,
			existingObjects: []runtime.Object{tm.NewTemplateManagement(tm.WithName(tmName))},
		},
		{
			name:            "should succeed because no cluster references the template",
			template:        tpl,
			existingObjects: []runtime.Object{managedcluster.NewManagedCluster()},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			c := fake.NewClientBuilder().
				WithScheme(scheme.Scheme).
				WithRuntimeObjects(tt.existingObjects...).
				WithIndex(&v1alpha1.ManagedCluster{}, v1alpha1.TemplateKey, v1alpha1.ExtractTemplateName).
				Build()
			validator := &ClusterTemplateValidator{
				TemplateValidator: TemplateValidator{
					Client:          c,
					SystemNamespace: systemNamespace,
				},
			}

			t.Setenv(ServiceAccountEnvName, hmcServiceAccountName)

			req := admission.Request{
				AdmissionRequest: admissionv1.AdmissionRequest{
					UserInfo: tt.userInfo,
				},
			}
			warn, err := validator.ValidateDelete(admission.NewContextWithRequest(ctx, req), tt.template)
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
		userInfo        authenticationv1.UserInfo
		warnings        admission.Warnings
		err             string
	}{
		{
			title:           "should fail if the template is managed by HMC and the user triggered the deletion",
			template:        template.NewServiceTemplate(template.ManagedByHMC()),
			existingObjects: []runtime.Object{tm.NewTemplateManagement(tm.WithName(tmName))},
			err:             "template deletion is forbidden",
		},
		{
			title:           "should fail if the template is in the system namespace",
			template:        template.NewServiceTemplate(template.WithNamespace(systemNamespace)),
			existingObjects: []runtime.Object{},
			err:             "template deletion is forbidden",
		},
		{
			title:    "should fail if ManagedCluster exists in same namespace",
			template: tmpl,
			existingObjects: []runtime.Object{
				managedcluster.NewManagedCluster(
					managedcluster.WithNamespace(tmpl.Namespace),
					managedcluster.WithServiceTemplate(tmpl.Name),
				),
			},
			warnings: admission.Warnings{"The ServiceTemplate object can't be removed if ManagedCluster objects referencing it still exist"},
			err:      errTemplateDeletionForbidden.Error(),
		},
		{
			title:    "should succeed if managedCluster referencing ServiceTemplate is another namespace",
			template: tmpl,
			existingObjects: []runtime.Object{
				managedcluster.NewManagedCluster(
					managedcluster.WithNamespace("someothernamespace"),
					managedcluster.WithServiceTemplate(tmpl.Name),
				),
			},
		},
		{
			title:           "should succeed if the template is managed by HMC and the controller triggered the deletion",
			template:        template.NewServiceTemplate(template.ManagedByHMC()),
			userInfo:        authenticationv1.UserInfo{Username: fmt.Sprintf("system:serviceaccount:hmc-system:%s", hmcServiceAccountName)},
			existingObjects: []runtime.Object{tm.NewTemplateManagement(tm.WithName(tmName))},
		},
		{
			title:           "should succeed if the template is not managed by HMC",
			template:        tmpl,
			existingObjects: []runtime.Object{tm.NewTemplateManagement(tm.WithName(tmName))},
		},
		{
			title:           "should succeed because no cluster references the template",
			template:        tmpl,
			existingObjects: []runtime.Object{managedcluster.NewManagedCluster()},
		},
	}

	for _, tt := range tests {
		t.Run(tt.title, func(t *testing.T) {
			g := NewWithT(t)

			c := fake.
				NewClientBuilder().
				WithScheme(scheme.Scheme).
				WithRuntimeObjects(tt.existingObjects...).
				WithIndex(&v1alpha1.ManagedCluster{}, v1alpha1.ServicesTemplateKey, v1alpha1.ExtractServiceTemplateName).
				Build()

			validator := &ServiceTemplateValidator{
				TemplateValidator{
					Client:          c,
					SystemNamespace: systemNamespace,
				},
			}

			t.Setenv(ServiceAccountEnvName, hmcServiceAccountName)

			req := admission.Request{
				AdmissionRequest: admissionv1.AdmissionRequest{
					UserInfo: tt.userInfo,
				},
			}
			warn, err := validator.ValidateDelete(admission.NewContextWithRequest(ctx, req), tt.template)
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
