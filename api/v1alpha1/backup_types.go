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

package v1alpha1

import (
	velerov1 "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// BackupSpec defines the desired state of Backup
type BackupSpec struct {
	// +kubebuilder:default="0 */6 * * *"

	// Schedule is a Cron expression defining when to run the Backup.
	// A shortcut instead of filling the .customSchedule field up.
	// Default value is to backup every 6 hours.
	// If both this field and the .customSchedule field
	// are given, the schedule from the latter will be utilized.
	Schedule string `json:"schedule"`

	// Oneshot indicates whether the Backup should not be scheduled
	// and rather created immediately and only once.
	// If set to true, the .schedule field is ignored.
	// If set to true and the .customSchedule field is given,
	// the .spec.template from the latter will be utilized,
	// the HMC-required options still might override or precede the options
	// from the field.
	Oneshot bool `json:"oneshot,omitempty"`
}

// BackupStatus defines the observed state of Backup
type BackupStatus struct {
	// Reference to the underlying Velero object being managed.
	// Might be either Velero Backup or Schedule.
	Reference *corev1.ObjectReference `json:"reference,omitempty"`
	// Status of the Velero Schedule if .spec.oneshot is set to false.
	Schedule *velerov1.ScheduleStatus `json:"schedule,omitempty"`
	// Last Velero Backup that has been created.
	LastBackup *velerov1.BackupStatus `json:"lastBackup,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster

// Backup is the Schema for the backups API
type Backup struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   BackupSpec   `json:"spec,omitempty"`
	Status BackupStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// BackupList contains a list of Backup
type BackupList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Backup `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Backup{}, &BackupList{})
}
