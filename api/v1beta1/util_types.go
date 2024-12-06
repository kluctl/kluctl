package v1beta1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type LocalObjectReference struct {
	// Name of the referent.
	// +required
	Name string `json:"name"`
}

// SecretKeyReference contains enough information to locate the referenced Kubernetes Secret object in the same
// namespace. Optionally a key can be specified.
// Use this type instead of core/v1 SecretKeySelector when the Key is optional and the Optional field is not
// applicable.
type SecretKeyReference struct {
	// Name of the Secret.
	// +required
	Name string `json:"name"`

	// Key in the Secret, when not specified an implementation-specific default key is used.
	// +optional
	Key string `json:"key,omitempty"`
}

// +kubebuilder:validation:Type=string
// +kubebuilder:validation:Pattern="^(([0-9]+(\\.[0-9]+)?(ms|s|m|h))+)"
type SafeDuration struct {
	metav1.Duration
}
