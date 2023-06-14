/*
Copyright 2023.

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

package v1beta1

import (
	"github.com/kluctl/kluctl/v2/pkg/types"
	"github.com/kluctl/kluctl/v2/pkg/types/result"
	"github.com/kluctl/kluctl/v2/pkg/yaml"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"time"
)

const (
	KluctlDeploymentKind      = "KluctlDeployment"
	KluctlDeploymentFinalizer = "finalizers.gitops.kluctl.io"
	MaxConditionMessageLength = 20000

	KluctlDeployModeFull   = "full-deploy"
	KluctlDeployPokeImages = "poke-images"

	KluctlRequestReconcileAnnotation = "kluctl.io/request-reconcile"
	KluctlRequestDeployAnnotation    = "kluctl.io/request-deploy"
)

type KluctlDeploymentSpec struct {
	// Specifies the project source location
	Source ProjectSource `json:"source"`

	// Decrypt Kubernetes secrets before applying them on the cluster.
	// +optional
	Decryption *Decryption `json:"decryption,omitempty"`

	// The interval at which to reconcile the KluctlDeployment.
	// Reconciliation means that the deployment is fully rendered and only deployed when the result changes compared
	// to the last deployment.
	// To override this behavior, set the DeployInterval value.
	// +required
	// +kubebuilder:validation:Type=string
	// +kubebuilder:validation:Pattern="^([0-9]+(\\.[0-9]+)?(ms|s|m|h))+$"
	Interval metav1.Duration `json:"interval"`

	// The interval at which to retry a previously failed reconciliation.
	// When not specified, the controller uses the Interval
	// value to retry failures.
	// +optional
	// +kubebuilder:validation:Type=string
	// +kubebuilder:validation:Pattern="^([0-9]+(\\.[0-9]+)?(ms|s|m|h))+$"
	RetryInterval *metav1.Duration `json:"retryInterval,omitempty"`

	// DeployInterval specifies the interval at which to deploy the KluctlDeployment, even in cases the rendered
	// result does not change.
	// +optional
	DeployInterval *SafeDuration `json:"deployInterval,omitempty"`

	// ValidateInterval specifies the interval at which to validate the KluctlDeployment.
	// Validation is performed the same way as with 'kluctl validate -t <target>'.
	// Defaults to the same value as specified in Interval.
	// Validate is also performed whenever a deployment is performed, independent of the value of ValidateInterval
	// +optional
	ValidateInterval *SafeDuration `json:"validateInterval,omitempty"`

	// Timeout for all operations.
	// Defaults to 'Interval' duration.
	// +optional
	// +kubebuilder:validation:Type=string
	// +kubebuilder:validation:Pattern="^([0-9]+(\\.[0-9]+)?(ms|s|m|h))+$"
	Timeout *metav1.Duration `json:"timeout,omitempty"`

	// This flag tells the controller to suspend subsequent kluctl executions,
	// it does not apply to already started executions. Defaults to false.
	// +optional
	Suspend bool `json:"suspend,omitempty"`

	// HelmCredentials is a list of Helm credentials used when non pre-pulled Helm Charts are used inside a
	// Kluctl deployment.
	// +optional
	HelmCredentials []HelmCredentials `json:"helmCredentials,omitempty"`

	// The name of the Kubernetes service account to use while deploying.
	// If not specified, the default service account is used.
	// +optional
	ServiceAccountName string `json:"serviceAccountName,omitempty"`

	// The KubeConfig for deploying to the target cluster.
	// Specifies the kubeconfig to be used when invoking kluctl. Contexts in this kubeconfig must match
	// the context found in the kluctl target. As an alternative, specify the context to be used via 'context'
	// +optional
	KubeConfig *KubeConfig `json:"kubeConfig"`

	// Target specifies the kluctl target to deploy. If not specified, an empty target is used that has no name and no
	// context. Use 'TargetName' and 'Context' to specify the name and context in that case.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=63
	// +optional
	Target *string `json:"target,omitempty"`

	// TargetNameOverride sets or overrides the target name. This is especially useful when deployment without a target.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=63
	// +optional
	TargetNameOverride *string `json:"targetNameOverride,omitempty"`

	// If specified, overrides the context to be used. This will effectively make kluctl ignore the context specified
	// in the target.
	// +optional
	Context *string `json:"context,omitempty"`

	// Args specifies dynamic target args.
	// +optional
	// +kubebuilder:pruning:PreserveUnknownFields
	Args runtime.RawExtension `json:"args,omitempty"`

	// Images contains a list of fixed image overrides.
	// Equivalent to using '--fixed-images-file' when calling kluctl.
	// +optional
	Images []types.FixedImage `json:"images,omitempty"`

	// DryRun instructs kluctl to run everything in dry-run mode.
	// Equivalent to using '--dry-run' when calling kluctl.
	// +kubebuilder:default:=false
	// +optional
	DryRun bool `json:"dryRun,omitempty"`

	// NoWait instructs kluctl to not wait for any resources to become ready, including hooks.
	// Equivalent to using '--no-wait' when calling kluctl.
	// +kubebuilder:default:=false
	// +optional
	NoWait bool `json:"noWait,omitempty"`

	// ForceApply instructs kluctl to force-apply in case of SSA conflicts.
	// Equivalent to using '--force-apply' when calling kluctl.
	// +kubebuilder:default:=false
	// +optional
	ForceApply bool `json:"forceApply,omitempty"`

	// ReplaceOnError instructs kluctl to replace resources on error.
	// Equivalent to using '--replace-on-error' when calling kluctl.
	// +kubebuilder:default:=false
	// +optional
	ReplaceOnError bool `json:"replaceOnError,omitempty"`

	// ForceReplaceOnError instructs kluctl to force-replace resources in case a normal replace fails.
	// Equivalent to using '--force-replace-on-error' when calling kluctl.
	// +kubebuilder:default:=false
	// +optional
	ForceReplaceOnError bool `json:"forceReplaceOnError,omitempty"`

	// ForceReplaceOnError instructs kluctl to abort deployments immediately when something fails.
	// Equivalent to using '--abort-on-error' when calling kluctl.
	// +kubebuilder:default:=false
	// +optional
	AbortOnError bool `json:"abortOnError,omitempty"`

	// IncludeTags instructs kluctl to only include deployments with given tags.
	// Equivalent to using '--include-tag' when calling kluctl.
	// +optional
	IncludeTags []string `json:"includeTags,omitempty"`

	// ExcludeTags instructs kluctl to exclude deployments with given tags.
	// Equivalent to using '--exclude-tag' when calling kluctl.
	// +optional
	ExcludeTags []string `json:"excludeTags,omitempty"`

	// IncludeDeploymentDirs instructs kluctl to only include deployments with the given dir.
	// Equivalent to using '--include-deployment-dir' when calling kluctl.
	// +optional
	IncludeDeploymentDirs []string `json:"includeDeploymentDirs,omitempty"`

	// ExcludeDeploymentDirs instructs kluctl to exclude deployments with the given dir.
	// Equivalent to using '--exclude-deployment-dir' when calling kluctl.
	// +optional
	ExcludeDeploymentDirs []string `json:"excludeDeploymentDirs,omitempty"`

	// DeployMode specifies what deploy mode should be used.
	// The options 'full-deploy' and 'poke-images' are supported.
	// With the 'poke-images' option, only images are patched into the target without performing a full deployment.
	// +kubebuilder:default:=full-deploy
	// +kubebuilder:validation:Enum=full-deploy;poke-images
	// +optional
	DeployMode string `json:"deployMode,omitempty"`

	// Validate enables validation after deploying
	// +kubebuilder:default:=true
	// +optional
	Validate bool `json:"validate"`

	// Prune enables pruning after deploying.
	// +kubebuilder:default:=false
	// +optional
	Prune bool `json:"prune,omitempty"`

	// Delete enables deletion of the specified target when the KluctlDeployment object gets deleted.
	// +kubebuilder:default:=false
	// +optional
	Delete bool `json:"delete,omitempty"`
}

// GetRetryInterval returns the retry interval
func (in KluctlDeploymentSpec) GetRetryInterval() time.Duration {
	if in.RetryInterval != nil {
		return in.RetryInterval.Duration
	}
	return in.Interval.Duration
}

type ProjectSource struct {
	// Url specifies the Git url where the project source is located
	// +required
	URL types.GitUrl `json:"url"`

	// Ref specifies the branch, tag or commit that should be used. If omitted, the default branch of the repo is used.
	// +optional
	Ref *GitRef `json:"ref,omitempty"`

	// Path specifies the sub-directory to be used as project directory
	// +optional
	Path string `json:"path,omitempty"`

	// SecretRef specifies the Secret containing authentication credentials for
	// the git repository.
	// For HTTPS repositories the Secret must contain 'username' and 'password'
	// fields.
	// For SSH repositories the Secret must contain 'identity'
	// and 'known_hosts' fields.
	// +optional
	SecretRef *LocalObjectReference `json:"secretRef,omitempty"`
}

// Decryption defines how decryption is handled for Kubernetes manifests.
type Decryption struct {
	// Provider is the name of the decryption engine.
	// +kubebuilder:validation:Enum=sops
	// +required
	Provider string `json:"provider"`

	// The secret name containing the private OpenPGP keys used for decryption.
	// +optional
	SecretRef *LocalObjectReference `json:"secretRef,omitempty"`

	// ServiceAccount specifies the service account used to authenticate against cloud providers.
	// This is currently only usable for AWS KMS keys. The specified service account will be used to authenticate to AWS
	// by signing a token in an IRSA compliant way.
	// +optional
	ServiceAccount string `json:"serviceAccount,omitempty"`
}

type HelmCredentials struct {
	// SecretRef holds the name of a secret that contains the Helm credentials.
	// The secret must either contain the fields `credentialsId` which refers to the credentialsId
	// found in https://kluctl.io/docs/kluctl/reference/deployments/helm/#private-chart-repositories or an `url` used
	// to match the credentials found in Kluctl projects helm-chart.yaml files.
	// The secret can either container basic authentication credentials via `username` and `password` or
	// TLS authentication via `certFile` and `keyFile`. `caFile` can be specified to override the CA to use while
	// contacting the repository.
	// The secret can also contain `insecureSkipTlsVerify: "true"`, which will disable TLS verification.
	// `passCredentialsAll: "true"` can be specified to make the controller pass credentials to all requests, even if
	// the hostname changes in-between.
	// +required
	SecretRef LocalObjectReference `json:"secretRef,omitempty"`
}

// KubeConfig references a Kubernetes secret that contains a kubeconfig file.
type KubeConfig struct {
	// SecretRef holds the name of a secret that contains a key with
	// the kubeconfig file as the value. If no key is set, the key will default
	// to 'value'. The secret must be in the same namespace as
	// the Kustomization.
	// It is recommended that the kubeconfig is self-contained, and the secret
	// is regularly updated if credentials such as a cloud-access-token expire.
	// Cloud specific `cmd-path` auth helpers will not function without adding
	// binaries and credentials to the Pod that is responsible for reconciling
	// the KluctlDeployment.
	// +required
	SecretRef SecretKeyReference `json:"secretRef,omitempty"`
}

// KluctlDeploymentStatus defines the observed state of KluctlDeployment
type KluctlDeploymentStatus struct {
	// LastHandledReconcileAt holds the value of the most recent
	// reconcile request value, so a change of the annotation value
	// can be detected.
	// +optional
	LastHandledReconcileAt string `json:"lastHandledReconcileAt,omitempty"`

	// +optional
	LastHandledDeployAt string `json:"LastHandledDeployAt,omitempty"`

	// ObservedGeneration is the last reconciled generation.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// ObservedCommit is the last commit observed
	ObservedCommit string `json:"observedCommit,omitempty"`

	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// +optional
	ProjectKey *result.ProjectKey `json:"projectKey,omitempty"`

	// +optional
	TargetKey *result.TargetKey `json:"targetKey,omitempty"`

	// +optional
	LastObjectsHash string `json:"lastObjectsHash,omitempty"`

	// +optional
	LastDeployError string `json:"lastDeployError,omitempty"`
	// +optional
	LastValidateError string `json:"lastValidateError,omitempty"`

	// LastDeployResult is the result of the last deploy command
	// +optional
	// +kubebuilder:pruning:PreserveUnknownFields
	LastDeployResult *runtime.RawExtension `json:"lastDeployResult,omitempty"`

	// LastValidateResult is the result of the last validate command
	// +optional
	LastValidateResult *runtime.RawExtension `json:"lastValidateResult,omitempty"`
}

func (s *KluctlDeploymentStatus) SetLastDeployResult(crs *result.CommandResultSummary, err error) error {
	s.LastDeployError = ""
	if err != nil {
		s.LastDeployError = err.Error()
	}
	if crs == nil {
		s.LastDeployResult = nil
	} else {
		b, err := yaml.WriteJsonString(crs)
		if err != nil {
			return err
		}
		s.LastDeployResult = &runtime.RawExtension{Raw: []byte(b)}
	}
	return nil
}

func (s *KluctlDeploymentStatus) SetLastValidateResult(crs *result.ValidateResult, err error) error {
	s.LastValidateError = ""
	if err != nil {
		s.LastValidateError = err.Error()
	}
	if crs == nil {
		s.LastValidateResult = nil
	} else {
		b, err := yaml.WriteJsonString(crs)
		if err != nil {
			return err
		}
		s.LastValidateResult = &runtime.RawExtension{Raw: []byte(b)}
	}
	return nil
}

func (s *KluctlDeploymentStatus) GetLastDeployResult() (*result.CommandResultSummary, error) {
	if s.LastDeployResult == nil {
		return nil, nil
	}
	var ret result.CommandResultSummary
	err := yaml.ReadYamlBytes(s.LastDeployResult.Raw, &ret)
	if err != nil {
		return nil, err
	}
	return &ret, nil
}

func (s *KluctlDeploymentStatus) GetLastValidateResult() (*result.ValidateResult, error) {
	if s.LastValidateResult == nil {
		return nil, nil
	}
	var ret result.ValidateResult
	err := yaml.ReadYamlBytes(s.LastValidateResult.Raw, &ret)
	if err != nil {
		return nil, err
	}
	return &ret, nil
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:printcolumn:name="DryRun",type="boolean",JSONPath=".spec.dryRun",description=""
//+kubebuilder:printcolumn:name="Deployed",type="date",JSONPath=".status.lastDeployResult.commandInfo.endTime",description=""
//+kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type==\"Ready\")].status",description=""
//+kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.conditions[?(@.type==\"Ready\")].message",description=""
//+kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp",description=""

// KluctlDeployment is the Schema for the kluctldeployments API
type KluctlDeployment struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   KluctlDeploymentSpec   `json:"spec,omitempty"`
	Status KluctlDeploymentStatus `json:"status,omitempty"`
}

// GetConditions returns the status conditions of the object.
func (in *KluctlDeployment) GetConditions() []metav1.Condition {
	return in.Status.Conditions
}

// SetConditions sets the status conditions on the object.
func (in *KluctlDeployment) SetConditions(conditions []metav1.Condition) {
	in.Status.Conditions = conditions
}

//+kubebuilder:object:root=true

// KluctlDeploymentList contains a list of KluctlDeployment
type KluctlDeploymentList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []KluctlDeployment `json:"items"`
}

func init() {
	SchemeBuilder.Register(&KluctlDeployment{}, &KluctlDeploymentList{})
}
