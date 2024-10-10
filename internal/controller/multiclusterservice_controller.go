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

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	hmc "github.com/Mirantis/hmc/api/v1alpha1"
)

// MultiClusterServiceReconciler reconciles a MultiClusterService object
type MultiClusterServiceReconciler struct {
	client.Client
}

// Reconcile reconciles a MultiClusterService object.
func (*MultiClusterServiceReconciler) Reconcile(ctx context.Context, _ ctrl.Request) (ctrl.Result, error) {
	_ = ctrl.LoggerFrom(ctx)

	// TODO(https://github.com/Mirantis/hmc/issues/455): Implement me.

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *MultiClusterServiceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&hmc.MultiClusterService{}).
		Complete(r)
}
