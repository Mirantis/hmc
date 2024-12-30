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

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	hmcv1alpha1 "github.com/Mirantis/hmc/api/v1alpha1"
)

type ManagementBackupValidator struct {
	client.Client
}

// SetupWebhookWithManager will setup the manager to manage the webhooks
func (v *ManagementBackupValidator) SetupWebhookWithManager(mgr ctrl.Manager) error {
	v.Client = mgr.GetClient()
	return ctrl.NewWebhookManagedBy(mgr).
		For(&hmcv1alpha1.Release{}).
		WithValidator(v).
		Complete()
}

var _ webhook.CustomValidator = &ManagementBackupValidator{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (v *ManagementBackupValidator) ValidateCreate(ctx context.Context, _ runtime.Object) (admission.Warnings, error) {
	return v.validateBackupEnabled(ctx)
}

func (v *ManagementBackupValidator) validateBackupEnabled(ctx context.Context) (admission.Warnings, error) {
	mgmt, err := getManagement(ctx, v.Client)
	if err != nil {
		return nil, fmt.Errorf("failed to get Management: %w", err)
	}

	if !mgmt.Spec.Backup.Enabled {
		return admission.Warnings{"Management backup feature is disabled"}, apierrors.NewBadRequest("management backup is disabled, create or update of ManagementBackup objects disabled")
	}

	return nil, nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (v *ManagementBackupValidator) ValidateUpdate(ctx context.Context, _, _ runtime.Object) (admission.Warnings, error) {
	return v.validateBackupEnabled(ctx)
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (*ManagementBackupValidator) ValidateDelete(context.Context, runtime.Object) (admission.Warnings, error) {
	return nil, nil
}
