/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TrainingUsageSpec defines the desired state of TrainingUsage.
type TrainingUsageSpec struct {
	// trainJob is the name of the TrainJob this usage record tracks.
	// +kubebuilder:validation:MinLength=1
	TrainJob string `json:"trainJob"`

	// instanceType is the InstanceType used by the training job.
	// +kubebuilder:validation:MinLength=1
	InstanceType string `json:"instanceType"`

	// workers is the number of worker nodes.
	// +kubebuilder:validation:Minimum=1
	Workers int32 `json:"workers"`

	// resources are the per-worker resource requests.
	Resources corev1.ResourceRequirements `json:"resources"`
}

// ResourceUsage captures resource consumption over a time period.
type ResourceUsage struct {
	// cpu is the total CPU-seconds consumed.
	CPU resource.Quantity `json:"cpu"`
	// memory is the total memory-byte-seconds consumed.
	Memory resource.Quantity `json:"memory"`
	// gpu is the total GPU-seconds consumed.
	GPU resource.Quantity `json:"gpu"`
	// gpuMemory is the total GPU memory-byte-seconds consumed.
	// +optional
	GPUMemory *resource.Quantity `json:"gpuMemory,omitempty"`
}

// UsagePhase represents the billing phase of a usage record.
type UsagePhase string

const (
	// UsagePhaseActive means the job is running and accumulating usage.
	UsagePhaseActive UsagePhase = "Active"
	// UsagePhaseFailed means the job failed; usage may be billable depending on policy.
	UsagePhaseFailed UsagePhase = "Failed"
	// UsagePhaseCompleted means the job succeeded and usage is finalized.
	UsagePhaseCompleted UsagePhase = "Completed"
	// UsagePhaseExported means the usage has been exported to the billing system.
	UsagePhaseExported UsagePhase = "Exported"
)

// TrainingUsageStatus defines the observed state of TrainingUsage.
type TrainingUsageStatus struct {
	// phase is the current billing phase.
	// +optional
	Phase UsagePhase `json:"phase,omitempty"`

	// startTime is when the job started running (first pod became Running).
	// +optional
	StartTime *metav1.Time `json:"startTime,omitempty"`

	// endTime is when the job finished (succeeded or failed).
	// +optional
	EndTime *metav1.Time `json:"endTime,omitempty"`

	// usage is the accumulated resource consumption.
	// +optional
	Usage *ResourceUsage `json:"usage,omitempty"`

	// lastUpdateTime is when the usage was last calculated.
	// +optional
	LastUpdateTime *metav1.Time `json:"lastUpdateTime,omitempty"`

	// attempts tracks retries: each retry's usage is summed.
	// +optional
	Attempts int32 `json:"attempts,omitempty"`

	// conditions represent the latest available observations of the usage state.
	// +optional
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,shortName=tu
// +kubebuilder:printcolumn:name="TrainJob",type=string,JSONPath=`.spec.trainJob`
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="CPU",type=string,JSONPath=`.status.usage.cpu`
// +kubebuilder:printcolumn:name="GPU",type=string,JSONPath=`.status.usage.gpu`
// +kubebuilder:printcolumn:name="Duration",type=date,JSONPath=`.status.lastUpdateTime`

// TrainingUsage tracks resource consumption of a training job for billing.
type TrainingUsage struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   TrainingUsageSpec   `json:"spec,omitempty"`
	Status TrainingUsageStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// TrainingUsageList contains a list of TrainingUsage.
type TrainingUsageList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []TrainingUsage `json:"items"`
}
