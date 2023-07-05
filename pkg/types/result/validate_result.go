package result

import (
	"github.com/kluctl/kluctl/v2/pkg/types/k8s"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type ValidateResultEntry struct {
	Ref        k8s.ObjectRef `json:"ref"`
	Annotation string        `json:"annotation"`
	Message    string        `json:"message"`
}

type ValidateResult struct {
	Id               string                `json:"id"`
	ProjectKey       ProjectKey            `json:"projectKey"`
	TargetKey        TargetKey             `json:"targetKey"`
	KluctlDeployment *KluctlDeploymentInfo `json:"kluctlDeployment,omitempty"`
	StartTime        metav1.Time           `json:"startTime"`
	EndTime          metav1.Time           `json:"endTime"`
	Ready            bool                  `json:"ready"`
	Warnings         []DeploymentError     `json:"warnings,omitempty"`
	Errors           []DeploymentError     `json:"errors,omitempty"`
	Results          []ValidateResultEntry `json:"results,omitempty"`

	Drift []ChangedObject `json:"drift,omitempty"`
}

type ValidateResultSummary struct {
	Id         string      `json:"id"`
	ProjectKey ProjectKey  `json:"projectKey"`
	TargetKey  TargetKey   `json:"targetKey"`
	StartTime  metav1.Time `json:"startTime"`
	EndTime    metav1.Time `json:"endTime"`
	Ready      bool        `json:"ready"`

	Warnings int `json:"warnings"`
	Errors   int `json:"errors"`
	Results  int `json:"results"`
}

func (vr *ValidateResult) BuildSummary() ValidateResultSummary {
	return ValidateResultSummary{
		Id:         vr.Id,
		ProjectKey: vr.ProjectKey,
		TargetKey:  vr.TargetKey,
		StartTime:  vr.StartTime,
		EndTime:    vr.EndTime,
		Ready:      vr.Ready,
		Warnings:   len(vr.Warnings),
		Errors:     len(vr.Errors),
		Results:    len(vr.Results),
	}
}
