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
	"fmt"
	"slices"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	hmc "github.com/Mirantis/hmc/api/v1alpha1"
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

	for _, kind := range []string{hmc.ClusterTemplateKind, hmc.ServiceTemplateKind} {
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
		if kind == hmc.ClusterTemplateKind {
			clusterTemplatesList = partialList
		}
		if kind == hmc.ServiceTemplateKind {
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
	ctChains, err := getTemplateChains(ctx, cl, hmc.ClusterTemplateKind)
	if err != nil {
		return State{}, err
	}
	stChains, err := getTemplateChains(ctx, cl, hmc.ServiceTemplateKind)
	if err != nil {
		return State{}, err
	}
	for _, rule := range rules {
		namespaces, err := getTargetNamespaces(ctx, cl, rule.TargetNamespaces)
		if err != nil {
			return State{}, err
		}
		for _, ct := range getSupportedTemplates(ctChains, rule.ClusterTemplateChains) {
			if expectedState.ClusterTemplatesState[ct.Name] == nil {
				expectedState.ClusterTemplatesState[ct.Name] = make(map[string]bool)
			}
			for _, ns := range namespaces {
				expectedState.ClusterTemplatesState[ct.Name][ns] = true
			}
		}
		for _, st := range getSupportedTemplates(stChains, rule.ServiceTemplateChains) {
			if expectedState.ServiceTemplatesState[st.Name] == nil {
				expectedState.ServiceTemplatesState[st.Name] = make(map[string]bool)
			}
			for _, ns := range namespaces {
				expectedState.ServiceTemplatesState[st.Name][ns] = true
			}
		}
	}
	if errs != nil {
		return State{}, errs
	}
	return expectedState, nil
}

func IsAvailableForUpgrade(ctx context.Context, cl client.Client, templateKind string, source, target string) (bool, error) {
	tmList := &hmc.TemplateManagementList{}
	listOpts := &client.ListOptions{Limit: 1}
	err := cl.List(ctx, tmList, listOpts)
	if err != nil {
		return false, err
	}
	if len(tmList.Items) == 0 {
		return false, fmt.Errorf("TemplateManagement is not found")
	}
	allChains, err := getTemplateChains(ctx, cl, templateKind)
	if err != nil {
		return false, err
	}
	for _, accessRule := range tmList.Items[0].Spec.AccessRules {
		var accessRuleChains []string
		switch templateKind {
		case hmc.ClusterTemplateKind:
			accessRuleChains = accessRule.ClusterTemplateChains
		case hmc.ServiceTemplateKind:
			accessRuleChains = accessRule.ServiceTemplateChains
		default:
			return false, fmt.Errorf("invalid template kind. Supported values: %s and %s", hmc.ClusterTemplateKind, hmc.ServiceTemplateKind)
		}
		for _, supportedTemplate := range getSupportedTemplates(allChains, accessRuleChains) {
			if source == supportedTemplate.Name {
				if slices.ContainsFunc(supportedTemplate.AvailableUpgrades, func(au hmc.AvailableUpgrade) bool {
					return target == au.Name
				}) {
					return true, nil
				}
			}
		}
	}
	return false, nil
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

func getSupportedTemplates(allChains []templateChain, accessRuleChains []string) []hmc.SupportedTemplate {
	supportedTemplates := make(map[string][]hmc.SupportedTemplate)
	for _, chain := range allChains {
		supportedTemplates[chain.GetName()] = chain.GetSupportedTemplates()
	}
	var result []hmc.SupportedTemplate
	for _, chainName := range accessRuleChains {
		result = append(result, supportedTemplates[chainName]...)
	}
	return result
}

type templateChain interface {
	client.Object
	GetSupportedTemplates() []hmc.SupportedTemplate
}

func getTemplateChains(ctx context.Context, cl client.Client, kind string) ([]templateChain, error) {
	switch kind {
	case hmc.ClusterTemplateKind:
		ctChains := &hmc.ClusterTemplateChainList{}
		err := cl.List(ctx, ctChains)
		if err != nil {
			return nil, err
		}
		templateChains := make([]templateChain, len(ctChains.Items))
		for i, chain := range ctChains.Items {
			templateChains[i] = &chain
		}
		return templateChains, nil
	case hmc.ServiceTemplateKind:
		stChains := &hmc.ServiceTemplateChainList{}
		err := cl.List(ctx, stChains)
		if err != nil {
			return nil, err
		}
		templateChains := make([]templateChain, len(stChains.Items))
		for i, chain := range stChains.Items {
			templateChains[i] = &chain
		}
		return templateChains, nil
	default:
		return nil, fmt.Errorf("invalid template kind. Supported values: %s and %s", hmc.ClusterTemplateKind, hmc.ServiceTemplateKind)
	}
}
