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
	"encoding/json"
	"testing"

	. "github.com/onsi/gomega"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/Mirantis/hmc/api/v1alpha1"
	"github.com/Mirantis/hmc/test/objects/managedcluster"
	"github.com/Mirantis/hmc/test/objects/management"
	"github.com/Mirantis/hmc/test/scheme"
)

func TestManagementValidateDelete(t *testing.T) {
	g := NewWithT(t)

	ctx := context.Background()

	tests := []struct {
		name            string
		management      *v1alpha1.Management
		existingObjects []runtime.Object
		err             string
		warnings        admission.Warnings
	}{
		{
			name:            "should fail if ManagedCluster objects exist",
			management:      management.NewManagement(),
			existingObjects: []runtime.Object{managedcluster.NewManagedCluster()},
			warnings:        admission.Warnings{"The Management object can't be removed if ManagedCluster objects still exist"},
			err:             "management deletion is forbidden",
		},
		{
			name:       "should succeed",
			management: management.NewManagement(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := fake.NewClientBuilder().WithScheme(scheme.Scheme).WithRuntimeObjects(tt.existingObjects...).Build()
			validator := &ManagementValidator{Client: c}
			warn, err := validator.ValidateDelete(ctx, tt.management)
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

func getManagementObjWithCreateFlag(flag bool) (*v1alpha1.Management, error) {
	hmcConfig := map[string]any{
		"controller": map[string]any{
			"createManagement": flag,
		},
	}

	rawConfig, err := json.Marshal(hmcConfig)
	if err != nil {
		return nil, err
	}

	mgmtObj := &v1alpha1.Management{
		Spec: v1alpha1.ManagementSpec{
			Core: &v1alpha1.Core{
				HMC: v1alpha1.Component{
					Config: &apiextensionsv1.JSON{
						Raw: rawConfig,
					},
				},
			},
		},
	}

	return mgmtObj, nil
}

func TestManagementValidateUpdate(t *testing.T) {
	g := NewWithT(t)

	ctx := context.Background()

	objToFail, err := getManagementObjWithCreateFlag(true)
	g.Expect(err).To(Succeed())

	objToSucceed, err := getManagementObjWithCreateFlag(false)
	g.Expect(err).To(Succeed())

	tests := []struct {
		name       string
		management *v1alpha1.Management
		err        string
	}{
		{
			name:       "should fail if Management object has controller.createManagement=true",
			management: objToFail,
			err:        "reenabling of the createManagement parameter is forbidden",
		},
		{
			name:       "should succeed for controller.createManagement=false",
			management: objToSucceed,
		},
		{
			name:       "should succeed",
			management: management.NewManagement(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := fake.NewClientBuilder().WithScheme(scheme.Scheme).Build()
			validator := &ManagementValidator{Client: c}
			_, err := validator.ValidateUpdate(ctx, management.NewManagement(), tt.management)
			if tt.err != "" {
				g.Expect(err).To(HaveOccurred())
				if err.Error() != tt.err {
					t.Fatalf("expected error '%s', got error: %s", tt.err, err.Error())
				}
			} else {
				g.Expect(err).To(Succeed())
			}
		})
	}
}
