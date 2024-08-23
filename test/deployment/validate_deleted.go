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
package deployment

import (
	"context"
	"fmt"

	"github.com/Mirantis/hmc/test/kubeclient"
	. "github.com/onsi/ginkgo/v2"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

var deletionValidators = map[string]resourceValidationFunc{
	"clusters":           validateClusterDeleted,
	"machinedeployments": validateMachineDeploymentsDeleted,
	"control-planes":     validateK0sControlPlanesDeleted,
}

// VerifyProviderDeleted is a provider-agnostic verification that checks
// to ensure generic resources managed by the provider have been deleted.
// It is intended to be used in conjunction with an Eventually block.
func VerifyProviderDeleted(ctx context.Context, kc *kubeclient.KubeClient, clusterName string) error {
	// Sequentially validate each resource type, only returning the first error
	// as to not move on to the next resource type until the first is resolved.
	// We use []string here since order is important.
	for _, name := range []string{"control-planes", "machinedeployments", "clusters"} {
		validator, ok := deletionValidators[name]
		if !ok {
			continue
		}

		if err := validator(ctx, kc, clusterName); err != nil {
			_, _ = fmt.Fprintf(GinkgoWriter, "[%s] validation error: %v\n", name, err)
			return err
		}

		_, _ = fmt.Fprintf(GinkgoWriter, "[%s] validation succeeded\n", name)
		delete(resourceValidators, name)
	}

	return nil
}

// validateClusterDeleted validates that the Cluster resource has been deleted.
func validateClusterDeleted(ctx context.Context, kc *kubeclient.KubeClient, clusterName string) error {
	// Validate that the Cluster resource has been deleted
	cluster, err := kc.GetCluster(ctx, clusterName)
	if err != nil {
		return err
	}

	var inPhase string

	if cluster != nil {
		phase, _, _ := unstructured.NestedString(cluster.Object, "status", "phase")
		if phase != "" {
			inPhase = ", in phase: " + phase
		}

		return fmt.Errorf("cluster %q still exists%s", clusterName, inPhase)
	}

	return nil
}

// validateMachineDeploymentsDeleted validates that all MachineDeployments have
// been deleted.
func validateMachineDeploymentsDeleted(ctx context.Context, kc *kubeclient.KubeClient, clusterName string) error {
	machineDeployments, err := kc.ListMachineDeployments(ctx, clusterName)
	if err != nil {
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
	if err != nil {
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
