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

package credspropagation

import (
	"context"
	"fmt"

	hmc "github.com/Mirantis/hmc/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func PropagateOpenStackSecrets(ctx context.Context, cfg *PropagationCfg) error {
	openstackManagedCluster := &hmc.ManagedCluster{}
	if err := cfg.Client.Get(ctx, client.ObjectKey{
		Name:      cfg.ManagedCluster.Name,
		Namespace: cfg.ManagedCluster.Namespace,
	}, openstackManagedCluster); err != nil {
		return fmt.Errorf("failed to get ManagedCluster %s: %w", cfg.ManagedCluster.Name, err)
	}

	openstackCredential := &hmc.Credential{}
	if err := cfg.Client.Get(ctx, client.ObjectKey{
		Name:      openstackManagedCluster.Spec.Credential,
		Namespace: openstackManagedCluster.Namespace,
	}, openstackCredential); err != nil {
		return fmt.Errorf("failed to get OpenStackCredential %s: %w", cfg.ManagedCluster.Spec.Credential, err)
	}

	// Fetch the secret containing OpenStack credentials
	openstackSecret := &corev1.Secret{}
	openstackSecretName := openstackCredential.Spec.IdentityRef.Name
	openstackSecretNamespace := openstackCredential.Spec.IdentityRef.Namespace
	if err := cfg.Client.Get(ctx, client.ObjectKey{
		Name:      openstackSecretName,
		Namespace: openstackSecretNamespace,
	}, openstackSecret); err != nil {
		return fmt.Errorf("failed to get OpenStack secret %s: %w", openstackSecretName, err)
	}

	// Generate CCM secret
	ccmSecret, err := generateOpenStackCCMSecret(openstackSecret)
	if err != nil {
		return fmt.Errorf("failed to generate OpenStack CCM secret: %s", err)
	}

	// Apply CCM config
	if err := applyCCMConfigs(ctx, cfg.KubeconfSecret, ccmSecret); err != nil {
		return fmt.Errorf("failed to apply OpenStack CCM secret: %s", err)
	}

	return nil
}

func generateOpenStackCCMSecret(openstackSecret *corev1.Secret) (*corev1.Secret, error) {
	// Use the data from the fetched secret
	secretData := map[string][]byte{
		"clouds.yaml": openstackSecret.Data["clouds.yaml"],
	}

	return makeSecret("openstack-cloud-config", metav1.NamespaceSystem, secretData), nil
}
