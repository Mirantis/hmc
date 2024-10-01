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

package templatemanagement

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/Mirantis/hmc/api/v1alpha1"
)

const (
	DefaultName = "hmc-tc"
)

type TemplateChain struct {
	metav1.ObjectMeta `json:",inline"`
	Spec              v1alpha1.TemplateChainSpec `json:"spec"`
}

type Opt func(tc *TemplateChain)

func NewClusterTemplateChain(opts ...Opt) *v1alpha1.ClusterTemplateChain {
	tc := NewTemplateChain(opts...)
	return &v1alpha1.ClusterTemplateChain{
		ObjectMeta: tc.ObjectMeta,
		Spec:       tc.Spec,
	}
}

func NewServiceTemplateChain(opts ...Opt) *v1alpha1.ServiceTemplateChain {
	tc := NewTemplateChain(opts...)
	return &v1alpha1.ServiceTemplateChain{
		ObjectMeta: tc.ObjectMeta,
		Spec:       tc.Spec,
	}
}

func NewTemplateChain(opts ...Opt) *TemplateChain {
	tc := &TemplateChain{
		ObjectMeta: metav1.ObjectMeta{
			Name: DefaultName,
		},
	}
	for _, opt := range opts {
		opt(tc)
	}
	return tc
}

func WithName(name string) Opt {
	return func(tc *TemplateChain) {
		tc.Name = name
	}
}

func WithNamespace(namespace string) Opt {
	return func(tc *TemplateChain) {
		tc.Namespace = namespace
	}
}

func ManagedByHMC() Opt {
	return func(t *TemplateChain) {
		if t.Labels == nil {
			t.Labels = make(map[string]string)
		}
		t.Labels[v1alpha1.HMCManagedLabelKey] = v1alpha1.HMCManagedLabelValue
	}
}

func WithSupportedTemplates(supportedTemplates []v1alpha1.SupportedTemplate) Opt {
	return func(tc *TemplateChain) {
		tc.Spec.SupportedTemplates = supportedTemplates
	}
}
