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

	"github.com/Masterminds/semver/v3"
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

	if mgmt.Status.Release == "" {
		return nil, nil // nothing to do at the moment
	}

	skipValidation := true
	capiTplName := ""
	for cn, c := range mgmt.Status.Components {
		if cn == hmcv1alpha1.CoreCAPIName && c.Success {
			skipValidation = false
			capiTplName = c.Template
			break
		}
	}

	if skipValidation {
		return nil, nil // either no status, or capi has been failed to deploy
	}

	ptpls := new(hmcv1alpha1.ProviderTemplateList)
	if err := v.List(ctx, ptpls); err != nil {
		return nil, fmt.Errorf("failed to list ProviderTemplates: %v", err)
	}

	if len(ptpls.Items) == 0 {
		return nil, nil // nothing to do
	}

	name2Tpl := make(map[string]*hmcv1alpha1.ProviderTemplate, len(ptpls.Items)-1)
	capiRequiredVersion := new(semver.Version)
	for _, v := range ptpls.Items { // cluster-scoped
		if v.Name != capiTplName {
			name2Tpl[v.Name] = &v
			continue
		}

		if v.Status.CAPIVersion == "" {
			return nil, nil // nothing to validate against
		}

		var err error
		capiRequiredVersion, err = semver.NewVersion(v.Status.CAPIVersion)
		if err != nil { // should never happen
			return nil, fmt.Errorf("%s: invalid CAPI version %s in the ProviderTemplate %s to be validated against: %v", invalidMgmtMsg, v.Status.CAPIVersion, v.Name, err)
		}
	}

	var wrongVersions error
	for _, c := range mgmt.Status.Components {
		if c.Template == capiTplName {
			continue
		}

		tpl, ok := name2Tpl[c.Template]
		if !ok || tpl.Status.CAPIVersion == "" {
			continue
		}

		ver, err := semver.NewVersion(tpl.Status.CAPIVersion)
		if err != nil { // should never happen
			return nil, fmt.Errorf("%s: invalid CAPI version %s in the ProviderTemplate %s: %v", invalidMgmtMsg, tpl.Status.CAPIVersion, tpl.Name, err)
		}

		if !capiRequiredVersion.Equal(ver) {
			wrongVersions = errors.Join(wrongVersions, fmt.Errorf("wrong CAPI version in the ProviderTemplate %s: required %s but CAPI has %s", tpl.Name, ver, capiRequiredVersion))
		}
	}

	if wrongVersions != nil {
		return admission.Warnings{"The Management object has incompatible CAPI versions ProviderTemplates"}, fmt.Errorf("%s: %s", invalidMgmtMsg, wrongVersions)
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
