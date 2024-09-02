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

type Opt func(template *v1alpha1.Template)

func NewTemplate(opts ...Opt) *v1alpha1.Template {
	p := &v1alpha1.Template{
		ObjectMeta: metav1.ObjectMeta{
			Name:      DefaultName,
			Namespace: DefaultNamespace,
		},
	}

	for _, opt := range opts {
		opt(p)
	}
	return p
}

func WithName(name string) Opt {
	return func(p *v1alpha1.Template) {
		p.Name = name
	}
}

func WithNamespace(namespace string) Opt {
	return func(p *v1alpha1.Template) {
		p.Namespace = namespace
	}
}

func WithHelmSpec(helmSpec v1alpha1.HelmSpec) Opt {
	return func(p *v1alpha1.Template) {
		p.Spec.Helm = helmSpec
	}
}

func WithType(templateType v1alpha1.TemplateType) Opt {
	return func(p *v1alpha1.Template) {
		p.Spec.Type = templateType
	}
}

func WithProviders(providers v1alpha1.Providers) Opt {
	return func(p *v1alpha1.Template) {
		p.Spec.Providers = providers
	}
}

func WithTypeStatus(templateType v1alpha1.TemplateType) Opt {
	return func(p *v1alpha1.Template) {
		p.Status.Type = templateType
	}
}

func WithValidationStatus(validationStatus v1alpha1.TemplateValidationStatus) Opt {
	return func(p *v1alpha1.Template) {
		p.Status.TemplateValidationStatus = validationStatus
	}
}

func WithProvidersStatus(providers v1alpha1.Providers) Opt {
	return func(p *v1alpha1.Template) {
		p.Status.Providers = providers
	}
}

func WithConfigStatus(config string) Opt {
	return func(p *v1alpha1.Template) {
		p.Status.Config = &apiextensionsv1.JSON{
			Raw: []byte(config),
		}
	}
}
