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
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	hmc "github.com/Mirantis/hmc/api/v1alpha1"
)

// CredentialReconciler reconciles a Credential object
type CredentialReconciler struct {
	client.Client
}

func (r *CredentialReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	l := ctrl.LoggerFrom(ctx)
	l.Info("Credential reconcile start")

	cred := &hmc.Credential{}
	if err := r.Client.Get(ctx, req.NamespacedName, cred); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	clIdty := &unstructured.Unstructured{}
	clIdty.SetAPIVersion(cred.Spec.IdentityRef.APIVersion)
	clIdty.SetKind(cred.Spec.IdentityRef.Kind)
	clIdty.SetName(cred.Spec.IdentityRef.Name)
	clIdty.SetNamespace(cred.Spec.IdentityRef.Namespace)

	if err := r.Client.Get(ctx, client.ObjectKey{
		Name:      cred.Spec.IdentityRef.Name,
		Namespace: cred.Spec.IdentityRef.Namespace,
	}, clIdty); err != nil {
		if apierrors.IsNotFound(err) {
			stateErr := r.setState(ctx, cred, hmc.CredentialNotFound)
			if stateErr != nil {
				err = errors.Join(err, stateErr)
			}

			l.Error(err, "ClusterIdentity not found")

			return ctrl.Result{}, err
		}

		l.Error(err, "failed to get ClusterIdentity")

		return ctrl.Result{}, err
	}

	if err := r.setState(ctx, cred, hmc.CredentialReady); err != nil {
		l.Error(err, "failed to set Credential state")

		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *CredentialReconciler) setState(ctx context.Context, cred *hmc.Credential, state hmc.CredentialState) error {
	cred.Status.State = state

	if err := r.Client.Status().Update(ctx, cred); err != nil {
		return fmt.Errorf("failed to update Credential %s/%s status: %w", cred.Namespace, cred.Name, err)
	}

	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *CredentialReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&hmc.Credential{}).
		Complete(r)
}
