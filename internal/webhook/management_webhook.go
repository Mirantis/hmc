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
	"errors"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/Mirantis/hmc/api/v1alpha1"
)

type ManagementValidator struct {
	client.Client
}

var errManagementDeletionForbidden = errors.New("management deletion is forbidden")

func (v *ManagementValidator) SetupWebhookWithManager(mgr ctrl.Manager) error {
	v.Client = mgr.GetClient()
	return ctrl.NewWebhookManagedBy(mgr).
		For(&v1alpha1.Management{}).
		WithValidator(v).
		WithDefaulter(v).
		Complete()
}

var (
	_ webhook.CustomValidator = &ManagementValidator{}
	_ webhook.CustomDefaulter = &ManagementValidator{}
)

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type.
func (*ManagementValidator) ValidateCreate(_ context.Context, _ runtime.Object) (admission.Warnings, error) {
	return nil, nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type.
func (*ManagementValidator) ValidateUpdate(_ context.Context, _ runtime.Object, _ runtime.Object) (admission.Warnings, error) {
	return nil, nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type.
func (v *ManagementValidator) ValidateDelete(ctx context.Context, _ runtime.Object) (admission.Warnings, error) {
	managedClusters := &v1alpha1.ManagedClusterList{}
	err := v.Client.List(ctx, managedClusters, client.Limit(1))
	if err != nil {
		return nil, err
	}
	if len(managedClusters.Items) > 0 {
		return admission.Warnings{"The Management object can't be removed if ManagedCluster objects still exist"}, errManagementDeletionForbidden
	}
	return nil, nil
}

// Default implements webhook.Defaulter so a webhook will be registered for the type.
func (*ManagementValidator) Default(_ context.Context, _ runtime.Object) error {
	return nil
}
