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
	"errors"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	hmc "github.com/Mirantis/hmc/api/v1alpha1"
)

// CredentialReconciler reconciles a Credential object
type CredentialReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

func (r *CredentialReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	l := log.FromContext(ctx).WithValues("CredentialController", req.NamespacedName)
	l.Info("Credential reconcile start")
	cred := &hmc.Credential{}
	err := r.Client.Get(ctx, req.NamespacedName, cred)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{},
			fmt.Errorf("error getting Credential object: %s", err)
	}

	clIdty := &unstructured.Unstructured{}
	clIdty.SetAPIVersion(cred.Spec.IdentityRef.APIVersion)
	clIdty.SetKind(cred.Spec.IdentityRef.Kind)
	clIdty.SetName(cred.Spec.IdentityRef.Name)
	clIdty.SetNamespace(cred.Spec.IdentityRef.Namespace)

	err = r.Client.Get(ctx, client.ObjectKey{
		Name:      cred.Spec.IdentityRef.Name,
		Namespace: cred.Spec.IdentityRef.Namespace,
	}, clIdty)
	if err != nil {
		if apierrors.IsNotFound(err) {
			stateErr := r.setState(ctx, cred, hmc.CredentialNotFound)
			if stateErr != nil {
				err = errors.Join(err, stateErr)
			}
			return ctrl.Result{},
				fmt.Errorf("cluster identity not found: %s", err)
		}
		return ctrl.Result{},
			fmt.Errorf("failed to get ClusterIdentity object: %s", err)
	}

	err = r.setState(ctx, cred, hmc.CredentialReady)
	if err != nil {
		return ctrl.Result{},
			fmt.Errorf("failed to set Credential state: %s", err)
	}

	return ctrl.Result{}, nil
}

func (r *CredentialReconciler) setState(ctx context.Context, cred *hmc.Credential,
	state hmc.CredentialState,
) error {
	cred.Status.State = state
	err := r.Client.Status().Update(ctx, cred)
	if err != nil {
		return err
	}
	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *CredentialReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&hmc.Credential{}).
		Complete(r)
}
