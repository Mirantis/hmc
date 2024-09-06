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
	"github.com/fluxcd/pkg/apis/meta"
	sourcev1 "github.com/fluxcd/source-controller/api/v1"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chartutil"
	"helm.sh/helm/v3/pkg/storage/driver"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

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

	CreateManagement bool
	CreateTemplates  bool

	// DefaultRepoType is the type specified by default in HelmRepository
	// objects.  Valid types are 'default' for http/https repositories, and
	// 'oci' for OCI repositories.  The RepositoryType is set in main based on
	// the URI scheme of the DefaultRegistryURL.
	DefaultRepoType           string
	DefaultRegistryURL        string
	RegistryCredentialsSecret string
	InsecureRegistry          bool
	HMCTemplatesChartName     string
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
	l := log.FromContext(ctx).WithValues("controller", "ReleaseController")

	l.Info("Poll is run")
	defer l.Info("Poll is finished")

	err := p.reconcileDefaultHelmRepo(ctx)
	if err != nil {
		l.Error(err, "failed to reconcile default HelmRepository")
		return err
	}
	err = p.reconcileHMCTemplates(ctx)
	if err != nil {
		l.Error(err, "failed to reconcile HMC Templates")
		return err
	}
	err = p.ensureManagement(ctx)
	if err != nil {
		l.Error(err, "failed to ensure default Management object")
		return err
	}
	return nil
}

func (p *Poller) ensureManagement(ctx context.Context) error {
	if !p.CreateManagement {
		return nil
	}
	l := log.FromContext(ctx)
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
			return fmt.Errorf("failed to get %s/%s Management object", hmc.ManagementNamespace, hmc.ManagementName)
		}
		mgmtObj.Spec.SetProvidersDefaults()

		getter := helm.NewMemoryRESTClientGetter(p.Config, p.RESTMapper())
		actionConfig := new(action.Configuration)
		err = actionConfig.Init(getter, hmc.TemplatesNamespace, "secret", l.Info)
		if err != nil {
			return err
		}

		hmcConfig := make(chartutil.Values)
		release, err := actionConfig.Releases.Last("hmc")
		if err != nil {
			if !errors.Is(err, driver.ErrReleaseNotFound) {
				return err
			}
		} else {
			if len(release.Config) > 0 {
				chartutil.CoalesceTables(hmcConfig, release.Config)
			}
		}

		// Initially set createManagement:false to automatically create Management object only once
		chartutil.CoalesceTables(hmcConfig, map[string]interface{}{
			"controller": map[string]interface{}{
				"createManagement": false,
			},
		})
		rawConfig, err := json.Marshal(hmcConfig)
		if err != nil {
			return err
		}
		mgmtObj.Spec.Core = &hmc.DefaultCoreConfiguration
		mgmtObj.Spec.Core.HMC.Config = &apiextensionsv1.JSON{
			Raw: rawConfig,
		}

		err = p.Create(ctx, mgmtObj)
		if err != nil {
			return fmt.Errorf("failed to create %s/%s Management object: %s", hmc.ManagementNamespace, hmc.ManagementName, err)
		}
		l.Info("Successfully created Management object with default configuration")
	}
	return nil
}

func (p *Poller) reconcileDefaultHelmRepo(ctx context.Context) error {
	l := log.FromContext(ctx)
	helmRepo := &sourcev1.HelmRepository{
		ObjectMeta: metav1.ObjectMeta{
			Name:      defaultRepoName,
			Namespace: hmc.TemplatesNamespace,
		},
	}
	operation, err := ctrl.CreateOrUpdate(ctx, p.Client, helmRepo, func() error {
		if helmRepo.Labels == nil {
			helmRepo.Labels = make(map[string]string)
		}

		helmRepo.Labels[hmc.HMCManagedLabelKey] = hmc.HMCManagedLabelValue
		helmRepo.Spec = sourcev1.HelmRepositorySpec{
			Type:     p.DefaultRepoType,
			URL:      p.DefaultRegistryURL,
			Interval: metav1.Duration{Duration: defaultReconcileInterval},
			Insecure: p.InsecureRegistry,
		}
		if p.RegistryCredentialsSecret != "" {
			helmRepo.Spec.SecretRef = &meta.LocalObjectReference{
				Name: p.RegistryCredentialsSecret,
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	if operation == controllerutil.OperationResultCreated || operation == controllerutil.OperationResultUpdated {
		l.Info(fmt.Sprintf("Successfully %s %s/%s HelmRepository", operation, hmc.TemplatesNamespace, defaultRepoName))
	}
	return nil
}

func (p *Poller) reconcileHMCTemplates(ctx context.Context) error {
	l := log.FromContext(ctx)
	if !p.CreateTemplates {
		l.Info("Reconciling HMC Templates is skipped")
		return nil
	}
	helmChart := &sourcev1.HelmChart{
		ObjectMeta: metav1.ObjectMeta{
			Name:      p.HMCTemplatesChartName,
			Namespace: hmc.TemplatesNamespace,
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
			Interval: metav1.Duration{Duration: defaultReconcileInterval},
		}
		return nil
	})
	if err != nil {
		return err
	}
	if operation == controllerutil.OperationResultCreated || operation == controllerutil.OperationResultUpdated {
		l.Info(fmt.Sprintf("Successfully %s %s/%s HelmChart", operation, hmc.TemplatesNamespace, p.HMCTemplatesChartName))
	}

	err, _ = helm.ArtifactReady(helmChart)
	if err != nil {
		return fmt.Errorf("HelmChart %s/%s Artifact is not ready: %w", hmc.TemplatesNamespace, p.HMCTemplatesChartName, err)
	}

	chartRef := &hcv2.CrossNamespaceSourceReference{
		Kind:      helmChart.Kind,
		Name:      helmChart.Name,
		Namespace: helmChart.Namespace,
	}
	_, operation, err = helm.ReconcileHelmRelease(ctx, p.Client, hmcTemplatesReleaseName, hmc.TemplatesNamespace, nil, nil, chartRef, defaultReconcileInterval, nil)
	if err != nil {
		return err
	}
	if operation == controllerutil.OperationResultCreated || operation == controllerutil.OperationResultUpdated {
		l.Info(fmt.Sprintf("Successfully %s %s/%s HelmRelease", operation, hmc.TemplatesNamespace, hmcTemplatesReleaseName))
	}
	return nil
}
