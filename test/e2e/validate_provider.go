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

	. "github.com/onsi/ginkgo/v2"

	"github.com/Mirantis/hmc/test/kubeclient"
	"github.com/Mirantis/hmc/test/utils"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type resourceValidationFunc func(context.Context, *kubeclient.KubeClient, string) error

// verifyProviderDeployed is a provider-agnostic verification that checks for
// the presence of cluster, machine and k0scontrolplane resources and their
// underlying status conditions.  It is meant to be used in conjunction with
// an Eventually block.
func verifyProviderDeployed(ctx context.Context, kc *kubeclient.KubeClient, clusterName string) error {
	for _, resourceValidator := range []resourceValidationFunc{
		validateClusters,
		validateMachines,
		validateK0sControlPlanes,
	} {
		// XXX: Once we validate for the first time should we move the
		// validation out and consider it "done"?  Or is there a possibility
		// that the resources could enter a non-ready state later?
		if err := resourceValidator(ctx, kc, clusterName); err != nil {
			return err
		}
	}

	return nil
}

func validateClusters(ctx context.Context, kc *kubeclient.KubeClient, clusterName string) error {
	return validateNameAndStatus(ctx, kc, clusterName, schema.GroupVersionResource{
		Group:    "cluster.x-k8s.io",
		Version:  "v1beta1",
		Resource: "clusters",
	})
}

func validateMachines(ctx context.Context, kc *kubeclient.KubeClient, clusterName string) error {
	return validateNameAndStatus(ctx, kc, clusterName, schema.GroupVersionResource{
		Group:    "cluster.x-k8s.io",
		Version:  "v1beta1",
		Resource: "machines",
	})
}

func validateNameAndStatus(ctx context.Context, kc *kubeclient.KubeClient,
	clusterName string, gvr schema.GroupVersionResource) error {
	client, err := kc.GetDynamicClient(gvr)
	if err != nil {
		Fail(fmt.Sprintf("failed to get %s client: %v", gvr.Resource, err))
	}

	list, err := client.List(ctx, metav1.ListOptions{})
	if err != nil {
		Fail(fmt.Sprintf("failed to list %s: %v", gvr.Resource, err))
	}

	for _, item := range list.Items {
		phase, _, err := unstructured.NestedString(item.Object, "status", "phase")
		if err != nil {
			Fail(fmt.Sprintf("failed to get phase for %s: %v", item.GetName(), err))
		}

		if phase == "Deleting" {
			Fail(fmt.Sprintf("%s is in 'Deleting' phase", item.GetName()))
		}

		if err := utils.ValidateObjectNamePrefix(&item, clusterName); err != nil {
			Fail(err.Error())
		}

		if err := utils.ValidateConditionsTrue(&item); err != nil {
			return err
		}
	}

	return nil
}

type k0smotronControlPlaneStatus struct {
	// Ready denotes that the control plane is ready
	Ready                       bool `json:"ready"`
	ControlPlaneReady           bool `json:"controlPlaneReady"`
	Inititalized                bool `json:"initialized"`
	ExternalManagedControlPlane bool `json:"externalManagedControlPlane"`
}

func validateK0sControlPlanes(ctx context.Context, kc *kubeclient.KubeClient, clusterName string) error {
	k0sControlPlaneClient, err := kc.GetDynamicClient(schema.GroupVersionResource{
		Group:    "controlplane.cluster.x-k8s.io",
		Version:  "v1beta1",
		Resource: "K0sControlPlane",
	})
	if err != nil {
		return fmt.Errorf("failed to get K0sControlPlane client: %w", err)
	}

	controlPlanes, err := k0sControlPlaneClient.List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("failed to list K0sControlPlanes: %w", err)
	}

	for _, controlPlane := range controlPlanes.Items {
		if err := utils.ValidateObjectNamePrefix(&controlPlane, clusterName); err != nil {
			Fail(err.Error())
		}

		objKind, objName := utils.ObjKindName(&controlPlane)

		// k0smotron does not use the metav1.Condition type for status
		// conditions, instead it uses a custom type so we can't use
		// ValidateConditionsTrue here.
		conditions, found, err := unstructured.NestedFieldCopy(controlPlane.Object, "status", "conditions")
		if !found {
			return fmt.Errorf("no status conditions found for %s: %s", objKind, objName)
		}
		if err != nil {
			return fmt.Errorf("failed to get status conditions for %s: %s: %w", objKind, objName, err)
		}

		c, ok := conditions.(k0smotronControlPlaneStatus)
		if !ok {
			return fmt.Errorf("expected K0sControlPlane condition to be type K0smotronControlPlaneStatus, got: %T", conditions)
		}

		if !c.Ready {
			return fmt.Errorf("K0sControlPlane %s is not ready, status: %+v", controlPlane.GetName(), c)
		}
	}

	return nil
}
