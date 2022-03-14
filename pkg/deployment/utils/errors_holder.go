package utils

import (
	"errors"
	"fmt"
	k8s2 "github.com/codablock/kluctl/pkg/k8s"
	"github.com/codablock/kluctl/pkg/types"
	"github.com/codablock/kluctl/pkg/types/k8s"
	"github.com/codablock/kluctl/pkg/utils"
	"sync"
)

type DeploymentErrorsAndWarnings struct {
	errors   map[k8s.ObjectRef]map[types.DeploymentError]bool
	warnings map[k8s.ObjectRef]map[types.DeploymentError]bool
	mutex    sync.Mutex
}

func NewDeploymentErrorsAndWarnings() *DeploymentErrorsAndWarnings {
	dew := &DeploymentErrorsAndWarnings{}
	dew.init()
	return dew
}

func (dew *DeploymentErrorsAndWarnings) init() {
	dew.mutex.Lock()
	defer dew.mutex.Unlock()
	dew.warnings = map[k8s.ObjectRef]map[types.DeploymentError]bool{}
	dew.errors = map[k8s.ObjectRef]map[types.DeploymentError]bool{}
}

func (dew *DeploymentErrorsAndWarnings) AddWarning(ref k8s.ObjectRef, warning error) {
	de := types.DeploymentError{
		Ref:   ref,
		Error: warning.Error(),
	}
	dew.mutex.Lock()
	defer dew.mutex.Unlock()
	m, ok := dew.warnings[ref]
	if !ok {
		m = make(map[types.DeploymentError]bool)
		dew.warnings[ref] = m
	}
	m[de] = true
}

func (dew *DeploymentErrorsAndWarnings) AddError(ref k8s.ObjectRef, err error) {
	de := types.DeploymentError{
		Ref:   ref,
		Error: err.Error(),
	}
	dew.mutex.Lock()
	defer dew.mutex.Unlock()
	m, ok := dew.errors[ref]
	if !ok {
		m = make(map[types.DeploymentError]bool)
		dew.errors[ref] = m
	}
	m[de] = true
}

func (dew *DeploymentErrorsAndWarnings) AddApiWarnings(ref k8s.ObjectRef, warnings []k8s2.ApiWarning) {
	for _, w := range warnings {
		dew.AddWarning(ref, fmt.Errorf(w.Text))
	}
}

func (dew *DeploymentErrorsAndWarnings) HadError(ref k8s.ObjectRef) bool {
	dew.mutex.Lock()
	defer dew.mutex.Unlock()
	_, ok := dew.errors[ref]
	return ok
}

func (dew *DeploymentErrorsAndWarnings) GetErrorsList() []types.DeploymentError {
	dew.mutex.Lock()
	defer dew.mutex.Unlock()
	var ret []types.DeploymentError
	for _, m := range dew.errors {
		for e := range m {
			ret = append(ret, e)
		}
	}
	return ret
}

func (dew *DeploymentErrorsAndWarnings) GetWarningsList() []types.DeploymentError {
	dew.mutex.Lock()
	defer dew.mutex.Unlock()
	var ret []types.DeploymentError
	for _, m := range dew.warnings {
		for e := range m {
			ret = append(ret, e)
		}
	}
	return ret
}

func (dew *DeploymentErrorsAndWarnings) getPlainErrorsList() []error {
	var ret []error
	for _, e := range dew.GetErrorsList() {
		ret = append(ret, errors.New(e.Error))
	}
	return ret
}

func (dew *DeploymentErrorsAndWarnings) GetMultiError() error {
	l := dew.getPlainErrorsList()
	if len(l) == 0 {
		return nil
	}
	return utils.NewErrorList(l)
}
