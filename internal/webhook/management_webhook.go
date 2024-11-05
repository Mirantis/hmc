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
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	hmcv1alpha1 "github.com/Mirantis/hmc/api/v1alpha1"
)

type ManagementValidator struct {
	client.Client
}

var errManagementDeletionForbidden = errors.New("management deletion is forbidden")

func (v *ManagementValidator) SetupWebhookWithManager(mgr ctrl.Manager) error {
	v.Client = mgr.GetClient()
	return ctrl.NewWebhookManagedBy(mgr).
		For(&hmcv1alpha1.Management{}).
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
func (v *ManagementValidator) ValidateUpdate(ctx context.Context, _, newObj runtime.Object) (admission.Warnings, error) {
	const invalidMgmtMsg = "the Management is invalid"

	mgmt, ok := newObj.(*hmcv1alpha1.Management)
	if !ok {
		return nil, apierrors.NewBadRequest(fmt.Sprintf("expected Management but got a %T", newObj))
	}

	release := new(hmcv1alpha1.Release)
	if err := v.Get(ctx, client.ObjectKey{Name: mgmt.Spec.Release}, release); err != nil {
		// TODO: probably we do not want this skip if extra checks will be introduced
		if apierrors.IsNotFound(err) && (mgmt.Spec.Core == nil || mgmt.Spec.Core.CAPI.Template == "") {
			return nil, nil // nothing to do
		}
		return nil, fmt.Errorf("failed to get Release %s: %w", mgmt.Spec.Release, err)
	}

	capiTplName := release.Spec.CAPI.Template
	if mgmt.Spec.Core != nil && mgmt.Spec.Core.CAPI.Template != "" {
		capiTplName = mgmt.Spec.Core.CAPI.Template
	}

	capiTpl := new(hmcv1alpha1.ProviderTemplate)
	if err := v.Get(ctx, client.ObjectKey{Name: capiTplName}, capiTpl); err != nil {
		return nil, fmt.Errorf("failed to get ProviderTemplate %s: %w", capiTplName, err)
	}

	if len(capiTpl.Status.CAPIContracts) == 0 {
		return nil, nil // nothing to validate against
	}

	if !capiTpl.Status.Valid {
		return nil, fmt.Errorf("%s: not valid ProviderTemplate %s", invalidMgmtMsg, capiTpl.Name)
	}

	var wrongVersions error
	for _, p := range mgmt.Spec.Providers {
		tplName := p.Template
		if tplName == "" {
			tplName = release.ProviderTemplate(p.Name)
		}

		if tplName == capiTpl.Name { // skip capi itself
			continue
		}

		pTpl := new(hmcv1alpha1.ProviderTemplate)
		if err := v.Get(ctx, client.ObjectKey{Name: tplName}, pTpl); err != nil {
			return nil, fmt.Errorf("failed to get ProviderTemplate %s: %w", tplName, err)
		}

		if len(pTpl.Status.CAPIContracts) == 0 {
			continue
		}

		if !pTpl.Status.Valid {
			return nil, fmt.Errorf("%s: not valid ProviderTemplate %s", invalidMgmtMsg, tplName)
		}

		for capiVersion := range pTpl.Status.CAPIContracts {
			if _, ok := capiTpl.Status.CAPIContracts[capiVersion]; !ok {
				wrongVersions = errors.Join(wrongVersions, fmt.Errorf("core CAPI contract versions does not support %s version in the ProviderTemplate %s", capiVersion, pTpl.Name))
			}
		}
	}

	if wrongVersions != nil {
		return admission.Warnings{"The Management object has incompatible CAPI contract versions in ProviderTemplates"}, fmt.Errorf("%s: %s", invalidMgmtMsg, wrongVersions.Error())
	}

	return nil, nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type.
func (v *ManagementValidator) ValidateDelete(ctx context.Context, _ runtime.Object) (admission.Warnings, error) {
	managedClusters := &hmcv1alpha1.ManagedClusterList{}
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
