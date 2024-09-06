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

package telemetry

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/types"
	crclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/Mirantis/hmc/api/v1alpha1"
)

type Tracker struct {
	crclient.Client

	SystemNamespace string
}

const interval = 10 * time.Minute

func (t *Tracker) Start(ctx context.Context) error {
	timer := time.NewTimer(0)
	for {
		select {
		case <-timer.C:
			t.Tick(ctx)
			timer.Reset(interval)
		case <-ctx.Done():
			return nil
		}
	}
}

func (t *Tracker) Tick(ctx context.Context) {
	l := log.FromContext(ctx).WithName("telemetry tracker")

	logger := l.WithValues("event", managedClusterHeartbeatEvent)
	err := t.trackManagedClusterHeartbeat(ctx)
	if err != nil {
		logger.Error(err, "failed to track an event")
	} else {
		logger.Info("successfully tracked an event")
	}
}

func (t *Tracker) trackManagedClusterHeartbeat(ctx context.Context) error {
	mgmt := &v1alpha1.Management{}
	mgmtRef := types.NamespacedName{Name: v1alpha1.ManagementName}
	err := t.Get(ctx, mgmtRef, mgmt)
	if err != nil {
		return err
	}

	templates := make(map[string]v1alpha1.Template)
	templatesList := &v1alpha1.TemplateList{}
	err = t.List(ctx, templatesList, crclient.InNamespace(t.SystemNamespace))
	if err != nil {
		return err
	}
	for _, template := range templatesList.Items {
		templates[template.Name] = template
	}

	var errs error
	managedClusters := &v1alpha1.ManagedClusterList{}
	err = t.List(ctx, managedClusters, &crclient.ListOptions{})
	if err != nil {
		return err
	}

	for _, managedCluster := range managedClusters.Items {
		template := templates[managedCluster.Spec.Template]
		// TODO: get k0s cluster ID once it's exposed in k0smotron API
		clusterID := ""
		err = TrackManagedClusterHeartbeat(
			string(mgmt.UID),
			string(managedCluster.UID),
			clusterID,
			managedCluster.Spec.Template,
			template.Spec.Helm.ChartVersion,
			strings.Join(template.Status.Providers.InfrastructureProviders, ","),
			strings.Join(template.Status.Providers.BootstrapProviders, ","),
			strings.Join(template.Status.Providers.ControlPlaneProviders, ","),
		)
		if err != nil {
			errs = errors.Join(errs, fmt.Errorf("failed to track the heartbeat of the managedcluster %s/%s", managedCluster.Namespace, managedCluster.Name))
			continue
		}
	}
	return errs
}
