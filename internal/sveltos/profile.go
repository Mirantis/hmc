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

package sveltos

import (
	"context"
	"fmt"
	"math"
	"unsafe"

	hmc "github.com/Mirantis/hmc/api/v1alpha1"
	sveltosv1beta1 "github.com/projectsveltos/addon-controller/api/v1beta1"
	libsveltosv1beta1 "github.com/projectsveltos/libsveltos/api/v1beta1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/yaml"
)

type ReconcileProfileOpts struct {
	OwnerReference *metav1.OwnerReference
	LabelSelector  metav1.LabelSelector
	HelmChartOpts  []HelmChartOpts
	Priority       int32
	StopOnConflict bool
}

type HelmChartOpts struct {
	Values                *apiextensionsv1.JSON
	RepositoryURL         string
	RepositoryName        string
	ChartName             string
	ChartVersion          string
	ReleaseName           string
	ReleaseNamespace      string
	PlainHTTP             bool
	InsecureSkipTLSVerify bool
}

// ReconcileClusterProfile reconciles a Sveltos ClusterProfile object.
func ReconcileClusterProfile(
	ctx context.Context,
	cl client.Client,
	name string,
	opts ReconcileProfileOpts,
) (*sveltosv1beta1.ClusterProfile, error) {
	l := ctrl.LoggerFrom(ctx)
	obj := objectMeta(opts.OwnerReference)
	obj.SetName(name)

	cp := &sveltosv1beta1.ClusterProfile{
		ObjectMeta: obj,
	}

	operation, err := ctrl.CreateOrUpdate(ctx, cl, cp, func() error {
		spec, err := Spec(&opts)
		if err != nil {
			return err
		}
		cp.Spec = *spec

		return nil
	})
	if err != nil {
		return nil, err
	}

	if operation == controllerutil.OperationResultCreated || operation == controllerutil.OperationResultUpdated {
		l.Info(fmt.Sprintf("Successfully %s ClusterProfile %s", string(operation), cp.Name))
	}

	return cp, nil
}

// ReconcileProfile reconciles a Sveltos Profile object.
func ReconcileProfile(
	ctx context.Context,
	cl client.Client,
	namespace string,
	name string,
	opts ReconcileProfileOpts,
) (*sveltosv1beta1.Profile, error) {
	l := ctrl.LoggerFrom(ctx)
	obj := objectMeta(opts.OwnerReference)
	obj.SetNamespace(namespace)
	obj.SetName(name)

	p := &sveltosv1beta1.Profile{
		ObjectMeta: obj,
	}

	operation, err := ctrl.CreateOrUpdate(ctx, cl, p, func() error {
		spec, err := Spec(&opts)
		if err != nil {
			return err
		}
		p.Spec = *spec

		return nil
	})
	if err != nil {
		return nil, err
	}

	if operation == controllerutil.OperationResultCreated || operation == controllerutil.OperationResultUpdated {
		l.Info(fmt.Sprintf("Successfully %s Profile %s", string(operation), p.Name))
	}

	return p, nil
}

// Spec returns a spec object to be used with
// a Sveltos Profle or ClusterProfile object.
func Spec(opts *ReconcileProfileOpts) (*sveltosv1beta1.Spec, error) {
	tier, err := PriorityToTier(opts.Priority)
	if err != nil {
		return nil, err
	}

	spec := &sveltosv1beta1.Spec{
		ClusterSelector: libsveltosv1beta1.Selector{
			LabelSelector: opts.LabelSelector,
		},
		Tier:               tier,
		ContinueOnConflict: !opts.StopOnConflict,
		HelmCharts:         make([]sveltosv1beta1.HelmChart, 0, len(opts.HelmChartOpts)),
	}

	for _, hc := range opts.HelmChartOpts {
		helmChart := sveltosv1beta1.HelmChart{
			RepositoryURL:    hc.RepositoryURL,
			RepositoryName:   hc.RepositoryName,
			ChartName:        hc.ChartName,
			ChartVersion:     hc.ChartVersion,
			ReleaseName:      hc.ReleaseName,
			ReleaseNamespace: hc.ReleaseNamespace,
			HelmChartAction:  sveltosv1beta1.HelmChartActionInstall,
			RegistryCredentialsConfig: &sveltosv1beta1.RegistryCredentialsConfig{
				PlainHTTP:             hc.PlainHTTP,
				InsecureSkipTLSVerify: hc.InsecureSkipTLSVerify,
			},
		}

		if hc.PlainHTTP {
			// InsecureSkipTLSVerify is redundant in this case.
			helmChart.RegistryCredentialsConfig.InsecureSkipTLSVerify = false
		}

		if hc.Values != nil && len(hc.Values.Raw) > 0 {
			b, err := yaml.JSONToYAML(hc.Values.Raw)
			if err != nil {
				return nil, fmt.Errorf("failed to convert values from JSON to YAML for service %s: %w", hc.RepositoryName, err)
			}

			helmChart.Values = unsafe.String(&b[0], len(b))
		}

		spec.HelmCharts = append(spec.HelmCharts, helmChart)
	}

	return spec, nil
}

func objectMeta(owner *metav1.OwnerReference) metav1.ObjectMeta {
	obj := metav1.ObjectMeta{
		Labels: map[string]string{
			hmc.HMCManagedLabelKey: hmc.HMCManagedLabelValue,
		},
	}

	if owner != nil {
		obj.OwnerReferences = []metav1.OwnerReference{*owner}
	}

	return obj
}

// DeleteProfile deletes a Sveltos Profile object.
func DeleteProfile(ctx context.Context, cl client.Client, namespace string, name string) error {
	err := cl.Delete(ctx, &sveltosv1beta1.Profile{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
	})

	return client.IgnoreNotFound(err)
}

// DeleteClusterProfile deletes a Sveltos ClusterProfile object.
func DeleteClusterProfile(ctx context.Context, cl client.Client, name string) error {
	err := cl.Delete(ctx, &sveltosv1beta1.ClusterProfile{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	})

	return client.IgnoreNotFound(err)
}

// PriorityToTier converts priority value to Sveltos tier value.
func PriorityToTier(priority int32) (int32, error) {
	var mini int32 = 1
	maxi := math.MaxInt32 - mini

	// This check is needed because Sveltos asserts a min value of 1 on tier.
	if priority >= mini && priority <= maxi {
		return math.MaxInt32 - priority, nil
	}

	return 0, fmt.Errorf("invalid value %d, priority has to be between %d and %d", priority, mini, maxi)
}
