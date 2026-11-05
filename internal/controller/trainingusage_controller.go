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

package controller

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	forgeaiv1alpha1 "github.com/wutz/training/api/v1alpha1"
)

// TrainingUsageReconciler reconciles a TrainingUsage object.
type TrainingUsageReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	Now    func() time.Time
}

// +kubebuilder:rbac:groups=forgeai.wutz.dev,resources=trainingusages,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=forgeai.wutz.dev,resources=trainingusages/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=forgeai.wutz.dev,resources=trainingusages/finalizers,verbs=update
// +kubebuilder:rbac:groups=trainer.kubeflow.org,resources=trainjobs,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch

func (r *TrainingUsageReconciler) now() time.Time {
	if r.Now != nil {
		return r.Now()
	}
	return time.Now()
}

func (r *TrainingUsageReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	usage := &forgeaiv1alpha1.TrainingUsage{}
	if err := r.Get(ctx, req.NamespacedName, usage); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Get the TrainJob
	tj, err := r.getTrainJob(ctx, usage.Namespace, usage.Spec.TrainJob)
	if err != nil {
		return ctrl.Result{}, err
	}

	// Get pods for the job
	pods, err := r.getJobPods(ctx, tj)
	if err != nil {
		return ctrl.Result{}, err
	}

	// Calculate current usage
	newUsage, phase := r.calculateUsage(usage, tj, pods)

	// Update status if changed
	if r.needsUpdate(usage, newUsage, phase) {
		usage.Status.Usage = newUsage
		usage.Status.Phase = phase
		now := metav1.NewTime(r.now())
		usage.Status.LastUpdateTime = &now

		// Track retry attempts
		if annotations := tj.GetAnnotations(); annotations != nil {
			if attempt := annotations["forgeai.wutz.dev/retry"]; attempt != "" {
				usage.Status.Attempts = parseInt32(attempt)
			}
		}

		// Set condition
		condition := metav1.Condition{
			Type:               "Ready",
			Status:             metav1.ConditionTrue,
			LastTransitionTime: now,
			Reason:             "UsageCalculated",
			Message:            fmt.Sprintf("Usage calculated at %s", now.Format(time.RFC3339)),
		}
		meta.SetStatusCondition(&usage.Status.Conditions, condition)

		if err := r.Status().Update(ctx, usage); err != nil {
			logger.Error(err, "Failed to update TrainingUsage status")
			return ctrl.Result{}, err
		}
	}

	// Requeue active jobs periodically to update usage
	if phase == forgeaiv1alpha1.UsagePhaseActive {
		return ctrl.Result{RequeueAfter: 1 * time.Minute}, nil
	}

	return ctrl.Result{}, nil
}

func (r *TrainingUsageReconciler) getTrainJob(ctx context.Context, namespace, name string) (*unstructured.Unstructured, error) {
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(TrainJobGVK)
	if err := r.Get(ctx, types.NamespacedName{Namespace: namespace, Name: name}, u); err != nil {
		return nil, err
	}
	return u, nil
}

func (r *TrainingUsageReconciler) getJobPods(ctx context.Context, tj *unstructured.Unstructured) ([]corev1.Pod, error) {
	pods := &corev1.PodList{}
	if err := r.List(ctx, pods,
		client.InNamespace(tj.GetNamespace()),
		client.MatchingLabels{"job-set-name": tj.GetName()}); err != nil {
		return nil, err
	}
	return pods.Items, nil
}

func (r *TrainingUsageReconciler) calculateUsage(
	usage *forgeaiv1alpha1.TrainingUsage,
	tj *unstructured.Unstructured,
	pods []corev1.Pod,
) (*forgeaiv1alpha1.ResourceUsage, forgeaiv1alpha1.UsagePhase) {
	// Determine phase
	phase := forgeaiv1alpha1.UsagePhaseActive
	if finished, _ := isFinished(tj); finished {
		if failed, _ := isFailed(tj); failed {
			phase = forgeaiv1alpha1.UsagePhaseFailed
		} else {
			phase = forgeaiv1alpha1.UsagePhaseCompleted
		}
	}

	// Initialize usage
	if usage.Status.Usage == nil {
		usage.Status.Usage = &forgeaiv1alpha1.ResourceUsage{
			CPU:    *resource.NewQuantity(0, resource.DecimalSI),
			Memory: *resource.NewQuantity(0, resource.BinarySI),
			GPU:    *resource.NewQuantity(0, resource.DecimalSI),
		}
	}

	// Find start and end times
	startTime := usage.Status.StartTime
	endTime := usage.Status.EndTime
	lastUpdate := usage.Status.LastUpdateTime

	for i := range pods {
		p := &pods[i]
		if p.Status.Phase == corev1.PodRunning && startTime == nil {
			// First running pod sets the start time
			for _, cs := range p.Status.ContainerStatuses {
				if cs.State.Running != nil {
					t := metav1.NewTime(cs.State.Running.StartedAt.Time)
					if startTime == nil || t.Before(startTime) {
						startTime = &t
					}
				}
			}
		}
	}

	// Calculate duration since last update
	if startTime != nil && phase == forgeaiv1alpha1.UsagePhaseActive {
		from := startTime.Time
		if lastUpdate != nil {
			from = lastUpdate.Time
		}
		duration := r.now().Sub(from)

		// Accumulate usage: resource * workers * duration
		workers := int64(usage.Spec.Workers)
		cpuCores := usage.Spec.Resources.Requests[corev1.ResourceCPU]
		memoryBytes := usage.Spec.Resources.Requests[corev1.ResourceMemory]
		gpuCount := usage.Spec.Resources.Requests[corev1.ResourceName("nvidia.com/gpu")]

		// CPU-seconds
		cpuSeconds := cpuCores.AsApproximateFloat64() * float64(workers) * duration.Seconds()
		usage.Status.Usage.CPU.Add(*resource.NewMilliQuantity(int64(cpuSeconds*1000), resource.DecimalSI))

		// Memory-byte-seconds
		memoryByteSeconds := memoryBytes.AsApproximateFloat64() * float64(workers) * duration.Seconds()
		usage.Status.Usage.Memory.Add(*resource.NewQuantity(int64(memoryByteSeconds), resource.BinarySI))

		// GPU-seconds
		gpuSeconds := gpuCount.AsApproximateFloat64() * float64(workers) * duration.Seconds()
		usage.Status.Usage.GPU.Add(*resource.NewMilliQuantity(int64(gpuSeconds*1000), resource.DecimalSI))
	}

	// Set end time for finished jobs
	if phase != forgeaiv1alpha1.UsagePhaseActive && endTime == nil {
		now := metav1.NewTime(r.now())
		endTime = &now
	}

	usage.Status.StartTime = startTime
	usage.Status.EndTime = endTime

	return usage.Status.Usage, phase
}

func (r *TrainingUsageReconciler) needsUpdate(
	usage *forgeaiv1alpha1.TrainingUsage,
	newUsage *forgeaiv1alpha1.ResourceUsage,
	newPhase forgeaiv1alpha1.UsagePhase,
) bool {
	if usage.Status.Phase != newPhase {
		return true
	}
	if usage.Status.Usage == nil || newUsage == nil {
		return true
	}
	// Always update for active jobs (accumulating usage)
	if newPhase == forgeaiv1alpha1.UsagePhaseActive {
		return true
	}
	return false
}

// SetupWithManager sets up the controller with the Manager.
func (r *TrainingUsageReconciler) SetupWithManager(mgr ctrl.Manager) error {
	tj := &unstructured.Unstructured{}
	tj.SetGroupVersionKind(TrainJobGVK)

	// Watch TrainJobs and reconcile their TrainingUsage
	byTrainJob := handler.EnqueueRequestsFromMapFunc(func(_ context.Context, o client.Object) []reconcile.Request {
		// Find TrainingUsage for this TrainJob
		usages := &forgeaiv1alpha1.TrainingUsageList{}
		if err := mgr.GetClient().List(context.Background(), usages,
			client.InNamespace(o.GetNamespace())); err != nil {
			return nil
		}
		var requests []reconcile.Request
		for i := range usages.Items {
			if usages.Items[i].Spec.TrainJob == o.GetName() {
				requests = append(requests, reconcile.Request{
					NamespacedName: types.NamespacedName{
						Namespace: usages.Items[i].Namespace,
						Name:      usages.Items[i].Name,
					},
				})
			}
		}
		return requests
	})

	return ctrl.NewControllerManagedBy(mgr).
		For(&forgeaiv1alpha1.TrainingUsage{}).
		Watches(tj, byTrainJob).
		Named("trainingusage").
		Complete(r)
}

func parseInt32(s string) int32 {
	var n int32
	if _, err := fmt.Sscanf(s, "%d", &n); err != nil {
		return 0
	}
	return n
}

// TrainJobGVK is the GroupVersionKind for TrainJob.
var TrainJobGVK = schema.GroupVersionKind{
	Group:   "trainer.kubeflow.org",
	Version: "v1alpha1",
	Kind:    "TrainJob",
}

func isFinished(u *unstructured.Unstructured) (bool, error) {
	conditions, found, err := unstructured.NestedSlice(u.Object, "status", "conditions")
	if err != nil || !found {
		return false, err
	}
	for _, c := range conditions {
		cond, ok := c.(map[string]any)
		if !ok {
			continue
		}
		condType, _, _ := unstructured.NestedString(cond, "type")
		status, _, _ := unstructured.NestedString(cond, "status")
		if (condType == "Complete" || condType == "Failed") && status == "True" {
			return true, nil
		}
	}
	return false, nil
}

func isFailed(u *unstructured.Unstructured) (bool, error) {
	conditions, found, err := unstructured.NestedSlice(u.Object, "status", "conditions")
	if err != nil || !found {
		return false, err
	}
	for _, c := range conditions {
		cond, ok := c.(map[string]any)
		if !ok {
			continue
		}
		condType, _, _ := unstructured.NestedString(cond, "type")
		status, _, _ := unstructured.NestedString(cond, "status")
		if condType == "Failed" && status == "True" {
			return true, nil
		}
	}
	return false, nil
}
