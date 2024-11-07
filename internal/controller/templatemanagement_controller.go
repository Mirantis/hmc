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

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	hmc "github.com/Mirantis/hmc/api/v1alpha1"
)

// TemplateManagementReconciler reconciles a TemplateManagement object
type TemplateManagementReconciler struct {
	client.Client
	Config          *rest.Config
	SystemNamespace string
}

func (r *TemplateManagementReconciler) Reconcile(ctx context.Context, req ctrl.Request) (result ctrl.Result, err error) {
	l := ctrl.LoggerFrom(ctx)
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

	systemCtChains, managedCtChains, err := r.getCurrentTemplateChains(ctx, hmc.ClusterTemplateChainKind)
	if err != nil {
		return ctrl.Result{}, err
	}
	systemStChains, managedStChains, err := r.getCurrentTemplateChains(ctx, hmc.ServiceTemplateChainKind)
	if err != nil {
		return ctrl.Result{}, err
	}

	keepCtChains := make(map[string]bool)
	keepStChains := make(map[string]bool)

	var errs error
	for _, rule := range templateMgmt.Spec.AccessRules {
		namespaces, err := getTargetNamespaces(ctx, r.Client, rule.TargetNamespaces)
		if err != nil {
			return ctrl.Result{}, err
		}
		for _, namespace := range namespaces {
			for _, ctChain := range rule.ClusterTemplateChains {
				keepCtChains[getNamespacedName(namespace, ctChain)] = true
				if systemCtChains[ctChain] == nil {
					errs = errors.Join(errs, fmt.Errorf("ClusterTemplateChain %s/%s is not found", r.SystemNamespace, ctChain))
					continue
				}
				err = r.createTemplateChain(ctx, systemCtChains[ctChain], namespace)
				if err != nil {
					errs = errors.Join(errs, err)
					continue
				}
			}
			for _, stChain := range rule.ServiceTemplateChains {
				keepStChains[getNamespacedName(namespace, stChain)] = true
				if systemStChains[stChain] == nil {
					errs = errors.Join(errs, fmt.Errorf("ServiceTemplateChain %s/%s is not found", r.SystemNamespace, stChain))
					continue
				}
				err = r.createTemplateChain(ctx, systemStChains[stChain], namespace)
				if err != nil {
					errs = errors.Join(errs, err)
					continue
				}
			}
		}
	}

	for _, managedChain := range append(managedCtChains, managedStChains...) {
		keep := false
		templateNamespacedName := getNamespacedName(managedChain.GetNamespace(), managedChain.GetName())
		switch managedChain.GetObjectKind().GroupVersionKind().Kind {
		case hmc.ClusterTemplateChainKind:
			keep = keepCtChains[templateNamespacedName]
		case hmc.ServiceTemplateChainKind:
			keep = keepStChains[templateNamespacedName]
		default:
			errs = errors.Join(errs, fmt.Errorf("invalid TemplateChain kind. Supported kinds are %s and %s", hmc.ClusterTemplateChainKind, hmc.ServiceTemplateChainKind))
		}

		if !keep {
			err := r.deleteTemplateChain(ctx, managedChain)
			if err != nil {
				errs = errors.Join(errs, err)
				continue
			}
		}
	}

	if errs != nil {
		return ctrl.Result{}, errs
	}

	templateMgmt.Status.Current = templateMgmt.Spec.AccessRules
	return ctrl.Result{}, nil
}

func getNamespacedName(namespace, name string) string {
	return fmt.Sprintf("%s/%s", namespace, name)
}

func (r *TemplateManagementReconciler) getCurrentTemplateChains(ctx context.Context, templateChainKind string) (map[string]templateChain, []templateChain, error) {
	var templateChains []templateChain
	switch templateChainKind {
	case hmc.ClusterTemplateChainKind:
		ctChainList := &hmc.ClusterTemplateChainList{}
		err := r.List(ctx, ctChainList)
		if err != nil {
			return nil, nil, err
		}
		for _, chain := range ctChainList.Items {
			templateChains = append(templateChains, &chain)
		}
	case hmc.ServiceTemplateChainKind:
		stChainList := &hmc.ServiceTemplateChainList{}
		err := r.List(ctx, stChainList)
		if err != nil {
			return nil, nil, err
		}
		for _, chain := range stChainList.Items {
			templateChains = append(templateChains, &chain)
		}
	default:
		return nil, nil, fmt.Errorf("invalid TemplateChain kind. Supported kinds are %s and %s", hmc.ClusterTemplateChainKind, hmc.ServiceTemplateChainKind)
	}

	var (
		systemTemplateChains  = make(map[string]templateChain, len(templateChains))
		managedTemplateChains = make([]templateChain, 0, len(templateChains))
	)
	for _, chain := range templateChains {
		if chain.GetNamespace() == r.SystemNamespace {
			systemTemplateChains[chain.GetName()] = chain
			continue
		}

		if chain.GetLabels()[hmc.HMCManagedLabelKey] == hmc.HMCManagedLabelValue {
			managedTemplateChains = append(managedTemplateChains, chain)
		}
	}

	return systemTemplateChains, managedTemplateChains, nil
}

func getTargetNamespaces(ctx context.Context, cl client.Client, targetNamespaces hmc.TargetNamespaces) ([]string, error) {
	if len(targetNamespaces.List) > 0 {
		return targetNamespaces.List, nil
	}
	var selector labels.Selector
	var err error
	if targetNamespaces.StringSelector != "" {
		selector, err = labels.Parse(targetNamespaces.StringSelector)
		if err != nil {
			return nil, err
		}
	} else {
		selector, err = metav1.LabelSelectorAsSelector(targetNamespaces.Selector)
		if err != nil {
			return nil, fmt.Errorf("failed to construct selector from the namespaces selector %s: %w", targetNamespaces.Selector, err)
		}
	}

	var (
		namespaces = new(corev1.NamespaceList)
		listOpts   = new(client.ListOptions)
	)
	if !selector.Empty() {
		listOpts.LabelSelector = selector
	}

	if err := cl.List(ctx, namespaces, listOpts); err != nil {
		return nil, err
	}

	result := make([]string, len(namespaces.Items))
	for i, ns := range namespaces.Items {
		result[i] = ns.Name
	}

	return result, nil
}

func (r *TemplateManagementReconciler) createTemplateChain(ctx context.Context, source templateChain, targetNamespace string) error {
	l := ctrl.LoggerFrom(ctx)

	meta := metav1.ObjectMeta{
		Name:      source.GetName(),
		Namespace: targetNamespace,
		Labels: map[string]string{
			hmc.HMCManagedLabelKey: hmc.HMCManagedLabelValue,
		},
	}
	var target templateChain
	kind := source.GetObjectKind().GroupVersionKind().Kind
	switch kind {
	case hmc.ClusterTemplateChainKind:
		target = &hmc.ClusterTemplateChain{ObjectMeta: meta, Spec: *source.GetSpec()}
	case hmc.ServiceTemplateChainKind:
		target = &hmc.ServiceTemplateChain{ObjectMeta: meta, Spec: *source.GetSpec()}
	}

	err := r.Create(ctx, target)
	if err != nil {
		if apierrors.IsAlreadyExists(err) {
			return nil
		}
		return err
	}
	l.Info(kind+" was successfully created", "target namespace", targetNamespace, "source name", source.GetName())
	return nil
}

func (r *TemplateManagementReconciler) deleteTemplateChain(ctx context.Context, chain templateChain) error {
	l := ctrl.LoggerFrom(ctx)

	err := r.Delete(ctx, chain)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return err
	}
	l.Info(chain.GetObjectKind().GroupVersionKind().Kind+" was successfully deleted", "chain namespace", chain.GetNamespace(), "chain name", chain.GetName())
	return nil
}

func (r *TemplateManagementReconciler) updateStatus(ctx context.Context, templateMgmt *hmc.TemplateManagement) error {
	if err := r.Status().Update(ctx, templateMgmt); err != nil {
		return fmt.Errorf("failed to update status for TemplateManagement %s: %w", templateMgmt.Name, err)
	}
	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *TemplateManagementReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&hmc.TemplateManagement{}).
		Complete(r)
}
