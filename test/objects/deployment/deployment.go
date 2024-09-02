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

package deployment

import (
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/Mirantis/hmc/api/v1alpha1"
)

const (
	DefaultName      = "deployment"
	DefaultNamespace = "default"
)

type Opt func(deployment *v1alpha1.Deployment)

func NewDeployment(opts ...Opt) *v1alpha1.Deployment {
	p := &v1alpha1.Deployment{
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
	return func(p *v1alpha1.Deployment) {
		p.Name = name
	}
}

func WithNamespace(namespace string) Opt {
	return func(p *v1alpha1.Deployment) {
		p.Namespace = namespace
	}
}

func WithDryRun(dryRun bool) Opt {
	return func(p *v1alpha1.Deployment) {
		p.Spec.DryRun = dryRun
	}
}

func WithTemplate(templateName string) Opt {
	return func(p *v1alpha1.Deployment) {
		p.Spec.Template = templateName
	}
}

func WithConfig(config string) Opt {
	return func(p *v1alpha1.Deployment) {
		p.Spec.Config = &apiextensionsv1.JSON{
			Raw: []byte(config),
		}
	}
}
