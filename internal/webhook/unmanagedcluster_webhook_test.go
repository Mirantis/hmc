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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/secret"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/Mirantis/hmc/api/v1alpha1"
	uc "github.com/Mirantis/hmc/test/objects/unmanagedcluster"
	"github.com/Mirantis/hmc/test/scheme"
)

func TestUnmanagedClusterValidateCreate(t *testing.T) {
	const (
		testNamespace   = "test-namespace"
		testClusterName = "test"
	)
	g := NewWithT(t)

	ctx := context.Background()

	kubecfg := "apiVersion: v1\nclusters:\n- cluster:\n    certificate-authority-data: \n\tserver: https://nowhere.xyz\n" +
		"  name: test\ncontexts:\n- context:\n    cluster: test\n    user: test-admin\n  name: test-admin@test\n" +
		"current-context: test-admin@test\nkind: Config\npreferences: {}\nusers:\n- name: test-admin\n  user:\n    " +
		"client-certificate-data: \n\tclient-key-data: "

	secretName := secret.Name(testClusterName, secret.Kubeconfig)
	kubeSecret := &corev1.Secret{
		TypeMeta: metav1.TypeMeta{APIVersion: "v1", Kind: "Secret"},
		ObjectMeta: metav1.ObjectMeta{
			Name: secretName, Namespace: testNamespace,
			Labels: map[string]string{v1beta1.ClusterNameLabel: testClusterName},
		},
		Data: map[string][]byte{secret.KubeconfigDataName: []byte(kubecfg)},
	}

	tests := []struct {
		name            string
		tm              *v1alpha1.UnmanagedCluster
		existingObjects []runtime.Object
		err             string
		warnings        admission.Warnings
	}{
		{
			name:            "should fail if the required secret does not exist",
			tm:              uc.NewUnmanagedCluster(uc.WithNameAndNamespace(testClusterName, testNamespace)),
			existingObjects: nil,
			err:             fmt.Sprintf("required secret with name: %s not found in namespace: %s", secretName, testNamespace),
		},
		{
			name:            "should succeed",
			tm:              uc.NewUnmanagedCluster(uc.WithNameAndNamespace(testClusterName, testNamespace)),
			existingObjects: []runtime.Object{kubeSecret},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := fake.NewClientBuilder().
				WithScheme(scheme.Scheme).
				WithRuntimeObjects(tt.existingObjects...).
				Build()
			validator := &UnmanagedClusterValidator{Client: c}
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
