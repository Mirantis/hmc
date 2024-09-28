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

package templateutil

import (
	"context"
	"errors"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	hmc "github.com/Mirantis/hmc/api/v1alpha1"
)

const (
	ClusterTemplateKind = "ClusterTemplate"
	ServiceTemplateKind = "ServiceTemplate"
)

type State struct {
	// ClusterTemplatesState is a map where keys are ClusterTemplate names and values is the map of namespaces
	// where this ClusterTemplate should be distributed
	ClusterTemplatesState map[string]map[string]bool
	// ServiceTemplatesState is a map where keys are ServiceTemplate names and values is the map of namespaces
	// where this ServiceTemplate should be distributed
	ServiceTemplatesState map[string]map[string]bool
}

func GetCurrentTemplatesState(ctx context.Context, cl client.Client, systemNamespace string) (State, error) {
	listOpts := &client.ListOptions{
		LabelSelector: labels.SelectorFromSet(map[string]string{hmc.HMCManagedLabelKey: hmc.HMCManagedLabelValue}),
		Namespace:     "",
	}
	clusterTemplatesList, serviceTemplatesList := &metav1.PartialObjectMetadataList{}, &metav1.PartialObjectMetadataList{}

	for _, kind := range []string{ClusterTemplateKind, ServiceTemplateKind} {
		partialList := &metav1.PartialObjectMetadataList{}
		partialList.SetGroupVersionKind(schema.GroupVersionKind{
			Group:   hmc.GroupVersion.Group,
			Version: hmc.GroupVersion.Version,
			Kind:    kind,
		})
		err := cl.List(ctx, partialList, listOpts)
		if err != nil {
			return State{}, err
		}
		if kind == ClusterTemplateKind {
			clusterTemplatesList = partialList
		}
		if kind == ServiceTemplateKind {
			serviceTemplatesList = partialList
		}
	}

	clusterTemplates, serviceTemplates := make(map[string]map[string]bool), make(map[string]map[string]bool)
	for _, ct := range clusterTemplatesList.Items {
		if ct.Namespace == systemNamespace {
			continue
		}
		if clusterTemplates[ct.Name] == nil {
			clusterTemplates[ct.Name] = make(map[string]bool)
		}
		clusterTemplates[ct.Name][ct.Namespace] = false
	}
	for _, st := range serviceTemplatesList.Items {
		if st.Namespace == systemNamespace {
			continue
		}
		if serviceTemplates[st.Name] == nil {
			serviceTemplates[st.Name] = make(map[string]bool)
		}
		serviceTemplates[st.Name][st.Namespace] = false
	}
	return State{
		ClusterTemplatesState: clusterTemplates,
		ServiceTemplatesState: serviceTemplates,
	}, nil
}

func ParseAccessRules(ctx context.Context, cl client.Client, rules []hmc.AccessRule, currentState State) (State, error) {
	var errs error

	expectedState := currentState
	if expectedState.ClusterTemplatesState == nil {
		expectedState.ClusterTemplatesState = make(map[string]map[string]bool)
	}
	if expectedState.ServiceTemplatesState == nil {
		expectedState.ServiceTemplatesState = make(map[string]map[string]bool)
	}
	for _, rule := range rules {
		var clusterTemplates []string
		var serviceTemplates []string
		for _, ctChainName := range rule.ClusterTemplateChains {
			ctChain := &hmc.ClusterTemplateChain{}
			err := cl.Get(ctx, client.ObjectKey{
				Name: ctChainName,
			}, ctChain)
			if err != nil {
				errs = errors.Join(errs, err)
				continue
			}
			for _, supportedTemplate := range ctChain.Spec.SupportedTemplates {
				clusterTemplates = append(clusterTemplates, supportedTemplate.Name)
			}
		}
		for _, stChainName := range rule.ServiceTemplateChains {
			stChain := &hmc.ServiceTemplateChain{}
			err := cl.Get(ctx, client.ObjectKey{
				Name: stChainName,
			}, stChain)
			if err != nil {
				errs = errors.Join(errs, err)
				continue
			}
			for _, supportedTemplate := range stChain.Spec.SupportedTemplates {
				serviceTemplates = append(serviceTemplates, supportedTemplate.Name)
			}
		}
		namespaces, err := getTargetNamespaces(ctx, cl, rule.TargetNamespaces)
		if err != nil {
			return State{}, err
		}
		for _, ct := range clusterTemplates {
			if expectedState.ClusterTemplatesState[ct] == nil {
				expectedState.ClusterTemplatesState[ct] = make(map[string]bool)
			}
			for _, ns := range namespaces {
				expectedState.ClusterTemplatesState[ct][ns] = true
			}
		}
		for _, st := range serviceTemplates {
			if expectedState.ServiceTemplatesState[st] == nil {
				expectedState.ServiceTemplatesState[st] = make(map[string]bool)
			}
			for _, ns := range namespaces {
				expectedState.ServiceTemplatesState[st][ns] = true
			}
		}
	}
	if errs != nil {
		return State{}, errs
	}
	return expectedState, nil
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

	namespaces := &corev1.NamespaceList{}
	listOpts := &client.ListOptions{}
	if selector.String() != "" {
		listOpts = &client.ListOptions{LabelSelector: selector}
	}
	err = cl.List(ctx, namespaces, listOpts)
	if err != nil {
		return []string{}, err
	}
	result := make([]string, len(namespaces.Items))
	for i, ns := range namespaces.Items {
		result[i] = ns.Name
	}
	return result, nil
}
