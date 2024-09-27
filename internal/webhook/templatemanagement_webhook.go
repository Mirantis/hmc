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
	"sort"
	"strings"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/Mirantis/hmc/api/v1alpha1"
	"github.com/Mirantis/hmc/internal/templateutil"
)

var errTemplateManagementDeletionForbidden = errors.New("TemplateManagement deletion is forbidden")

type TemplateManagementValidator struct {
	client.Client
	SystemNamespace string
}

func (v *TemplateManagementValidator) SetupWebhookWithManager(mgr ctrl.Manager) error {
	v.Client = mgr.GetClient()
	return ctrl.NewWebhookManagedBy(mgr).
		For(&v1alpha1.TemplateManagement{}).
		WithValidator(v).
		WithDefaulter(v).
		Complete()
}

var (
	_ webhook.CustomValidator = &TemplateManagementValidator{}
	_ webhook.CustomDefaulter = &TemplateManagementValidator{}
)

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type.
func (v *TemplateManagementValidator) ValidateCreate(ctx context.Context, _ runtime.Object) (admission.Warnings, error) {
	itemsList := &metav1.PartialObjectMetadataList{}
	gvk := v1alpha1.GroupVersion.WithKind(v1alpha1.TemplateManagementKind)
	itemsList.SetGroupVersionKind(gvk)
	if err := v.List(ctx, itemsList); err != nil {
		return nil, err
	}
	if len(itemsList.Items) > 0 {
		return nil, fmt.Errorf("TemplateManagement object already exists")
	}
	return nil, nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type.
func (v *TemplateManagementValidator) ValidateUpdate(ctx context.Context, _ runtime.Object, newObj runtime.Object) (admission.Warnings, error) {
	newTm, ok := newObj.(*v1alpha1.TemplateManagement)
	if !ok {
		return nil, apierrors.NewBadRequest(fmt.Sprintf("expected TemplateManagement but got a %T", newObj))
	}
	currentState, err := templateutil.GetCurrentTemplatesState(ctx, v.Client, v.SystemNamespace)
	if err != nil {
		return nil, fmt.Errorf("could not get current templates state: %v", err)
	}

	expectedState, err := templateutil.ParseAccessRules(ctx, v.Client, v.SystemNamespace, newTm.Spec.AccessRules, currentState)
	if err != nil {
		return nil, fmt.Errorf("failed to parse access rules for TemplateManagement: %v", err)
	}

	warnings := admission.Warnings{}
	for templateName, namespaces := range expectedState.ClusterTemplatesState {
		for namespace, keep := range namespaces {
			if !keep {
				managedClusters, err := getManagedClustersForTemplate(ctx, v.Client, namespace, templateName)
				if err != nil {
					return nil, err
				}

				if len(managedClusters) > 0 {
					errMsg := fmt.Sprintf("ClusterTemplate \"%s/%s\" can't be removed: found ManagedClusters that reference it: ", namespace, templateName)
					sort.Slice(managedClusters, func(i, j int) bool {
						return managedClusters[i].Name < managedClusters[j].Name
					})
					for _, cluster := range managedClusters {
						errMsg += fmt.Sprintf("\"%s/%s\", ", cluster.Namespace, cluster.Name)
					}
					warnings = append(warnings, strings.TrimRight(errMsg, ", "))
				}
			}
		}
	}
	if len(warnings) > 0 {
		sort.Strings(warnings)
		return warnings, fmt.Errorf("can not apply new access rules")
	}
	return nil, nil
}

func getManagedClustersForTemplate(ctx context.Context, cl client.Client, namespace, templateName string) ([]v1alpha1.ManagedCluster, error) {
	managedClusters := &v1alpha1.ManagedClusterList{}
	err := cl.List(ctx, managedClusters,
		client.InNamespace(namespace),
		client.MatchingFields{v1alpha1.TemplateKey: templateName},
	)
	if err != nil {
		return nil, err
	}
	return managedClusters.Items, nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type.
func (v *TemplateManagementValidator) ValidateDelete(ctx context.Context, _ runtime.Object) (admission.Warnings, error) {
	partialList := &metav1.PartialObjectMetadataList{}
	gvk := v1alpha1.GroupVersion.WithKind(v1alpha1.ManagementKind)
	partialList.SetGroupVersionKind(gvk)
	err := v.List(ctx, partialList)
	if err != nil {
		return nil, fmt.Errorf("failed to list Management objects: %v", err)
	}
	if len(partialList.Items) > 0 {
		mgmt := partialList.Items[0]
		if mgmt.DeletionTimestamp == nil {
			return nil, errTemplateManagementDeletionForbidden
		}
	}
	return nil, nil
}

// Default implements webhook.Defaulter so a webhook will be registered for the type.
func (*TemplateManagementValidator) Default(context.Context, runtime.Object) error {
	return nil
}
