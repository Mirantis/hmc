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

package backup

import (
	"context"
	"fmt"

	velerov1api "github.com/zerospiel/velero/pkg/apis/velero/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	hmcv1alpha1 "github.com/Mirantis/hmc/api/v1alpha1"
)

func (c *Config) ReconcileBackup(ctx context.Context, backup *hmcv1alpha1.ManagementBackup) error {
	if backup == nil {
		return nil
	}

	if backup.Status.Reference == nil { // backup is not yet created
		templateSpec, err := c.getBackupTemplateSpec(ctx)
		if err != nil {
			return fmt.Errorf("failed to construct velero backup spec: %w", err)
		}

		veleroBackup := &velerov1api.Backup{
			ObjectMeta: metav1.ObjectMeta{
				Name:      backup.Name,
				Namespace: c.systemNamespace,
			},
			Spec: *templateSpec,
		}

		_ = controllerutil.SetControllerReference(backup, veleroBackup, c.scheme, controllerutil.WithBlockOwnerDeletion(false))

		if err := c.cl.Create(ctx, veleroBackup); client.IgnoreAlreadyExists(err) != nil { // avoid err-loop on status update error
			return fmt.Errorf("failed to create velero Backup: %w", err)
		}

		backup.Status.Reference = &corev1.ObjectReference{
			APIVersion: velerov1api.SchemeGroupVersion.String(),
			Kind:       "Backup",
			Namespace:  veleroBackup.Namespace,
			Name:       veleroBackup.Name,
		}

		if err := c.cl.Status().Update(ctx, backup); err != nil {
			return fmt.Errorf("failed to update backup status with updated reference: %w", err)
		}

		// velero schedule has been created, nothing yet to update here
		return nil
	}

	// if backup does not exist then it has not been run yet
	veleroBackup := new(velerov1api.Backup)
	if err := c.cl.Get(ctx, client.ObjectKey{
		Name:      backup.Name,
		Namespace: c.systemNamespace,
	}, veleroBackup); err != nil {
		return fmt.Errorf("failed to get velero Backup: %w", err)
	}

	// decrease API calls
	if equality.Semantic.DeepEqual(backup.Status.GetLastBackupCopy(), veleroBackup.Status) {
		return nil
	}

	backup.Status.LastBackup = &veleroBackup.Status
	return c.cl.Status().Update(ctx, backup)
}
