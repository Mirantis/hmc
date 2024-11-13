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

package controller

import (
	"context"
	"fmt"
	"strconv"

	"github.com/projectsveltos/libsveltos/lib/clusterproxy"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/secret"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	hmc "github.com/Mirantis/hmc/api/v1alpha1"
)

// UnmanagedMachineReconciler reconciles a UnmanagedMachine object
type UnmanagedMachineReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

func (r *UnmanagedMachineReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	l := log.FromContext(ctx)
	unmanagedMachine := &hmc.UnmanagedMachine{}
	if err := r.Get(ctx, req.NamespacedName, unmanagedMachine); err != nil {
		if apierrors.IsNotFound(err) {
			l.Info("UnmanagedMachine not found, ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		l.Error(err, "Failed to get UnmanagedMachine")
		return ctrl.Result{}, err
	}

	requeue, err := r.reconcileMachine(ctx, unmanagedMachine)
	if err != nil {
		return ctrl.Result{Requeue: requeue}, err
	}

	requeue, err = r.reconcileStatus(ctx, unmanagedMachine)
	if err != nil {
		return ctrl.Result{Requeue: requeue}, err
	}

	return ctrl.Result{Requeue: requeue}, nil
}

func (r *UnmanagedMachineReconciler) reconcileStatus(ctx context.Context, unmanagedMachine *hmc.UnmanagedMachine) (bool, error) {
	requeue := false

	l := ctrl.LoggerFrom(ctx)
	clusterClient, err := clusterproxy.GetCAPIKubernetesClient(ctx, l, r.Client, r.Client.Scheme(), unmanagedMachine.Namespace, unmanagedMachine.Spec.ClusterName)
	if err != nil {
		return true, fmt.Errorf("failed to connect to remote cluster: %w", err)
	}

	node := &corev1.Node{}
	if err := clusterClient.Get(ctx, types.NamespacedName{Name: unmanagedMachine.Name, Namespace: ""}, node); err != nil {
		return true, fmt.Errorf("failed to get node :%w", err)
	}

	for _, nodeCondition := range node.Status.Conditions {
		if nodeCondition.Type == corev1.NodeReady {
			unmanagedMachine.Status.Ready = true
			machineCondition := metav1.Condition{
				Type:   hmc.NodeCondition,
				Status: "True",
				Reason: hmc.SucceededReason,
			}

			if nodeCondition.Status != corev1.ConditionTrue {
				requeue = true
				machineCondition.Reason = hmc.FailedReason
				machineCondition.Status = "False"
				unmanagedMachine.Status.Ready = false
			}
			apimeta.SetStatusCondition(unmanagedMachine.GetConditions(), machineCondition)
		}
	}

	if err := r.Status().Update(ctx, unmanagedMachine); err != nil {
		return true, fmt.Errorf("failed to update unmanaged machine status: %w", err)
	}

	return requeue, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *UnmanagedMachineReconciler) SetupWithManager(mgr ctrl.Manager) error {
	if err := v1beta1.AddToScheme(r.Client.Scheme()); err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&hmc.UnmanagedMachine{}).
		Complete(r)
}

func (r *UnmanagedMachineReconciler) reconcileMachine(ctx context.Context, unmanagedMachine *hmc.UnmanagedMachine) (bool, error) {
	l := log.FromContext(ctx)

	secretName := secret.Name(unmanagedMachine.Spec.ClusterName, secret.Kubeconfig)
	l.Info("Create machine", "node", unmanagedMachine.Name)
	machine := v1beta1.Machine{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Machine",
			APIVersion: v1beta1.GroupVersion.Identifier(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      unmanagedMachine.Name,
			Namespace: unmanagedMachine.Namespace,
			Labels: map[string]string{
				v1beta1.GroupVersion.Identifier(): hmc.GroupVersion.Version,
				v1beta1.ClusterNameLabel:          unmanagedMachine.Spec.ClusterName,
			},
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: hmc.GroupVersion.Identifier(),
					Kind:       "UnmanagedMachine",
					Name:       unmanagedMachine.Name,
					UID:        unmanagedMachine.UID,
				},
			},
		},
		Spec: v1beta1.MachineSpec{
			ClusterName: unmanagedMachine.Spec.ClusterName,
			Bootstrap: v1beta1.Bootstrap{
				DataSecretName: &secretName,
			},
			InfrastructureRef: corev1.ObjectReference{
				Kind:       "UnmanagedMachine",
				Namespace:  unmanagedMachine.Namespace,
				Name:       unmanagedMachine.Name,
				APIVersion: hmc.GroupVersion.Identifier(),
			},
			ProviderID: &unmanagedMachine.Spec.ProviderID,
		},
	}

	if machine.Labels == nil {
		machine.Labels = make(map[string]string)
	}
	machine.Labels[v1beta1.MachineControlPlaneLabel] = strconv.FormatBool(unmanagedMachine.Spec.ControlPlane)
	if err := r.Create(ctx, &machine); err != nil && !apierrors.IsAlreadyExists(err) {
		return true, fmt.Errorf("failed to create machine: %w", err)
	}

	return false, nil
}
