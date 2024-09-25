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

	hmc "github.com/Mirantis/hmc/api/v1alpha1"
	"github.com/go-logr/logr"
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
	HelmChartOpts  []HelmChartOpts
}

type HelmChartOpts struct {
	RepositoryURL         string
	RepositoryName        string
	ChartName             string
	ChartVersion          string
	ReleaseName           string
	ReleaseNamespace      string
	Values                *apiextensionsv1.JSON
	PlainHTTP             bool
	InsecureSkipTLSVerify bool
}

// ReconcileProfile reconciles a Sveltos Profile object.
func ReconcileProfile(ctx context.Context,
	cl client.Client,
	l logr.Logger,
	namespace string,
	name string,
	matchLabels map[string]string,
	opts ReconcileProfileOpts,
) (*sveltosv1beta1.Profile, controllerutil.OperationResult, error) {
	cp := &sveltosv1beta1.Profile{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
	}

	operation, err := ctrl.CreateOrUpdate(ctx, cl, cp, func() error {
		if cp.Labels == nil {
			cp.Labels = make(map[string]string)
		}

		cp.Labels[hmc.HMCManagedLabelKey] = hmc.HMCManagedLabelValue
		if opts.OwnerReference != nil {
			cp.OwnerReferences = []metav1.OwnerReference{*opts.OwnerReference}
		}

		cp.Spec = sveltosv1beta1.Spec{
			ClusterSelector: libsveltosv1beta1.Selector{
				LabelSelector: metav1.LabelSelector{
					MatchLabels: matchLabels,
				},
			},
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

			if hc.Values != nil {
				b, err := hc.Values.MarshalJSON()
				if err != nil {
					return fmt.Errorf("failed to marshal values to JSON for service (%s) in ManagedCluster: %w", hc.RepositoryName, err)
				}

				b, err = yaml.JSONToYAML(b)
				if err != nil {
					return fmt.Errorf("failed to convert values from JSON to YAML for service (%s) in ManagedCluster: %w", hc.RepositoryName, err)
				}

				helmChart.Values = string(b)
			}

			cp.Spec.HelmCharts = append(cp.Spec.HelmCharts, helmChart)
		}
		return nil
	})
	if err != nil {
		return nil, operation, err
	}

	if operation == controllerutil.OperationResultCreated || operation == controllerutil.OperationResultUpdated {
		l.Info(fmt.Sprintf("Successfully %s Profile (%s)", string(operation), cp.Name))
	}

	return cp, operation, nil
}

func DeleteProfile(ctx context.Context, cl client.Client, namespace string, name string) error {
	err := cl.Delete(ctx, &sveltosv1beta1.Profile{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
	})

	return client.IgnoreNotFound(err)
}
