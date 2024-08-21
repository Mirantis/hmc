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
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// resourceValidationFunc is intended to validate a specific kubernetes
// resource.  It is meant to be used in conjunction with an Eventually block,
// however, in some cases it may be necessary to end the Eventually block early
// if the resource will never reach a ready state, in these instances Ginkgo's
// Fail function should be used.
type resourceValidationFunc func(context.Context, *kubeclient.KubeClient, string) error

// verifyProviderDeployed is a provider-agnostic verification that checks for
// the presence of specific resources in the cluster using
// resourceValidationFuncs and clusterValidationFuncs.
func verifyProviderDeployed(ctx context.Context, kc *kubeclient.KubeClient, clusterName string) error {
	// Sequentially validate each resource type, only returning the first error
	// as to not move on to the next resource type until the first is resolved.
	for _, resourceValidator := range []resourceValidationFunc{
		validateClusters,
		validateMachines,
		validateK0sControlPlanes,
		validateCSIDriver,
		validateCCM,
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
			return fmt.Errorf("failed to get status.phase for %s: %v", item.GetName(), err)
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

		// k0s does not use the metav1.Condition type for status
		// conditions, instead it uses a custom type so we can't use
		// ValidateConditionsTrue here, instead we'll check for "ready: true".
		conditions, found, err := unstructured.NestedFieldCopy(controlPlane.Object, "status", "conditions")
		if !found {
			return fmt.Errorf("no status conditions found for %s: %s", objKind, objName)
		}
		if err != nil {
			return fmt.Errorf("failed to get status conditions for %s: %s: %w", objKind, objName, err)
		}

		c, ok := conditions.(map[string]interface{})
		if !ok {
			return fmt.Errorf("expected K0sControlPlane condition to be type map[string]interface{}, got: %T", conditions)
		}

		if c["ready"] != "true" {
			return fmt.Errorf("K0sControlPlane %s is not ready, status: %v", controlPlane.GetName(), conditions)
		}
	}

	return nil
}

// validateCSIDriver validates that the provider CSI driver is functioning
// by creating a PVC and verifying it enters "Bound" status.
func validateCSIDriver(ctx context.Context, kc *kubeclient.KubeClient, clusterName string) error {
	clusterKC, err := kc.NewFromCluster(ctx, "default", clusterName)
	if err != nil {
		Fail(fmt.Sprintf("failed to create KubeClient for managed cluster %s: %v", clusterName, err))
	}

	pvcName := clusterName + "-test-pvc"

	_, err = clusterKC.Client.CoreV1().PersistentVolumeClaims(clusterKC.Namespace).
		Create(ctx, &corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name: pvcName,
			},
			Spec: corev1.PersistentVolumeClaimSpec{
				AccessModes: []corev1.PersistentVolumeAccessMode{
					corev1.ReadWriteOnce,
				},
				Resources: corev1.VolumeResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceStorage: resource.MustParse("1Gi"),
					},
				},
			},
		}, metav1.CreateOptions{})
	if err != nil {
		// Since these resourceValidationFuncs are intended to be used in
		// Eventually we should ensure a follow-up PVCreate is a no-op.
		if !apierrors.IsAlreadyExists(err) {
			return fmt.Errorf("failed to create test PVC: %w", err)
		}
	}

	// Verify the PVC enters "Bound" status.
	pvc, err := clusterKC.Client.CoreV1().PersistentVolumeClaims(clusterKC.Namespace).
		Get(ctx, pvcName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get test PVC: %w", err)
	}

	if pvc.Status.Phase == corev1.ClaimBound {
		return nil
	}

	return fmt.Errorf("%s PersistentVolume not yet 'Bound', current phase: %q", pvcName, pvc.Status.Phase)
}

// validateCCM validates that the provider's cloud controller manager is
// functional by creating a LoadBalancer service and verifying it is assigned
// an external IP.
func validateCCM(ctx context.Context, kc *kubeclient.KubeClient, clusterName string) error {
	clusterKC, err := kc.NewFromCluster(ctx, "default", clusterName)
	if err != nil {
		Fail(fmt.Sprintf("failed to create KubeClient for managed cluster %s: %v", clusterName, err))
	}

	_, err = clusterKC.Client.CoreV1().Services(clusterKC.Namespace).Create(ctx, &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: clusterName + "-test-service",
		},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{
				"some": "selector",
			},
			Ports: []corev1.ServicePort{
				{
					Port:       8765,
					TargetPort: intstr.FromInt(9376),
				},
			},
			Type: corev1.ServiceTypeLoadBalancer,
		},
	}, metav1.CreateOptions{})
	if err != nil {
		// Since these resourceValidationFuncs are intended to be used in
		// Eventually we should ensure a follow-up ServiceCreate is a no-op.
		if !apierrors.IsAlreadyExists(err) {
			return fmt.Errorf("failed to create test Service: %w", err)
		}
	}

	// Verify the Service is assigned an external IP.
	service, err := clusterKC.Client.CoreV1().Services(clusterKC.Namespace).
		Get(ctx, clusterName+"-test-service", metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get test Service: %w", err)
	}

	for _, i := range service.Status.LoadBalancer.Ingress {
		if i.Hostname != "" {
			return nil
		}
	}

	return fmt.Errorf("%s Service does not yet have an external hostname", service.Name)
}
