package result

import (
	"fmt"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type DriftedObject struct {
	BaseObject

	LastResourceVersion string `json:"lastResourceVersion"`
}

type DriftDetectionResult struct {
	Id                  string                `json:"id"`
	ReconcileId         string                `json:"reconcileId"`
	ProjectKey          ProjectKey            `json:"projectKey"`
	TargetKey           TargetKey             `json:"targetKey"`
	KluctlDeployment    *KluctlDeploymentInfo `json:"kluctlDeployment,omitempty"`
	RenderedObjectsHash string                `json:"renderedObjectsHash,omitempty"`
	StartTime           metav1.Time           `json:"startTime"`
	EndTime             metav1.Time           `json:"endTime"`

	Warnings []DeploymentError `json:"warnings,omitempty"`
	Errors   []DeploymentError `json:"errors,omitempty"`
	Objects  []DriftedObject   `json:"objects,omitempty"`
}

func (cr *CommandResult) BuildDriftDetectionResult() *DriftDetectionResult {
	if cr == nil {
		return nil
	}

	ret := &DriftDetectionResult{
		Id:                  cr.Id,
		ReconcileId:         cr.ReconcileId,
		ProjectKey:          cr.ProjectKey,
		TargetKey:           cr.TargetKey,
		KluctlDeployment:    cr.KluctlDeployment,
		RenderedObjectsHash: cr.RenderedObjectsHash,
		StartTime:           cr.Command.StartTime,
		EndTime:             cr.Command.EndTime,
		Errors:              cr.Errors,
		Warnings:            cr.Warnings,
	}
	for _, o := range cr.Objects {
		if !o.New && !o.Orphan && !o.Deleted && len(o.Changes) == 0 {
			continue
		}
		resourceVersion := ""
		ro := o.Applied
		if ro == nil {
			ro = o.Remote
		}
		if ro != nil {
			resourceVersion = ro.GetK8sResourceVersion()
		}

		ret.Objects = append(ret.Objects, DriftedObject{
			BaseObject:          o.BaseObject,
			LastResourceVersion: resourceVersion,
		})
	}

	return ret
}

func (dr *DriftDetectionResult) BuildShortMessage() string {
	ret := ""

	count := func(f func(o DriftedObject) bool) int {
		cnt := 0
		for _, o := range dr.Objects {
			if f(o) {
				cnt++
			}
		}
		return cnt
	}

	add := func(s string, cnt int) {
		if cnt == 0 {
			return
		}
		if ret != "" {
			ret += "/"
		}
		ret += fmt.Sprintf("%d %s", cnt, s)
	}

	countAndAdd := func(s string, cntFun func(o DriftedObject) bool) {
		add(s, count(cntFun))
	}

	if len(dr.Objects) == 0 {
		ret = "no drift"
	} else {
		countAndAdd("new", func(o DriftedObject) bool { return o.New })
		countAndAdd("chg", func(o DriftedObject) bool { return len(o.Changes) != 0 })
		countAndAdd("orp", func(o DriftedObject) bool { return o.Orphan })
		countAndAdd("del", func(o DriftedObject) bool { return o.Deleted })
		add("err", len(dr.Errors))
		add("wrn", len(dr.Warnings))
	}

	return ret
}
