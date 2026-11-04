// Package naming holds object names, label keys and annotations shared by the
// training operator.
package naming

// Labels and annotations.
const (
	LabelManagedBy = "app.kubernetes.io/managed-by"
	ManagedBy      = "forgeai"

	// LabelJobGroup groups a job with its retries.
	LabelJobGroup = "forgeai.wutz.dev/job-group"
	// LabelInstanceType identifies the instance type.
	LabelInstanceType = "forgeai.wutz.dev/instance-type"

	// AnnotationMaxRetries is the maximum retry count.
	AnnotationMaxRetries = "forgeai.wutz.dev/max-retries"
	// AnnotationRetryOn is the retry policy (Infra or Any).
	AnnotationRetryOn = "forgeai.wutz.dev/retry-on"
	// AnnotationRetry is the current retry attempt number.
	AnnotationRetry = "forgeai.wutz.dev/retry"
	// AnnotationRetriedAs points to the next retry job name.
	AnnotationRetriedAs = "forgeai.wutz.dev/retried-as"
	// AnnotationFailureReason stores the classified failure reason.
	AnnotationFailureReason = "forgeai.wutz.dev/failure-reason"
	// AnnotationFailureMessage is the failure message.
	AnnotationFailureMessage = "forgeai.wutz.dev/failure-message"
	// AnnotationStopped marks a TrainJob the user stopped.
	AnnotationStopped = "forgeai.wutz.dev/stopped"

	// LiteralTrue is the string "true".
	LiteralTrue = "true"
)
