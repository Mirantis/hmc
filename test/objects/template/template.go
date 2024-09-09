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

package template

import (
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/Mirantis/hmc/api/v1alpha1"
)

const (
	DefaultName      = "template"
	DefaultNamespace = "hmc-system"
)

type Template struct {
	metav1.ObjectMeta `json:",inline"`
	Spec              v1alpha1.TemplateSpecMixin   `json:"spec"`
	Status            v1alpha1.TemplateStatusMixin `json:"status"`
}

type Opt func(template *Template)

func NewClusterTemplate(opts ...Opt) *v1alpha1.ClusterTemplate {
	templateState := NewTemplate(opts...)
	return &v1alpha1.ClusterTemplate{
		ObjectMeta: templateState.ObjectMeta,
		Spec:       v1alpha1.ClusterTemplateSpec{TemplateSpecMixin: templateState.Spec},
		Status:     v1alpha1.ClusterTemplateStatus{TemplateStatusMixin: templateState.Status},
	}
}

func NewServiceTemplate(opts ...Opt) *v1alpha1.ServiceTemplate {
	templateState := NewTemplate(opts...)
	return &v1alpha1.ServiceTemplate{
		ObjectMeta: templateState.ObjectMeta,
		Spec:       v1alpha1.ServiceTemplateSpec{TemplateSpecMixin: templateState.Spec},
		Status:     v1alpha1.ServiceTemplateStatus{TemplateStatusMixin: templateState.Status},
	}
}

func NewProviderTemplate(opts ...Opt) *v1alpha1.ProviderTemplate {
	templateState := NewTemplate(opts...)
	return &v1alpha1.ProviderTemplate{
		ObjectMeta: templateState.ObjectMeta,
		Spec:       v1alpha1.ProviderTemplateSpec{TemplateSpecMixin: templateState.Spec},
		Status:     v1alpha1.ProviderTemplateStatus{TemplateStatusMixin: templateState.Status},
	}
}

func NewTemplate(opts ...Opt) *Template {
	template := &Template{
		ObjectMeta: metav1.ObjectMeta{
			Name:      DefaultName,
			Namespace: DefaultNamespace,
		},
	}
	for _, opt := range opts {
		opt(template)
	}
	return template
}

func WithName(name string) Opt {
	return func(t *Template) {
		t.Name = name
	}
}

func WithNamespace(namespace string) Opt {
	return func(t *Template) {
		t.Namespace = namespace
	}
}

func WithHelmSpec(helmSpec v1alpha1.HelmSpec) Opt {
	return func(t *Template) {
		t.Spec.Helm = helmSpec
	}
}

func WithProviders(providers v1alpha1.Providers) Opt {
	return func(t *Template) {
		t.Spec.Providers = providers
	}
}

func WithTypeStatus(templateType v1alpha1.TemplateType) Opt {
	return func(t *Template) {
		t.Status.Type = templateType
	}
}

func WithValidationStatus(validationStatus v1alpha1.TemplateValidationStatus) Opt {
	return func(t *Template) {
		t.Status.TemplateValidationStatus = validationStatus
	}
}

func WithProvidersStatus(providers v1alpha1.Providers) Opt {
	return func(t *Template) {
		t.Status.Providers = providers
	}
}

func WithConfigStatus(config string) Opt {
	return func(t *Template) {
		t.Status.Config = &apiextensionsv1.JSON{
			Raw: []byte(config),
		}
	}
}
