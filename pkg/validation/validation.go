package validation

import (
	"fmt"
	"github.com/kluctl/kluctl/v2/pkg/k8s"
	"github.com/kluctl/kluctl/v2/pkg/types/result"
	"github.com/kluctl/kluctl/v2/pkg/utils"
	"github.com/kluctl/kluctl/v2/pkg/utils/uo"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var resultAnnotation = regexp.MustCompile("^validate-result.kluctl.io/.*")

type validationFailed struct {
}

func (err *validationFailed) Error() string {
	return "validation failed"
}

type condition struct {
	status  string
	reason  string
	message string
}

func (c condition) getMessage(def string) string {
	if c.message == "" {
		return def
	}
	return c.message
}

type errorReaction int

const (
	reactIgnore errorReaction = iota
	reactError
	reactWarning
	reactNotReady
)

func ValidateObject(k *k8s.K8sCluster, o *uo.UnstructuredObject, notReadyIsError bool, forceStatusRequired bool) (ret result.ValidateResult) {
	ref := o.GetK8sRef()

	// We assume all is good in case no validation is performed
	ret.Ready = true

	if utils.ParseBoolOrFalse(o.GetK8sAnnotation("kluctl.io/validate-ignore")) {
		return
	}

	defer func() {
		if r := recover(); r != nil {
			if _, ok := r.(*validationFailed); ok {
				// all good
			} else if e, ok := r.(error); ok {
				err := fmt.Errorf("panic in ValidateObject: %w", e)
				ret.Errors = append(ret.Errors, result.DeploymentError{Ref: ref, Message: err.Error()})
			} else {
				err := fmt.Errorf("panic in ValidateObject: %v", e)
				ret.Errors = append(ret.Errors, result.DeploymentError{Ref: ref, Message: err.Error()})
			}
			ret.Ready = false
		}
	}()

	for k, v := range o.GetK8sAnnotationsWithRegex(resultAnnotation) {
		ret.Results = append(ret.Results, result.ValidateResultEntry{
			Ref:        ref,
			Annotation: k,
			Message:    v,
		})
	}

	addError := func(message string) {
		ret.Errors = append(ret.Errors, result.DeploymentError{
			Ref:     ref,
			Message: message,
		})
		ret.Ready = false
	}
	addWarning := func(message string) {
		ret.Warnings = append(ret.Warnings, result.DeploymentError{
			Ref:     ref,
			Message: message,
		})
	}
	addNotReady := func(message string) {
		if notReadyIsError {
			addError(message)
		} else {
			addWarning(message)
		}
		ret.Ready = false
	}

	reactToError := func(err error, er errorReaction, doRaise bool) {
		if err == nil {
			return
		}
		if er == reactError {
			addError(err.Error())
		} else if er == reactWarning {
			addWarning(err.Error())
		} else if er == reactNotReady {
			addNotReady(err.Error())
		}
		if doRaise {
			panic(&validationFailed{})
		}
	}

	status, _, _ := o.GetNestedObject("status")
	if status == nil {
		if forceStatusRequired {
			addNotReady("no status available yet")
			return
		}
		if k == nil {
			// can't really say anything...
			return
		}
		s, err := k.Resources.GetSchemaForGVK(ref.GroupVersionKind())
		if err != nil && !errors.IsNotFound(err) {
			addError(err.Error())
			return
		}
		if s == nil {
			return
		} else {
			_, ok, _ := s.GetNestedObject("properties", "status")
			if !ok {
				// it has no status, so all is good
				return
			}

			age := time.Now().Sub(o.GetK8sCreationTime())
			if age > 15*time.Second {
				// TODO this is a hack for CRDs that pretend that a status should be there but the corresponding
				// controllers/operators don't set it for whatever reason (e.g. cilium)
				return
			}

			addNotReady("no status available yet")
			return
		}
	}

	findConditions := func(typ string, er errorReaction, doRaise bool) []condition {
		var ret []condition
		l, ok, _ := status.GetNestedObjectList("conditions")
		if ok {
			for _, c := range l {
				t, _, _ := c.GetNestedString("type")
				status, _, _ := c.GetNestedString("status")
				reason, _, _ := c.GetNestedString("reason")
				message, _, _ := c.GetNestedString("message")
				if t == typ {
					ret = append(ret, condition{
						status:  status,
						reason:  reason,
						message: message,
					})
				}
			}
		}
		if len(ret) == 0 {
			reactToError(fmt.Errorf("%s condition not in status", typ), er, doRaise)
		}
		return ret
	}

	getCondition := func(typ string, er errorReaction, doRaise bool) condition {
		c := findConditions(typ, er, doRaise)
		if len(c) == 0 {
			return condition{}
		}
		if len(c) != 1 {
			err := fmt.Errorf("%s condition found more then once", typ)
			addError(err.Error())
			if doRaise {
				panic(&validationFailed{})
			}
		}
		return c[0]
	}
	getStatusField := func(field string, er errorReaction, doRaise bool, def interface{}) interface{} {
		v, ok, _ := status.GetNestedField(field)
		if !ok {
			reactToError(fmt.Errorf("%s field not in status or empty", field), er, doRaise)
		}
		if !ok {
			return def
		}
		return v
	}
	getStatusFieldStr := func(field string, er errorReaction, doRaise bool, def string) string {
		v := getStatusField(field, er, doRaise, def)
		if s, ok := v.(string); ok {
			return s
		} else {
			err := fmt.Errorf("%s field is not a string", field)
			addError(err.Error())
			if doRaise {
				panic(&validationFailed{})
			}
		}
		return def
	}
	getStatusFieldInt := func(field string, er errorReaction, doRaise bool, def int64) int64 {
		v := getStatusField(field, er, doRaise, def)
		if i, ok := v.(int64); ok {
			return i
		} else if i, ok := v.(uint64); ok {
			return int64(i)
		} else if i, ok := v.(int); ok {
			return int64(i)
		} else {
			err := fmt.Errorf("%s field is not an int", field)
			reactToError(err, er, doRaise)
		}
		return def
	}
	parseIntOrPercent := func(v interface{}) (int64, bool, error) {
		if i, ok := v.(int64); ok {
			return i, false, nil
		}
		if i, ok := v.(uint64); ok {
			return int64(i), false, nil
		}
		if i, ok := v.(int); ok {
			return int64(i), false, nil
		}
		if s, ok := v.(string); ok {
			s = strings.ReplaceAll(s, "%", "")
			i, err := strconv.ParseInt(s, 10, 32)
			if err != nil {
				return 0, false, err
			}
			return i, true, nil
		}
		return 0, false, fmt.Errorf("don't know how to parse %v", v)
	}
	valueFromIntOrPercent := func(v interface{}, total int64) (int64, error) {
		i, isPercent, err := parseIntOrPercent(v)
		if err != nil {
			return 0, err
		}
		if isPercent {
			return int64((float32(i) * float32(total)) / 100), nil
		}
		return i, nil
	}

	observedGeneration := getStatusFieldInt("observedGeneration", reactIgnore, false, -1)
	if observedGeneration != -1 && observedGeneration != o.GetK8sGeneration() {
		addNotReady("Waiting for reconciliation")
		return
	}

	switch o.GetK8sGVK().GroupKind() {
	case schema.GroupKind{Group: "", Kind: "Pod"}:
		containerStatuses, _, err := status.GetNestedObjectList("containerStatuses")
		reactToError(err, reactError, true)
		for _, cs := range containerStatuses {
			containerName, _, err := cs.GetNestedString("name")
			reactToError(err, reactError, true)
			terminateReason, ok, err := cs.GetNestedString("state", "terminated", "reason")
			reactToError(err, reactError, true)
			if ok && terminateReason == "Error" {
				addError(fmt.Sprintf("container %s exited with error", containerName))
			}
		}

		rc := getCondition("Ready", reactNotReady, false)
		if rc.status == "False" && rc.reason == "PodCompleted" {
			// pod exited
			return
		}
		// pod is still running, so it is not ready
		addNotReady("Not ready")
	case schema.GroupKind{Group: "batch", Kind: "Job"}:
		c := getCondition("Failed", reactIgnore, false)
		if c.status == "True" {
			addError(c.getMessage("Failed"))
		} else {
			c = getCondition("Complete", reactIgnore, false)
			if c.status != "True" {
				addNotReady(c.getMessage("Not completed"))
			}
		}
	case schema.GroupKind{Group: "apps", Kind: "Deployment"}:
		specReplicas, ok, _ := o.GetNestedInt("spec", "replicas")
		if ok && specReplicas != 0 {
			readyReplicas := getStatusFieldInt("readyReplicas", reactNotReady, true, 0)
			replicas := getStatusFieldInt("replicas", reactNotReady, true, 0)
			if readyReplicas < replicas {
				addNotReady(fmt.Sprintf("readyReplicas (%d) is less then replicas (%d)", readyReplicas, replicas))
			}
		}
	case schema.GroupKind{Group: "", Kind: "PersistentVolumeClaim"}:
		phase := getStatusFieldStr("phase", reactNotReady, true, "")
		if phase != "Bound" {
			addNotReady("Volume is not bound")
		}
	case schema.GroupKind{Group: "", Kind: "Service"}:
		svcType, _, _ := o.GetNestedString("spec", "type")
		if svcType != "ExternalName" {
			clusterIP, _, _ := o.GetNestedString("spec", "clusterIP")
			if clusterIP == "" {
				addError("Service does not have a cluster IP")
			} else if svcType == "LoadBalancer" {
				externalIPs, _, _ := o.GetNestedList("spec", "externalIPs")
				if len(externalIPs) == 0 {
					ingress, _, _ := status.GetNestedList("loadBalancer", "ingress")
					if len(ingress) == 0 {
						addNotReady("Not ready")
					}
				}
			}
		}
	case schema.GroupKind{Group: "apps", Kind: "DaemonSet"}:
		updateStrategyType, _, _ := o.GetNestedString("spec", "updateStrategy", "type")
		if updateStrategyType == "RollingUpdate" {
			updatedNumberScheduled := getStatusFieldInt("updatedNumberScheduled", reactNotReady, true, 0)
			desiredNumberScheduled := getStatusFieldInt("desiredNumberScheduled", reactNotReady, true, 0)
			if updatedNumberScheduled != desiredNumberScheduled {
				addNotReady(fmt.Sprintf("DaemonSet is not ready. %d out of %d expected pods have been scheduled", updatedNumberScheduled, desiredNumberScheduled))
			} else {
				maxUnavailableI, _, _ := o.GetNestedField("spec", "updateStrategy", "maxUnavailable")
				if maxUnavailableI == nil {
					maxUnavailableI = 1
				}
				maxUnavailable, err := valueFromIntOrPercent(maxUnavailableI, desiredNumberScheduled)
				if err != nil {
					maxUnavailable = desiredNumberScheduled
				}
				expectedReady := desiredNumberScheduled - maxUnavailable
				numberReady := getStatusFieldInt("numberReady", reactNotReady, true, 0)
				if numberReady < expectedReady {
					addNotReady(fmt.Sprintf("DaemonSet is not ready. %d out of %d expected pods are ready", numberReady, expectedReady))
				}
			}
		}
	case schema.GroupKind{Group: "apiextensions.k8s.io", Kind: "CustomResourceDefinition"}:
		// This is based on how Helm check for ready CRDs.
		// See https://github.com/helm/helm/blob/249d1b5fb98541f5fb89ab11019b6060d6b169f1/pkg/kube/ready.go#L342
		c := getCondition("Established", reactIgnore, false)
		if c.status != "True" {
			c = getCondition("NamesAccepted", reactNotReady, true)
			if c.status != "False" {
				addNotReady("CRD is not ready")
			}
		}
	case schema.GroupKind{Group: "apps", Kind: "StatefulSet"}:
		updateStrategyType, _, _ := o.GetNestedString("spec", "updateStrategy", "type")
		if updateStrategyType == "RollingUpdate" {
			partition, _, _ := o.GetNestedInt("spec", "updateStrategy", "rollingUpdate", "partition")
			replicas, ok, _ := o.GetNestedInt("spec", "replicas")
			if !ok {
				replicas = 1
			}
			updatedReplicas := getStatusFieldInt("updatedReplicas", reactNotReady, true, 0)
			expectedReplicas := replicas - partition
			if updatedReplicas != expectedReplicas {
				addNotReady(fmt.Sprintf("StatefulSet is not ready. %d out of %d expected pods have been scheduled", updatedReplicas, expectedReplicas))
			} else {
				readyReplicas := getStatusFieldInt("readyReplicas", reactNotReady, true, 0)
				if readyReplicas != replicas {
					addNotReady(fmt.Sprintf("StatefulSet is not ready. %d out of %d expected pods are ready", readyReplicas, replicas))
				}
			}
		}
	case schema.GroupKind{Group: "cluster.x-k8s.io", Kind: "MachineDeployment"}:
		c := getCondition("Ready", reactNotReady, true)
		if c.status != "True" {
			addNotReady(c.getMessage("Not ready"))
		}
		c = getCondition("Available", reactNotReady, true)
		if c.status != "True" {
			addNotReady(c.getMessage("Not ready"))
		}

		readyReplicas := getStatusFieldInt("readyReplicas", reactNotReady, true, 0)
		replicas := getStatusFieldInt("replicas", reactNotReady, true, 0)
		unavailableReplicas := getStatusFieldInt("unavailableReplicas", reactIgnore, false, 0)
		if readyReplicas < replicas {
			addNotReady(fmt.Sprintf("readyReplicas (%d) is less then replicas (%d)", readyReplicas, replicas))
		}
		if unavailableReplicas != 0 {
			addNotReady(fmt.Sprintf("unavailableReplicas (%d) != 0", unavailableReplicas))
		}
	}
	return
}
