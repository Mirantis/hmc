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
	"encoding/json"
	"errors"
	"fmt"
	"time"

	hcv2 "github.com/fluxcd/helm-controller/api/v2"
	sourcev1 "github.com/fluxcd/source-controller/api/v1"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chartutil"
	"helm.sh/helm/v3/pkg/storage/driver"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	hmc "github.com/Mirantis/hmc/api/v1alpha1"
	"github.com/Mirantis/hmc/internal/build"
	"github.com/Mirantis/hmc/internal/helm"
)

const (
	pollPeriod    = 10 * time.Minute
	errPollPeriod = 10 * time.Second

	hmcTemplatesReleaseName = "hmc-templates"
)

// Poller reconciles a Template object
type Poller struct {
	client.Client

	Config *rest.Config

	CreateManagement         bool
	CreateTemplateManagement bool
	CreateTemplates          bool

	HMCTemplatesChartName string
	SystemNamespace       string
}

func (p *Poller) Start(ctx context.Context) error {
	timer := time.NewTimer(0)
	for {
		select {
		case <-timer.C:
			err := p.Tick(ctx)
			if err != nil {
				timer.Reset(errPollPeriod)
			} else {
				timer.Reset(pollPeriod)
			}
		case <-ctx.Done():
			return nil
		}
	}
}

func (p *Poller) Tick(ctx context.Context) error {
	l := ctrl.LoggerFrom(ctx).WithValues("controller", "ReleaseController")

	l.Info("Poll is run")
	defer l.Info("Poll is finished")

	err := p.reconcileHMCTemplates(ctx)
	if err != nil {
		l.Error(err, "failed to reconcile HMC Templates")
		return err
	}
	mgmt, err := p.getOrCreateManagement(ctx)
	if err != nil {
		l.Error(err, "failed to get or create Management object")
		return err
	}
	err = p.ensureTemplateManagement(ctx, mgmt)
	if err != nil {
		l.Error(err, "failed to ensure default TemplateManagement object")
		return err
	}
	return nil
}

func (p *Poller) getOrCreateManagement(ctx context.Context) (*hmc.Management, error) {
	l := ctrl.LoggerFrom(ctx)
	mgmtObj := &hmc.Management{
		ObjectMeta: metav1.ObjectMeta{
			Name:       hmc.ManagementName,
			Finalizers: []string{hmc.ManagementFinalizer},
		},
	}
	err := p.Get(ctx, client.ObjectKey{
		Name: hmc.ManagementName,
	}, mgmtObj)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return nil, fmt.Errorf("failed to get %s Management object: %w", hmc.ManagementName, err)
		}
		if !p.CreateManagement {
			return nil, nil
		}
		mgmtObj.Spec.Release, err = p.getCurrentReleaseName(ctx)
		if err != nil {
			return nil, err
		}

		if err := mgmtObj.Spec.SetProvidersDefaults(); err != nil {
			return nil, err
		}

		getter := helm.NewMemoryRESTClientGetter(p.Config, p.RESTMapper())
		actionConfig := new(action.Configuration)
		err = actionConfig.Init(getter, p.SystemNamespace, "secret", l.Info)
		if err != nil {
			return nil, err
		}

		hmcConfig := make(chartutil.Values)
		release, err := actionConfig.Releases.Last("hmc")
		if err != nil {
			if !errors.Is(err, driver.ErrReleaseNotFound) {
				return nil, err
			}
		} else {
			if len(release.Config) > 0 {
				chartutil.CoalesceTables(hmcConfig, release.Config)
			}
		}

		// Initially set createManagement:false to automatically create Management object only once
		chartutil.CoalesceTables(hmcConfig, map[string]any{
			"controller": map[string]any{
				"createManagement": false,
			},
		})
		rawConfig, err := json.Marshal(hmcConfig)
		if err != nil {
			return nil, err
		}
		mgmtObj.Spec.Core = &hmc.Core{
			HMC: hmc.Component{
				Config: &apiextensionsv1.JSON{
					Raw: rawConfig,
				},
			},
		}

		err = p.Create(ctx, mgmtObj)
		if err != nil {
			return nil, fmt.Errorf("failed to create %s Management object: %s", hmc.ManagementName, err)
		}
		l.Info("Successfully created Management object with default configuration")
	}
	return mgmtObj, nil
}

func (p *Poller) ensureTemplateManagement(ctx context.Context, mgmt *hmc.Management) error {
	l := ctrl.LoggerFrom(ctx)
	if !p.CreateTemplateManagement {
		return nil
	}
	if mgmt == nil {
		return fmt.Errorf("management object is not found")
	}
	tmObj := &hmc.TemplateManagement{
		ObjectMeta: metav1.ObjectMeta{
			Name: hmc.TemplateManagementName,
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: hmc.GroupVersion.String(),
					Kind:       mgmt.Kind,
					Name:       mgmt.Name,
					UID:        mgmt.UID,
				},
			},
		},
	}
	err := p.Get(ctx, client.ObjectKey{
		Name: hmc.TemplateManagementName,
	}, tmObj)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("failed to get %s TemplateManagement object: %w", hmc.TemplateManagementName, err)
		}
		err = p.Create(ctx, tmObj)
		if err != nil {
			return fmt.Errorf("failed to create %s TemplateManagement object: %w", hmc.TemplateManagementName, err)
		}
		l.Info("Successfully created TemplateManagement object")
	}
	return nil
}

func (p *Poller) reconcileHMCTemplates(ctx context.Context) error {
	l := ctrl.LoggerFrom(ctx)
	if !p.CreateTemplates {
		l.Info("Reconciling HMC Templates is skipped")
		return nil
	}
	helmChart := &sourcev1.HelmChart{
		ObjectMeta: metav1.ObjectMeta{
			Name:      p.HMCTemplatesChartName,
			Namespace: p.SystemNamespace,
		},
	}

	operation, err := ctrl.CreateOrUpdate(ctx, p.Client, helmChart, func() error {
		if helmChart.Labels == nil {
			helmChart.Labels = make(map[string]string)
		}
		helmChart.Labels[hmc.HMCManagedLabelKey] = hmc.HMCManagedLabelValue
		helmChart.Spec = sourcev1.HelmChartSpec{
			Chart:   p.HMCTemplatesChartName,
			Version: build.Version,
			SourceRef: sourcev1.LocalHelmChartSourceReference{
				Kind: sourcev1.HelmRepositoryKind,
				Name: defaultRepoName,
			},
			Interval: metav1.Duration{Duration: helm.DefaultReconcileInterval},
		}
		return nil
	})
	if err != nil {
		return err
	}
	if operation == controllerutil.OperationResultCreated || operation == controllerutil.OperationResultUpdated {
		l.Info(fmt.Sprintf("Successfully %s %s/%s HelmChart", operation, p.SystemNamespace, p.HMCTemplatesChartName))
	}

	if _, err := helm.ArtifactReady(helmChart); err != nil {
		return fmt.Errorf("HelmChart %s/%s Artifact is not ready: %w", p.SystemNamespace, p.HMCTemplatesChartName, err)
	}

	_, operation, err = helm.ReconcileHelmRelease(ctx, p.Client, hmcTemplatesReleaseName, p.SystemNamespace, helm.ReconcileHelmReleaseOpts{
		ChartRef: &hcv2.CrossNamespaceSourceReference{
			Kind:      helmChart.Kind,
			Name:      helmChart.Name,
			Namespace: helmChart.Namespace,
		},
	})
	if err != nil {
		return err
	}
	if operation == controllerutil.OperationResultCreated || operation == controllerutil.OperationResultUpdated {
		l.Info(fmt.Sprintf("Successfully %s %s/%s HelmRelease", operation, p.SystemNamespace, hmcTemplatesReleaseName))
	}
	return nil
}

func (p *Poller) getCurrentReleaseName(ctx context.Context) (string, error) {
	releases := &hmc.ReleaseList{}
	listOptions := client.ListOptions{
		FieldSelector: fields.SelectorFromSet(fields.Set{hmc.VersionKey: build.Version}),
	}
	if err := p.Client.List(ctx, releases, &listOptions); err != nil {
		return "", err
	}
	if len(releases.Items) != 1 {
		return "", fmt.Errorf("expected 1 Release with version %s, found %d", build.Version, len(releases.Items))
	}
	return releases.Items[0].Name, nil
}
