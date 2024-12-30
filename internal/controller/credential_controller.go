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
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	hmc "github.com/Mirantis/hmc/api/v1alpha1"
	"github.com/Mirantis/hmc/internal/utils"
)

const defaultSyncPeriod = 15 * time.Minute

// CredentialReconciler reconciles a Credential object
type CredentialReconciler struct {
	client.Client
}

func (r *CredentialReconciler) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, err error) {
	l := ctrl.LoggerFrom(ctx)
	l.Info("Credential reconcile start")

	cred := &hmc.Credential{}
	if err := r.Client.Get(ctx, req.NamespacedName, cred); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if err := utils.AddHMCComponentLabel(ctx, r.Client, cred); err != nil {
		l.Error(err, "adding component label")
		return ctrl.Result{}, err
	}

	defer func() {
		err = errors.Join(err, r.updateStatus(ctx, cred))
	}()

	clIdty := &unstructured.Unstructured{}
	clIdty.SetAPIVersion(cred.Spec.IdentityRef.APIVersion)
	clIdty.SetKind(cred.Spec.IdentityRef.Kind)
	clIdty.SetName(cred.Spec.IdentityRef.Name)
	clIdty.SetNamespace(cred.Spec.IdentityRef.Namespace)

	if err := r.Client.Get(ctx, client.ObjectKey{
		Name:      cred.Spec.IdentityRef.Name,
		Namespace: cred.Spec.IdentityRef.Namespace,
	}, clIdty); err != nil {
		errMsg := fmt.Sprintf("Failed to get ClusterIdentity %s: %s", cred.Spec.IdentityRef.Name, err)
		if apierrors.IsNotFound(err) {
			errMsg = fmt.Sprintf("ClusterIdentity %s not found", cred.Spec.IdentityRef.Name)
		}

		apimeta.SetStatusCondition(cred.GetConditions(), metav1.Condition{
			Type:    hmc.CredentialReadyCondition,
			Status:  metav1.ConditionFalse,
			Reason:  hmc.FailedReason,
			Message: errMsg,
		})

		return ctrl.Result{}, err
	}

	apimeta.SetStatusCondition(cred.GetConditions(), metav1.Condition{
		Type:    hmc.CredentialReadyCondition,
		Status:  metav1.ConditionTrue,
		Reason:  hmc.SucceededReason,
		Message: "Credential is ready",
	})

	return ctrl.Result{
		RequeueAfter: defaultSyncPeriod,
	}, nil
}

func (r *CredentialReconciler) updateStatus(ctx context.Context, cred *hmc.Credential) error {
	cred.Status.Ready = false
	for _, cond := range cred.Status.Conditions {
		if cond.Type == hmc.CredentialReadyCondition && cond.Status == metav1.ConditionTrue {
			cred.Status.Ready = true
			break
		}
	}

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
