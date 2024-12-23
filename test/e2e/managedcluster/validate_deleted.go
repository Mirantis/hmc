// Copyright 2024
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package managedcluster

import (
	"context"
	"errors"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/Mirantis/hmc/internal/utils/status"
	"github.com/Mirantis/hmc/test/e2e/kubeclient"
	"github.com/Mirantis/hmc/test/utils"
)

// validateClusterDeleted validates that the Cluster resource has been deleted.
func validateClusterDeleted(ctx context.Context, kc *kubeclient.KubeClient, clusterName string) error {
	// Validate that the Cluster resource has been deleted
	cluster, err := kc.GetCluster(ctx, clusterName)
	if err != nil && !apierrors.IsNotFound(err) {
		return err
	}

	if cluster != nil {
		phase, _, _ := unstructured.NestedString(cluster.Object, "status", "phase")
		if phase != "Deleting" {
			// TODO(#474): We should have a threshold error system for situations
			// like this, we probably don't want to wait the full Eventually
			// for something like this, but we can't immediately fail the test
			// either.
			return fmt.Errorf("cluster: %q exists, but is not in 'Deleting' phase", clusterName)
		}

		conditions, err := status.ConditionsFromUnstructured(cluster)
		if err != nil {
			return fmt.Errorf("failed to get conditions from unstructured object: %w", err)
		}

		var errs error

		for _, c := range conditions {
			errs = errors.Join(errors.New(utils.ConvertConditionsToString(c)), errs)
		}

		return fmt.Errorf("cluster %q still in 'Deleting' phase with conditions:\n%w", clusterName, errs)
	}

	return nil
}

// validateMachineDeploymentsDeleted validates that all MachineDeployments have
// been deleted.
func validateMachineDeploymentsDeleted(ctx context.Context, kc *kubeclient.KubeClient, clusterName string) error {
	machineDeployments, err := kc.ListMachineDeployments(ctx, clusterName)
	if err != nil && !apierrors.IsNotFound(err) {
		return err
	}

	var mdNames []string
	if len(machineDeployments) > 0 {
		for _, md := range machineDeployments {
			mdNames = append(mdNames, md.GetName())

			return fmt.Errorf("machine deployments still exist: %s", mdNames)
		}
	}

	return nil
}

// validateK0sControlPlanesDeleted validates that all k0scontrolplanes have
// been deleted.
func validateK0sControlPlanesDeleted(ctx context.Context, kc *kubeclient.KubeClient, clusterName string) error {
	controlPlanes, err := kc.ListK0sControlPlanes(ctx, clusterName)
	if err != nil && !apierrors.IsNotFound(err) {
		return err
	}

	var cpNames []string
	if len(controlPlanes) > 0 {
		for _, cp := range controlPlanes {
			cpNames = append(cpNames, cp.GetName())

			return fmt.Errorf("k0s control planes still exist: %s", cpNames)
		}
	}

	return nil
}

func validateAWSManagedControlPlanesDeleted(ctx context.Context, kc *kubeclient.KubeClient, clusterName string) error {
	controlPlanes, err := kc.ListAWSManagedControlPlanes(ctx, clusterName)
	if err != nil && !apierrors.IsNotFound(err) {
		return err
	}

	var cpNames []string
	if len(controlPlanes) > 0 {
		for _, cp := range controlPlanes {
			cpNames = append(cpNames, cp.GetName())

			return fmt.Errorf("AWS Managed control planes still exist: %s", cpNames)
		}
	}

	return nil
}
