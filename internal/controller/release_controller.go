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
	"fmt"
	"time"

	hmc "github.com/Mirantis/hmc/api/v1alpha1"
	"github.com/go-logr/logr"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	pollPeriod = 10 * time.Minute
)

// Poller reconciles a Template object
type Poller struct {
	client.Client

	CreateManagement bool
}

func (p *Poller) Start(ctx context.Context) error {
	timer := time.NewTimer(0)
	for {
		select {
		case <-timer.C:
			p.Tick(ctx)
			timer.Reset(pollPeriod)
		case <-ctx.Done():
			return nil
		}
	}
}

func (p *Poller) Tick(ctx context.Context) {
	l := log.FromContext(ctx).WithValues("controller", "ReleaseController")

	l.Info("Poll is run")
	defer l.Info("Poll is finished")

	err := p.ensureManagement(ctx, l)
	if err != nil {
		l.Error(err, "failed to ensure default Management object")
	}
}

func (p *Poller) ensureManagement(ctx context.Context, l logr.Logger) error {
	if !p.CreateManagement {
		return nil
	}
	mgmtObj := &hmc.Management{
		ObjectMeta: metav1.ObjectMeta{
			Name:      hmc.ManagementName,
			Namespace: hmc.ManagementNamespace,
		},
	}
	err := p.Get(ctx, client.ObjectKey{
		Name:      hmc.ManagementName,
		Namespace: hmc.ManagementNamespace,
	}, mgmtObj)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("failed to get %s/%s Management object", hmc.ManagementNamespace, hmc.ManagementName)
		}
		mgmtObj.Spec.SetDefaults()
		err := p.Create(ctx, mgmtObj)
		if err != nil {
			return fmt.Errorf("failed to create %s/%s Management object", hmc.ManagementNamespace, hmc.ManagementName)
		}
		l.Info("Successfully created Management object with default configuration")
	}
	return nil
}
