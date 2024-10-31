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
	"errors"
	"fmt"

	sveltosv1beta1 "github.com/projectsveltos/addon-controller/api/v1beta1"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	hmc "github.com/Mirantis/hmc/api/v1alpha1"
)

// SetStatusConditions transforms status from Sveltos ClusterSummary
// object and sets it into the provided list of conditions.
func SetStatusConditions(summary *sveltosv1beta1.ClusterSummary, conditions *[]metav1.Condition) error {
	if summary == nil {
		return errors.New("nil summary provided")
	}

	for _, x := range summary.Status.FeatureSummaries {
		msg := ""
		status := metav1.ConditionTrue
		if x.FailureMessage != nil && *x.FailureMessage != "" {
			msg = *x.FailureMessage
			status = metav1.ConditionFalse
		}

		apimeta.SetStatusCondition(conditions, metav1.Condition{
			Message: msg,
			Reason:  string(x.Status),
			Status:  status,
			Type:    string(x.FeatureID),
		})
	}

	for _, x := range summary.Status.HelmReleaseSummaries {
		status := metav1.ConditionTrue
		if x.ConflictMessage != "" {
			status = metav1.ConditionFalse
		}

		apimeta.SetStatusCondition(conditions, metav1.Condition{
			Message: helmReleaseConditionMessage(x.ReleaseNamespace, x.ReleaseName, x.ConflictMessage),
			Reason:  string(x.Status),
			Status:  status,
			Type:    HelmReleaseReadyConditionType(x.ReleaseNamespace, x.ReleaseName),
		})
	}

	return nil
}

// HelmReleaseReadyConditionType returns a SveltosHelmReleaseReady
// type per service to be used in status conditions.
func HelmReleaseReadyConditionType(releaseNamespace, releaseName string) string {
	return fmt.Sprintf(
		"%s.%s/%s",
		releaseNamespace,
		releaseName,
		hmc.SveltosHelmReleaseReadyCondition,
	)
}

func helmReleaseConditionMessage(releaseNamespace, releaseName, conflictMsg string) string {
	msg := "Release " + releaseNamespace + "/" + releaseName
	if conflictMsg != "" {
		msg += ": " + conflictMsg
	}

	return msg
}
