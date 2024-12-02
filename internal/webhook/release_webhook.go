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
	"slices"
	"strings"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	hmcv1alpha1 "github.com/Mirantis/hmc/api/v1alpha1"
)

type ReleaseValidator struct {
	client.Client
}

// SetupWebhookWithManager will setup the manager to manage the webhooks
func (v *ReleaseValidator) SetupWebhookWithManager(mgr ctrl.Manager) error {
	v.Client = mgr.GetClient()
	return ctrl.NewWebhookManagedBy(mgr).
		For(&hmcv1alpha1.Release{}).
		WithValidator(v).
		Complete()
}

var _ webhook.CustomValidator = &ReleaseValidator{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (*ReleaseValidator) ValidateCreate(_ context.Context, _ runtime.Object) (admission.Warnings, error) {
	return nil, nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (*ReleaseValidator) ValidateUpdate(_ context.Context, _, _ runtime.Object) (admission.Warnings, error) {
	return nil, nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (v *ReleaseValidator) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	release, ok := obj.(*hmcv1alpha1.Release)
	if !ok {
		return admission.Warnings{"Wrong object"}, apierrors.NewBadRequest(fmt.Sprintf("expected Release but got a %T", obj))
	}
	mgmtList := &hmcv1alpha1.ManagementList{}
	if err := v.List(ctx, mgmtList); err != nil {
		return nil, err
	}
	if len(mgmtList.Items) == 0 {
		return nil, nil
	}
	if len(mgmtList.Items) > 1 {
		return nil, fmt.Errorf("expected 1 Management object, got %d", len(mgmtList.Items))
	}

	mgmt := mgmtList.Items[0]
	if mgmt.Spec.Release == release.Name {
		return nil, fmt.Errorf("release %s is still in use", release.Name)
	}

	templates := release.Templates()
	templatesInUse := []string{}
	for _, t := range mgmt.Templates() {
		if slices.Contains(templates, t) {
			templatesInUse = append(templatesInUse, t)
		}
	}
	if len(templatesInUse) > 0 {
		return nil, fmt.Errorf("the following ProviderTemplates associated with the Release are still in use: %s", strings.Join(templatesInUse, ", "))
	}
	return nil, nil
}
