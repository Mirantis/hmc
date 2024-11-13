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

package webhook

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/secret"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	hmcv1alpha1 "github.com/Mirantis/hmc/api/v1alpha1"
)

type UnmanagedClusterValidator struct {
	client.Client
}

func (v *UnmanagedClusterValidator) SetupWebhookWithManager(mgr ctrl.Manager) error {
	v.Client = mgr.GetClient()
	return ctrl.NewWebhookManagedBy(mgr).
		For(&hmcv1alpha1.UnmanagedCluster{}).
		WithValidator(v).
		WithDefaulter(v).
		Complete()
}

var (
	_ webhook.CustomValidator = &UnmanagedClusterValidator{}
	_ webhook.CustomDefaulter = &UnmanagedClusterValidator{}
)

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type.
func (v *UnmanagedClusterValidator) ValidateCreate(ctx context.Context, newObj runtime.Object) (admission.Warnings, error) {
	return v.validate(ctx, newObj)
}

func (v *UnmanagedClusterValidator) validate(ctx context.Context, newObj runtime.Object) (admission.Warnings, error) {
	unmanagedCluster, ok := newObj.(*hmcv1alpha1.UnmanagedCluster)
	if !ok {
		return nil, apierrors.NewBadRequest(fmt.Sprintf("expected UnmanagedCluster but got a %T", newObj))
	}

	if !unmanagedCluster.DeletionTimestamp.IsZero() {
		return nil, nil
	}

	kubecfgSeccret := &corev1.Secret{}
	if err := v.Client.Get(ctx, types.NamespacedName{
		Namespace: unmanagedCluster.Namespace,
		Name:      secret.Name(unmanagedCluster.Name, secret.Kubeconfig),
	}, kubecfgSeccret); err != nil && !apierrors.IsNotFound(err) {
		return nil, apierrors.NewInternalError(err)
	} else if apierrors.IsNotFound(err) {
		return nil, apierrors.NewBadRequest(fmt.Sprintf("required secret with name: %s not found in namespace: %s",
			secret.Name(unmanagedCluster.Name, secret.Kubeconfig), unmanagedCluster.Namespace))
	}

	if _, ok := kubecfgSeccret.Data[secret.KubeconfigDataName]; !ok {
		return nil, apierrors.NewBadRequest(fmt.Sprintf("required secret with name: %s does not have a data item "+
			"with key %s", kubecfgSeccret.Name, secret.KubeconfigDataName))
	}

	if cluserNameLabel, ok := kubecfgSeccret.Labels[v1beta1.ClusterNameLabel]; !ok || cluserNameLabel != unmanagedCluster.Name {
		return nil, apierrors.NewBadRequest(fmt.Sprintf("required secret with name: %s does not have a %s label set to: %s",
			secret.Name(unmanagedCluster.Name, secret.Kubeconfig), v1beta1.ClusterNameLabel, unmanagedCluster.Name))
	}

	return nil, nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type.
func (v *UnmanagedClusterValidator) ValidateUpdate(ctx context.Context, _, newObj runtime.Object) (admission.Warnings, error) {
	return v.validate(ctx, newObj)
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type.
func (*UnmanagedClusterValidator) ValidateDelete(_ context.Context, _ runtime.Object) (admission.Warnings, error) {
	return nil, nil
}

// Default implements webhook.Defaulter so a webhook will be registered for the type.
func (*UnmanagedClusterValidator) Default(_ context.Context, _ runtime.Object) error {
	return nil
}
