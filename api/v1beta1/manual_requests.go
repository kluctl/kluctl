package v1beta1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

type ManualRequestResult struct {
	// +required
	RequestValue string `json:"requestValue"`

	// +required
	StartTime metav1.Time `json:"startTime"`

	// +optional
	EndTime *metav1.Time `json:"endTime,omitempty"`

	// +required
	ReconcileId string `json:"reconcileId"`

	// +optional
	ResultId string `json:"resultId,omitempty"`

	// +optional
	CommandError string `json:"commandError,omitempty"`
}
