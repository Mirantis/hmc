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

package helm

import (
	"context"
	"time"

	hcv2 "github.com/fluxcd/helm-controller/api/v2"
	"github.com/fluxcd/pkg/apis/meta"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	hmc "github.com/Mirantis/hmc/api/v1alpha1"
)

func ReconcileHelmRelease(
	ctx context.Context,
	cl client.Client,
	name string,
	namespace string,
	values *apiextensionsv1.JSON,
	ownerReference *metav1.OwnerReference,
	chartRef *hcv2.CrossNamespaceSourceReference,
	reconcileInterval time.Duration,
	dependsOn []meta.NamespacedObjectReference,
) (*hcv2.HelmRelease, controllerutil.OperationResult, error) {
	helmRelease := &hcv2.HelmRelease{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}

	operation, err := ctrl.CreateOrUpdate(ctx, cl, helmRelease, func() error {
		if helmRelease.Labels == nil {
			helmRelease.Labels = make(map[string]string)
		}
		helmRelease.Labels[hmc.HMCManagedLabelKey] = hmc.HMCManagedLabelValue
		if ownerReference != nil {
			helmRelease.OwnerReferences = []metav1.OwnerReference{*ownerReference}
		}
		helmRelease.Spec = hcv2.HelmReleaseSpec{
			ChartRef:    chartRef,
			Interval:    metav1.Duration{Duration: reconcileInterval},
			ReleaseName: name,
			Values:      values,
			DependsOn:   dependsOn,
		}
		return nil
	})
	if err != nil {
		return nil, operation, err
	}
	return helmRelease, operation, nil
}

func DeleteHelmRelease(ctx context.Context, cl client.Client, name string, namespace string) error {
	err := cl.Delete(ctx, &hcv2.HelmRelease{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	})
	if client.IgnoreNotFound(err) != nil {
		return err
	}
	return nil
}
