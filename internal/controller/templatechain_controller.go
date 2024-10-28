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
	"reflect"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	hmc "github.com/Mirantis/hmc/api/v1alpha1"
)

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

// templateChain is the interface defining a list of methods to interact with *templatechains
type templateChain interface {
	client.Object
	Kind() string
	TemplateKind() string
	GetSpec() *hmc.TemplateChainSpec
}

func (r *ClusterTemplateChainReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	l := ctrl.LoggerFrom(ctx)
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

	if !clusterTemplateChain.DeletionTimestamp.IsZero() {
		l.Info("Deleting ClusterTemplateChain")
		return r.Delete(ctx, clusterTemplateChain)
	}
	return r.Update(ctx, clusterTemplateChain)
}

func (r *ServiceTemplateChainReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	l := ctrl.LoggerFrom(ctx)
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

	if !serviceTemplateChain.DeletionTimestamp.IsZero() {
		l.Info("Deleting ServiceTemplateChain")
		return r.Delete(ctx, serviceTemplateChain)
	}
	return r.Update(ctx, serviceTemplateChain)
}

func (r *TemplateChainReconciler) Update(ctx context.Context, templateChain templateChain) (ctrl.Result, error) {
	l := ctrl.LoggerFrom(ctx)

	if controllerutil.AddFinalizer(templateChain, hmc.TemplateChainFinalizer) {
		if err := r.Client.Update(ctx, templateChain); err != nil {
			l.Error(err, "Failed to update TemplateChain finalizers")
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	systemTemplates, err := r.getSystemTemplates(ctx, templateChain)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to get system templates: %w", err)
	}

	var (
		errs         error
		keepTemplate = make(map[string]struct{}, len(templateChain.GetSpec().SupportedTemplates))
	)
	for _, supportedTemplate := range templateChain.GetSpec().SupportedTemplates {
		meta := metav1.ObjectMeta{
			Name:      supportedTemplate.Name,
			Namespace: templateChain.GetNamespace(),
			Labels: map[string]string{
				hmc.HMCManagedLabelKey: hmc.HMCManagedLabelValue,
			},
		}
		keepTemplate[supportedTemplate.Name] = struct{}{}

		source, found := systemTemplates[supportedTemplate.Name]
		if !found {
			errs = errors.Join(errs, fmt.Errorf("source %s %s/%s is not found", templateChain.TemplateKind(), r.SystemNamespace, supportedTemplate.Name))
			continue
		}
		if source.GetCommonStatus().ChartRef == nil {
			errs = errors.Join(errs, fmt.Errorf("source %s %s/%s does not have chart reference yet", templateChain.TemplateKind(), r.SystemNamespace, supportedTemplate.Name))
			continue
		}

		helmSpec := hmc.HelmSpec{
			ChartRef: source.GetCommonStatus().ChartRef,
		}

		var target client.Object
		switch templateChain.Kind() {
		case hmc.ClusterTemplateChainKind:
			target = &hmc.ClusterTemplate{ObjectMeta: meta, Spec: hmc.ClusterTemplateSpec{
				Helm: helmSpec,
			}}
		case hmc.ServiceTemplateChainKind:
			target = &hmc.ServiceTemplate{ObjectMeta: meta, Spec: hmc.ServiceTemplateSpec{
				Helm: helmSpec,
			}}
		default:
			return ctrl.Result{}, fmt.Errorf("invalid TemplateChain kind. Supported kinds are %s and %s", hmc.ClusterTemplateChainKind, hmc.ServiceTemplateChainKind)
		}

		if err := r.Create(ctx, target); err == nil {
			l.Info(templateChain.TemplateKind()+" was successfully created", "template namespace", templateChain.GetNamespace(), "template name", supportedTemplate.Name)
			continue
		}

		if !apierrors.IsAlreadyExists(err) {
			errs = errors.Join(errs, err)
		}
	}
	if errs != nil {
		return ctrl.Result{}, errs
	}

	err = r.removeOrphanedTemplates(ctx, templateChain)
	if err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

func (r *TemplateChainReconciler) Delete(ctx context.Context, chain templateChain) (ctrl.Result, error) {
	l := log.FromContext(ctx)

	err := r.removeOrphanedTemplates(ctx, chain)
	if err != nil {
		return ctrl.Result{}, err
	}

	// Removing finalizer in the end of cleanup
	l.Info("Removing TemplateChain finalizer")
	if controllerutil.RemoveFinalizer(chain, hmc.TemplateChainFinalizer) {
		return ctrl.Result{}, r.Client.Update(ctx, chain)
	}
	return ctrl.Result{}, nil
}

func (r TemplateChainReconciler) getSystemTemplates(ctx context.Context, chain templateChain) (systemTemplates map[string]templateCommon, _ error) {
	templates, err := r.getTemplates(ctx, chain.TemplateKind(), &client.ListOptions{Namespace: r.SystemNamespace})
	if err != nil {
		return nil, err
	}

	systemTemplates = make(map[string]templateCommon, len(templates))
	for _, template := range templates {
		systemTemplates[template.GetName()] = template
	}
	return systemTemplates, nil
}

func (r TemplateChainReconciler) getTemplates(ctx context.Context, templateKind string, opts *client.ListOptions) ([]templateCommon, error) {
	var templates []templateCommon

	switch templateKind {
	case hmc.ClusterTemplateKind:
		ctList := &hmc.ClusterTemplateList{}
		err := r.Client.List(ctx, ctList, opts)
		if err != nil {
			return nil, err
		}
		for _, template := range ctList.Items {
			templates = append(templates, &template)
		}
	case hmc.ServiceTemplateKind:
		stList := &hmc.ServiceTemplateList{}
		err := r.Client.List(ctx, stList, opts)
		if err != nil {
			return nil, err
		}
		for _, template := range stList.Items {
			templates = append(templates, &template)
		}
	default:
		return nil, fmt.Errorf("invalid Template kind. Supported kinds are %s and %s", hmc.ClusterTemplateKind, hmc.ServiceTemplateKind)
	}
	return templates, nil
}

func (r TemplateChainReconciler) removeOrphanedTemplates(ctx context.Context, chain templateChain) error {
	l := log.FromContext(ctx)

	managedTemplates, err := r.getTemplates(ctx, chain.TemplateKind(), &client.ListOptions{
		Namespace:     chain.GetNamespace(),
		LabelSelector: labels.SelectorFromSet(map[string]string{hmc.HMCManagedLabelKey: hmc.HMCManagedLabelValue}),
	})
	if err != nil {
		return err
	}

	// Removing templates not managed by any chain
	var errs error
	for _, template := range managedTemplates {
		isOrphaned, err := isTemplateOrphaned(ctx, r.Client, chain.TemplateKind(), template.GetNamespace(), template.GetName())
		if err != nil {
			errs = errors.Join(errs, err)
			continue
		}
		if isOrphaned {
			ll := l.WithValues("template kind", chain.TemplateKind(), "template namespace", template.GetNamespace(), "template name", template.GetName())
			ll.Info("Deleting Template")

			if err := r.Client.Delete(ctx, template); client.IgnoreNotFound(err) != nil {
				errs = errors.Join(errs, err)
				continue
			}
			ll.Info("Template has been deleted")
		}
	}
	if errs != nil {
		return errs
	}
	return nil
}

func isTemplateOrphaned(ctx context.Context, cl client.Client, templateKind, namespace, templateName string) (bool, error) {
	opts := &client.ListOptions{
		Namespace:     namespace,
		FieldSelector: fields.SelectorFromSet(fields.Set{hmc.SupportedTemplateKey: templateName}),
	}

	switch templateKind {
	case hmc.ClusterTemplateKind:
		list := &hmc.ClusterTemplateChainList{}
		err := cl.List(ctx, list, opts)
		if err != nil {
			return false, err
		}
		for _, chain := range list.Items {
			if chain.DeletionTimestamp == nil {
				return false, nil
			}
		}
		return true, nil
	case hmc.ServiceTemplateKind:
		list := &hmc.ServiceTemplateChainList{}
		err := cl.List(ctx, list, opts)
		if err != nil {
			return false, err
		}
		for _, chain := range list.Items {
			if chain.DeletionTimestamp == nil {
				return false, nil
			}
		}
		return true, nil
	default:
		return false, fmt.Errorf("invalid Template kind. Supported kinds are %s and %s", hmc.ClusterTemplateKind, hmc.ServiceTemplateKind)
	}
}

func getTemplateNamesManagedByChain(chain templateChain) []string {
	result := make([]string, 0, len(chain.GetSpec().SupportedTemplates))
	for _, template := range chain.GetSpec().SupportedTemplates {
		result = append(result, template.Name)
	}
	return result
}

var tmEvents = predicate.Funcs{
	UpdateFunc: func(e event.UpdateEvent) bool {
		// Only trigger if access rules were changed
		oldObject, ok := e.ObjectOld.(*hmc.TemplateManagement)
		if !ok {
			return false
		}
		newObject, ok := e.ObjectNew.(*hmc.TemplateManagement)
		if !ok {
			return false
		}
		return !reflect.DeepEqual(oldObject.Spec.AccessRules, newObject.Spec.AccessRules)
	},
	DeleteFunc:  func(event.DeleteEvent) bool { return false },
	GenericFunc: func(event.GenericEvent) bool { return false },
}

// SetupWithManager sets up the controller with the Manager.
func (r *ClusterTemplateChainReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&hmc.ClusterTemplateChain{}).
		Watches(&hmc.TemplateManagement{},
			handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, _ client.Object) []ctrl.Request {
				return getTemplateChainRequests(ctx, r.Client, r.SystemNamespace, &hmc.ClusterTemplateChainList{})
			}),
			builder.WithPredicates(tmEvents),
		).
		Complete(r)
}

// SetupWithManager sets up the controller with the Manager.
func (r *ServiceTemplateChainReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&hmc.ServiceTemplateChain{}).
		Watches(&hmc.TemplateManagement{},
			handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, _ client.Object) []ctrl.Request {
				return getTemplateChainRequests(ctx, r.Client, r.SystemNamespace, &hmc.ServiceTemplateChainList{})
			}),
			builder.WithPredicates(tmEvents),
		).
		Complete(r)
}

func getTemplateChainRequests(ctx context.Context, cl client.Client, systemNamespace string, list client.ObjectList) []ctrl.Request {
	err := cl.List(ctx, list, client.InNamespace(systemNamespace))
	if err != nil {
		return nil
	}

	var req []ctrl.Request
	switch chainList := list.(type) {
	case *hmc.ClusterTemplateChainList:
		for _, chain := range chainList.Items {
			req = append(req, ctrl.Request{
				NamespacedName: client.ObjectKey{
					Namespace: chain.Namespace,
					Name:      chain.Name,
				},
			})
		}
	case *hmc.ServiceTemplateChainList:
		for _, chain := range chainList.Items {
			req = append(req, ctrl.Request{
				NamespacedName: client.ObjectKey{
					Namespace: chain.Namespace,
					Name:      chain.Name,
				},
			})
		}
	}
	return req
}
