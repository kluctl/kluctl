package result

import (
	"fmt"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type DriftDetectionResult struct {
	Id                  string                `json:"id"`
	ProjectKey          ProjectKey            `json:"projectKey"`
	TargetKey           TargetKey             `json:"targetKey"`
	KluctlDeployment    *KluctlDeploymentInfo `json:"kluctlDeployment,omitempty"`
	RenderedObjectsHash string                `json:"renderedObjectsHash,omitempty"`
	StartTime           metav1.Time           `json:"startTime"`
	EndTime             metav1.Time           `json:"endTime"`

	Warnings []DeploymentError `json:"warnings,omitempty"`
	Errors   []DeploymentError `json:"errors,omitempty"`
	Objects  []BaseObject      `json:"objects,omitempty"`
}

func (cr *CommandResult) BuildDriftDetectionResult() *DriftDetectionResult {
	if cr == nil {
		return nil
	}

	ret := &DriftDetectionResult{
		Id:                  cr.Id,
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
		ret.Objects = append(ret.Objects, o.BaseObject)
	}

	return ret
}

func (dr *DriftDetectionResult) BuildShortMessage() string {
	ret := ""

	count := func(f func(o BaseObject) bool) int {
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

	countAndAdd := func(s string, cntFun func(o BaseObject) bool) {
		add(s, count(cntFun))
	}

	if len(dr.Objects) == 0 {
		ret = "no drift"
	} else {
		countAndAdd("new", func(o BaseObject) bool { return o.New })
		countAndAdd("chg", func(o BaseObject) bool { return len(o.Changes) != 0 })
		countAndAdd("orp", func(o BaseObject) bool { return o.Orphan })
		countAndAdd("del", func(o BaseObject) bool { return o.Deleted })
		add("err", len(dr.Errors))
		add("wrn", len(dr.Warnings))
	}

	return ret
}
