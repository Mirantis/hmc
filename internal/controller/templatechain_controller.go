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

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	hmc "github.com/Mirantis/hmc/api/v1alpha1"
)

const HMCManagedByChainLabelKey = "hmc.mirantis.com/managed-by-chain"

// TemplateChainReconciler reconciles a TemplateChain object
type TemplateChainReconciler struct {
	client.Client
	SystemNamespace string
}

type ClusterTemplateChainReconciler struct {
	TemplateChainReconciler
}

type ServiceTemplateChainReconciler struct {
	TemplateChainReconciler
}

// TemplateChain is the interface defining a list of methods to interact with templatechains
type TemplateChain interface {
	client.Object
	Kind() string
	TemplateKind() string
	GetSpec() *hmc.TemplateChainSpec
}

func (r *ClusterTemplateChainReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	l := log.FromContext(ctx).WithValues("ClusterTemplateChainController", req.NamespacedName)
	l.Info("Reconciling ClusterTemplateChain")

	clusterTemplateChain := &hmc.ClusterTemplateChain{}
	err := r.Get(ctx, req.NamespacedName, clusterTemplateChain)
	if err != nil {
		if apierrors.IsNotFound(err) {
			l.Info("ClusterTemplateChain not found, ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		l.Error(err, "Failed to get ClusterTemplateChain")
		return ctrl.Result{}, err
	}
	return r.ReconcileTemplateChain(ctx, clusterTemplateChain)
}

func (r *ServiceTemplateChainReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	l := log.FromContext(ctx).WithValues("ServiceTemplateChainReconciler", req.NamespacedName)
	l.Info("Reconciling ServiceTemplateChain")

	serviceTemplateChain := &hmc.ServiceTemplateChain{}
	err := r.Get(ctx, req.NamespacedName, serviceTemplateChain)
	if err != nil {
		if apierrors.IsNotFound(err) {
			l.Info("ServiceTemplateChain not found, ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		l.Error(err, "Failed to get ServiceTemplateChain")
		return ctrl.Result{}, err
	}
	return r.ReconcileTemplateChain(ctx, serviceTemplateChain)
}

func (r *TemplateChainReconciler) ReconcileTemplateChain(ctx context.Context, templateChain TemplateChain) (ctrl.Result, error) {
	l := log.FromContext(ctx)

	systemTemplates, managedTemplates, err := getCurrentTemplates(ctx, r.Client, templateChain.TemplateKind(), r.SystemNamespace, templateChain.GetNamespace(), templateChain.GetName())
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to get current templates: %v", err)
	}

	var errs error

	keepTemplate := make(map[string]bool)
	for _, supportedTemplate := range templateChain.GetSpec().SupportedTemplates {
		meta := metav1.ObjectMeta{
			Name:      supportedTemplate.Name,
			Namespace: templateChain.GetNamespace(),
			Labels: map[string]string{
				hmc.HMCManagedLabelKey:    hmc.HMCManagedLabelValue,
				HMCManagedByChainLabelKey: templateChain.GetName(),
			},
		}
		keepTemplate[supportedTemplate.Name] = true

		source, found := systemTemplates[supportedTemplate.Name]
		if !found {
			errs = errors.Join(errs, fmt.Errorf("source %s %s/%s is not found", templateChain.TemplateKind(), r.SystemNamespace, supportedTemplate.Name))
			continue
		}
		if source.GetStatus().ChartRef == nil {
			errs = errors.Join(errs, fmt.Errorf("source %s %s/%s does not have chart reference yet", templateChain.TemplateKind(), r.SystemNamespace, supportedTemplate.Name))
			continue
		}

		templateSpec := hmc.TemplateSpecCommon{
			Helm: hmc.HelmSpec{
				ChartRef: source.GetStatus().ChartRef,
			},
		}

		var target client.Object
		switch templateChain.Kind() {
		case hmc.ClusterTemplateChainKind:
			target = &hmc.ClusterTemplate{ObjectMeta: meta, Spec: hmc.ClusterTemplateSpec{
				TemplateSpecCommon: templateSpec,
			}}
		case hmc.ServiceTemplateChainKind:
			target = &hmc.ServiceTemplate{ObjectMeta: meta, Spec: hmc.ServiceTemplateSpec{
				TemplateSpecCommon: templateSpec,
			}}
		default:
			return ctrl.Result{}, fmt.Errorf("invalid TemplateChain kind. Supported kinds are %s and %s", hmc.ClusterTemplateChainKind, hmc.ServiceTemplateChainKind)
		}
		err := r.Create(ctx, target)
		if err == nil {
			l.Info(fmt.Sprintf("%s was successfully created", templateChain.TemplateKind()), "namespace", templateChain.GetNamespace(), "name", supportedTemplate)
			continue
		}
		if !apierrors.IsAlreadyExists(err) {
			errs = errors.Join(errs, err)
		}
	}
	for _, template := range managedTemplates {
		if !keepTemplate[template.GetName()] {
			l.Info(fmt.Sprintf("Deleting %s", templateChain.TemplateKind()), "namespace", templateChain.GetNamespace(), "name", template.GetName())
			err := r.Delete(ctx, template)
			if err == nil {
				l.Info(fmt.Sprintf("%s was deleted", templateChain.TemplateKind()), "namespace", templateChain.GetNamespace(), "name", template.GetName())
				continue
			}
			if !apierrors.IsNotFound(err) {
				errs = errors.Join(errs, err)
			}
		}
	}
	if errs != nil {
		return ctrl.Result{}, errs
	}
	return ctrl.Result{}, nil
}

func getCurrentTemplates(ctx context.Context, cl client.Client, templateKind, systemNamespace, targetNamespace, templateChainName string) (map[string]Template, []Template, error) {
	var templates []Template

	switch templateKind {
	case hmc.ClusterTemplateKind:
		ctList := &hmc.ClusterTemplateList{}
		err := cl.List(ctx, ctList)
		if err != nil {
			return nil, nil, err
		}
		for _, template := range ctList.Items {
			templates = append(templates, &template)
		}
	case hmc.ServiceTemplateKind:
		stList := &hmc.ServiceTemplateList{}
		err := cl.List(ctx, stList)
		if err != nil {
			return nil, nil, err
		}
		for _, template := range stList.Items {
			templates = append(templates, &template)
		}
	default:
		return nil, nil, fmt.Errorf("invalid Template kind. Supported kinds are %s and %s", hmc.ClusterTemplateKind, hmc.ServiceTemplateKind)
	}
	systemTemplates := make(map[string]Template)
	var managedTemplates []Template

	for _, template := range templates {
		if template.GetNamespace() == systemNamespace {
			systemTemplates[template.GetName()] = template
			continue
		}
		labels := template.GetLabels()
		if template.GetNamespace() == targetNamespace &&
			labels[hmc.HMCManagedLabelKey] == "true" &&
			labels[HMCManagedByChainLabelKey] == templateChainName {
			managedTemplates = append(managedTemplates, template)
		}
	}
	return systemTemplates, managedTemplates, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ClusterTemplateChainReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&hmc.ClusterTemplateChain{}).
		Complete(r)
}

// SetupWithManager sets up the controller with the Manager.
func (r *ServiceTemplateChainReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&hmc.ServiceTemplateChain{}).
		Complete(r)
}
