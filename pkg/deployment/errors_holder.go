package deployment

import (
	"errors"
	"fmt"
	k8s2 "github.com/codablock/kluctl/pkg/k8s"
	"github.com/codablock/kluctl/pkg/types"
	"github.com/codablock/kluctl/pkg/types/k8s"
	"github.com/codablock/kluctl/pkg/utils"
	"sync"
)

type deploymentErrorsAndWarnings struct {
	errors   map[k8s.ObjectRef]map[types.DeploymentError]bool
	warnings map[k8s.ObjectRef]map[types.DeploymentError]bool
	mutex    sync.Mutex
}

func NewDeploymentErrorsAndWarnings() *deploymentErrorsAndWarnings {
	dew := &deploymentErrorsAndWarnings{}
	dew.init()
	return dew
}

func (dew *deploymentErrorsAndWarnings) init() {
	dew.mutex.Lock()
	defer dew.mutex.Unlock()
	dew.warnings = map[k8s.ObjectRef]map[types.DeploymentError]bool{}
	dew.errors = map[k8s.ObjectRef]map[types.DeploymentError]bool{}
}

func (dew *deploymentErrorsAndWarnings) addWarning(ref k8s.ObjectRef, warning error) {
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

func (dew *deploymentErrorsAndWarnings) addError(ref k8s.ObjectRef, err error) {
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

func (dew *deploymentErrorsAndWarnings) addApiWarnings(ref k8s.ObjectRef, warnings []k8s2.ApiWarning) {
	for _, w := range warnings {
		dew.addWarning(ref, fmt.Errorf(w.Text))
	}
}

func (dew *deploymentErrorsAndWarnings) hadError(ref k8s.ObjectRef) bool {
	dew.mutex.Lock()
	defer dew.mutex.Unlock()
	_, ok := dew.errors[ref]
	return ok
}

func (dew *deploymentErrorsAndWarnings) getErrorsList() []types.DeploymentError {
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

func (dew *deploymentErrorsAndWarnings) getWarningsList() []types.DeploymentError {
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

func (dew *deploymentErrorsAndWarnings) getPlainErrorsList() []error {
	var ret []error
	for _, e := range dew.getErrorsList() {
		ret = append(ret, errors.New(e.Error))
	}
	return ret
}

func (dew *deploymentErrorsAndWarnings) getMultiError() error {
	l := dew.getPlainErrorsList()
	if len(l) == 0 {
		return nil
	}
	return utils.NewErrorList(l)
}
