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

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	hmc "github.com/Mirantis/hmc/api/v1alpha1"
)

func PropagateOpenStackSecrets(ctx context.Context, cfg *PropagationCfg, credential *hmc.Credential) error {
	// Fetch the secret containing OpenStack credentials
	openstackSecret := &corev1.Secret{}
	if err := cfg.Client.Get(ctx, client.ObjectKey{
		Name:      credential.Spec.IdentityRef.Name,
		Namespace: credential.Spec.IdentityRef.Namespace,
	}, openstackSecret); err != nil {
		return fmt.Errorf("failed to get OpenStack secret %s: %w", credential.Spec.IdentityRef.Name, err)
	}

	// Generate CCM secret
	ccmSecret := makeSecret("openstack-cloud-config", openstackSecret.Data)

	// Apply CCM config
	if err := applyCCMConfigs(ctx, cfg.KubeconfSecret, ccmSecret); err != nil {
		return fmt.Errorf("failed to apply OpenStack CCM secret: %w", err)
	}

	return nil
}
