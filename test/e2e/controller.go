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

package e2e

import (
	"context"
	"fmt"
	"strings"

	"github.com/Mirantis/hmc/test/kubeclient"
	"github.com/Mirantis/hmc/test/managedcluster"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	hmcControllerLabel = "app.kubernetes.io/name=hmc"
)

// verifyControllersUp validates that controllers for the given providers list
// are running and ready.  Optionally specify providers to check for rather than
// waiting for all providers to be ready.
func verifyControllersUp(kc *kubeclient.KubeClient, providers ...managedcluster.ProviderType) error {
	if err := validateController(kc, hmcControllerLabel, "hmc-controller-manager"); err != nil {
		return err
	}

	if providers == nil {
		providers = []managedcluster.ProviderType{
			managedcluster.ProviderCAPI,
			managedcluster.ProviderAWS,
			managedcluster.ProviderAzure,
		}
	}

	for _, provider := range providers {
		// Ensure only one controller pod is running.
		if err := validateController(kc, managedcluster.GetProviderLabel(provider), string(provider)); err != nil {
			return err
		}
	}

	return nil
}

func validateController(kc *kubeclient.KubeClient, labelSelector string, name string) error {
	deployList, err := kc.Client.AppsV1().Deployments(kc.Namespace).List(context.Background(), metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		return fmt.Errorf("failed to list %s controller deployments: %w", name, err)
	}

	if len(deployList.Items) < 1 {
		return fmt.Errorf("expected at least 1 %s controller deployment, got %d",
			name, len(deployList.Items))
	}

	deployment := deployList.Items[0]

	// Ensure the deployment is not being deleted.
	if deployment.DeletionTimestamp != nil {
		return fmt.Errorf("controller pod: %s deletion timestamp should be nil, got: %v",
			deployment.Name, deployment.DeletionTimestamp)
	}
	// Ensure the deployment is running and has the expected name.
	if !strings.Contains(deployment.Name, "controller-manager") {
		return fmt.Errorf("controller deployment name %s does not contain 'controller-manager'", deployment.Name)
	}
	if deployment.Status.ReadyReplicas < 1 {
		return fmt.Errorf("controller deployment: %s does not yet have any ReadyReplicas", deployment.Name)
	}

	return nil
}
