package v1beta1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// ManualRequest is used in json form inside the manual request annotations
type ManualRequest struct {
	// +required
	RequestValue string `json:"requestValue"`

	// +optional
	OverridesPatch *runtime.RawExtension `json:"overridesPatch,omitempty"`
}

type ManualRequestResult struct {
	// +required
	Request ManualRequest `json:"request"`

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
