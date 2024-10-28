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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/Mirantis/hmc/api/v1alpha1"
	"github.com/Mirantis/hmc/internal/utils"
	"github.com/Mirantis/hmc/test/objects/management"
	tm "github.com/Mirantis/hmc/test/objects/templatemanagement"
	"github.com/Mirantis/hmc/test/scheme"
)

func TestTemplateManagementValidateCreate(t *testing.T) {
	g := NewWithT(t)

	ctx := context.Background()

	tests := []struct {
		name            string
		tm              *v1alpha1.TemplateManagement
		existingObjects []runtime.Object
		err             string
		warnings        admission.Warnings
	}{
		{
			name:            "should fail if the TemplateManagement object already exists",
			tm:              tm.NewTemplateManagement(tm.WithName("new")),
			existingObjects: []runtime.Object{tm.NewTemplateManagement(tm.WithName(v1alpha1.TemplateManagementName))},
			err:             "TemplateManagement object already exists",
		},
		{
			name: "should succeed",
			tm:   tm.NewTemplateManagement(tm.WithName("new")),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := fake.NewClientBuilder().
				WithScheme(scheme.Scheme).
				WithRuntimeObjects(tt.existingObjects...).
				WithIndex(&v1alpha1.ManagedCluster{}, v1alpha1.ManagedClusterTemplateIndexKey, v1alpha1.ExtractTemplateNameFromManagedCluster).
				Build()
			validator := &TemplateManagementValidator{Client: c, SystemNamespace: utils.DefaultSystemNamespace}
			warn, err := validator.ValidateCreate(ctx, tt.tm)
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

func TestTemplateManagementValidateDelete(t *testing.T) {
	g := NewWithT(t)

	ctx := context.Background()

	tmName := "test"

	tests := []struct {
		name            string
		tm              *v1alpha1.TemplateManagement
		existingObjects []runtime.Object
		err             string
		warnings        admission.Warnings
	}{
		{
			name:            "should fail if Management object exists and was not deleted",
			tm:              tm.NewTemplateManagement(tm.WithName(tmName)),
			existingObjects: []runtime.Object{management.NewManagement()},
			err:             "TemplateManagement deletion is forbidden",
		},
		{
			name: "should succeed if Management object is not found",
			tm:   tm.NewTemplateManagement(tm.WithName(tmName)),
		},
		{
			name:            "should succeed if Management object was deleted",
			tm:              tm.NewTemplateManagement(tm.WithName(tmName)),
			existingObjects: []runtime.Object{management.NewManagement(management.WithDeletionTimestamp(metav1.Now()))},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := fake.NewClientBuilder().
				WithScheme(scheme.Scheme).
				WithRuntimeObjects(tt.existingObjects...).
				WithIndex(&v1alpha1.ManagedCluster{}, v1alpha1.ManagedClusterTemplateIndexKey, v1alpha1.ExtractTemplateNameFromManagedCluster).
				Build()
			validator := &TemplateManagementValidator{Client: c, SystemNamespace: utils.DefaultSystemNamespace}
			warn, err := validator.ValidateDelete(ctx, tt.tm)
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
