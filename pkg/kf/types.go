// Package kf contains the subset of the Kubeflow Trainer v2 (v1alpha1) and
// Kueue APIs training operator uses. Objects are exchanged as unstructured, so only the
// fields the operator sets or reads are modelled.
package kf

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// Group and Version of the Kubeflow Trainer v2 API.
const (
	Group   = "trainer.kubeflow.org"
	Version = "v1alpha1"
)

var (
	TrainJobGVK = schema.GroupVersionKind{Group: Group, Version: Version, Kind: "TrainJob"}
)

const (
	LabelJobSetName = "jobset.sigs.k8s.io/jobset-name"
	LabelCompletion = "batch.kubernetes.io/job-completion-index"

	ConditionComplete  = "Complete"
	ConditionFailed    = "Failed"
)

// TrainJob is a training run.
type TrainJob struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Status            *TrainJobStatus `json:"status,omitempty"`
}

type TrainJobStatus struct {
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}
