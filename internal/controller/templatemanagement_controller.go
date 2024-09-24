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
	"errors"
	"fmt"

	helmcontrollerv2 "github.com/fluxcd/helm-controller/api/v2"
	sourcev1 "github.com/fluxcd/source-controller/api/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	hmc "github.com/Mirantis/hmc/api/v1alpha1"
	"github.com/Mirantis/hmc/internal/templateutil"
)

// TemplateManagementReconciler reconciles a TemplateManagement object
type TemplateManagementReconciler struct {
	client.Client
	Scheme          *runtime.Scheme
	Config          *rest.Config
	SystemNamespace string
}

func (r *TemplateManagementReconciler) Reconcile(ctx context.Context, req ctrl.Request) (result ctrl.Result, err error) {
	l := log.FromContext(ctx).WithValues("TemplateManagementController", req.Name)
	log.IntoContext(ctx, l)
	l.Info("Reconciling TemplateManagement")
	templateMgmt := &hmc.TemplateManagement{}
	if err := r.Get(ctx, req.NamespacedName, templateMgmt); err != nil {
		if apierrors.IsNotFound(err) {
			l.Info("TemplateManagement not found, ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		l.Error(err, "Failed to get TemplateManagement")
		return ctrl.Result{}, err
	}

	defer func() {
		statusErr := ""
		if err != nil {
			statusErr = err.Error()
		}
		templateMgmt.Status.Error = statusErr
		templateMgmt.Status.ObservedGeneration = templateMgmt.Generation
		err = errors.Join(err, r.updateStatus(ctx, templateMgmt))
	}()

	currentState, err := templateutil.GetCurrentTemplatesState(ctx, r.Client, r.SystemNamespace)
	if err != nil {
		return ctrl.Result{}, err
	}
	expectedState, err := templateutil.ParseAccessRules(ctx, r.Client, templateMgmt.Spec.AccessRules, currentState)
	if err != nil {
		return ctrl.Result{}, err
	}

	var errs error
	err = r.distributeTemplates(ctx, expectedState.ClusterTemplatesState, templateutil.ClusterTemplateKind)
	if err != nil {
		errs = errors.Join(errs, err)
	}
	err = r.distributeTemplates(ctx, expectedState.ServiceTemplatesState, templateutil.ServiceTemplateKind)
	if err != nil {
		errs = errors.Join(errs, err)
	}
	if errs != nil {
		return ctrl.Result{}, errs
	}

	templateMgmt.Status.Current = templateMgmt.Spec.AccessRules
	return ctrl.Result{}, nil
}

func (r *TemplateManagementReconciler) updateStatus(ctx context.Context, templateMgmt *hmc.TemplateManagement) error {
	if err := r.Status().Update(ctx, templateMgmt); err != nil {
		return fmt.Errorf("failed to update status for TemplateManagement %s: %w", templateMgmt.Name, err)
	}
	return nil
}

func (r *TemplateManagementReconciler) distributeTemplates(ctx context.Context, state map[string]map[string]bool, kind string) error {
	var errs error
	for name, namespaces := range state {
		err := r.applyTemplates(ctx, kind, name, namespaces)
		if err != nil {
			errs = errors.Join(errs, err)
		}
	}
	if errs != nil {
		return errs
	}
	return nil
}

func (r *TemplateManagementReconciler) applyTemplates(ctx context.Context, kind string, name string, namespaces map[string]bool) error {
	l := log.FromContext(ctx)
	meta := metav1.ObjectMeta{
		Name: name,
		Labels: map[string]string{
			hmc.HMCManagedLabelKey: hmc.HMCManagedLabelValue,
		},
	}

	sourceFound := false
	chartName := ""

	switch kind {
	case templateutil.ClusterTemplateKind:
		source := &hmc.ClusterTemplate{}
		err := r.Get(ctx, client.ObjectKey{
			Namespace: r.SystemNamespace,
			Name:      name,
		}, source)
		if err == nil {
			sourceFound = true
			chartName = source.Spec.Helm.ChartName
		} else if !apierrors.IsNotFound(err) {
			return err
		}
	case templateutil.ServiceTemplateKind:
		source := &hmc.ServiceTemplate{}
		err := r.Get(ctx, client.ObjectKey{
			Namespace: r.SystemNamespace,
			Name:      name,
		}, source)
		if err == nil {
			sourceFound = true
			chartName = source.Spec.Helm.ChartName
		} else if !apierrors.IsNotFound(err) {
			return err
		}
	default:
		return fmt.Errorf("invalid kind %s. Only %s or %s kinds are supported", kind, templateutil.ClusterTemplateKind, templateutil.ServiceTemplateKind)
	}

	spec := hmc.TemplateSpecCommon{
		Helm: hmc.HelmSpec{
			ChartRef: &helmcontrollerv2.CrossNamespaceSourceReference{
				Kind:      sourcev1.HelmChartKind,
				Name:      chartName,
				Namespace: r.SystemNamespace,
			},
		},
	}
	var errs error
	for ns, keep := range namespaces {
		var target client.Object
		meta.Namespace = ns
		if kind == templateutil.ClusterTemplateKind {
			target = &hmc.ClusterTemplate{ObjectMeta: meta, Spec: hmc.ClusterTemplateSpec{TemplateSpecCommon: spec}}
		}
		if kind == templateutil.ServiceTemplateKind {
			target = &hmc.ServiceTemplate{ObjectMeta: meta, Spec: hmc.ServiceTemplateSpec{TemplateSpecCommon: spec}}
		}
		if keep {
			if !sourceFound {
				errs = errors.Join(errs, fmt.Errorf("source %s %s/%s is not found", kind, r.SystemNamespace, name))
				continue
			}
			l.Info(fmt.Sprintf("Creating %s", kind), "namespace", ns, "name", name)
			err := r.Create(ctx, target)
			if err == nil {
				l.Info(fmt.Sprintf("%s was successfully created", kind), "namespace", ns, "name", name)
				continue
			}
			if !apierrors.IsAlreadyExists(err) {
				errs = errors.Join(errs, err)
			}
		} else {
			l.Info(fmt.Sprintf("Deleting %s", kind), "namespace", ns, "name", name)
			err := r.Delete(ctx, target)
			if err == nil {
				l.Info(fmt.Sprintf("%s was deleted", kind), "namespace", ns, "name", name)
				continue
			}
			if !apierrors.IsNotFound(err) {
				errs = errors.Join(errs, err)
			}
		}
	}
	if errs != nil {
		return errs
	}
	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *TemplateManagementReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&hmc.TemplateManagement{}).
		Complete(r)
}
