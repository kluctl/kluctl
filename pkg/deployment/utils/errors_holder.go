package utils

import (
	"errors"
	"fmt"
	"github.com/hashicorp/go-multierror"
	k8s2 "github.com/kluctl/kluctl/v2/pkg/k8s"
	"github.com/kluctl/kluctl/v2/pkg/types/k8s"
	"github.com/kluctl/kluctl/v2/pkg/types/result"
	"sync"
)

type DeploymentErrorsAndWarnings struct {
	errors   map[k8s.ObjectRef]map[result.DeploymentError]bool
	warnings map[k8s.ObjectRef]map[result.DeploymentError]bool
	mutex    sync.Mutex
}

func NewDeploymentErrorsAndWarnings() *DeploymentErrorsAndWarnings {
	dew := &DeploymentErrorsAndWarnings{}
	dew.Init()
	return dew
}

func (dew *DeploymentErrorsAndWarnings) Init() {
	dew.mutex.Lock()
	defer dew.mutex.Unlock()
	dew.warnings = map[k8s.ObjectRef]map[result.DeploymentError]bool{}
	dew.errors = map[k8s.ObjectRef]map[result.DeploymentError]bool{}
}

func (dew *DeploymentErrorsAndWarnings) AddWarning(ref k8s.ObjectRef, warning error) {
	de := result.DeploymentError{
		Ref:     ref,
		Message: warning.Error(),
	}
	dew.mutex.Lock()
	defer dew.mutex.Unlock()
	m, ok := dew.warnings[ref]
	if !ok {
		m = make(map[result.DeploymentError]bool)
		dew.warnings[ref] = m
	}
	m[de] = true
}

func (dew *DeploymentErrorsAndWarnings) AddError(ref k8s.ObjectRef, err error) {
	de := result.DeploymentError{
		Ref:     ref,
		Message: err.Error(),
	}
	dew.mutex.Lock()
	defer dew.mutex.Unlock()
	m, ok := dew.errors[ref]
	if !ok {
		m = make(map[result.DeploymentError]bool)
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

func (dew *DeploymentErrorsAndWarnings) GetErrorsList() []result.DeploymentError {
	dew.mutex.Lock()
	defer dew.mutex.Unlock()
	var ret []result.DeploymentError
	for _, m := range dew.errors {
		for e := range m {
			ret = append(ret, e)
		}
	}
	return ret
}

func (dew *DeploymentErrorsAndWarnings) GetWarningsList() []result.DeploymentError {
	dew.mutex.Lock()
	defer dew.mutex.Unlock()
	var ret []result.DeploymentError
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
		ret = append(ret, errors.New(e.Message))
	}
	return ret
}

func (dew *DeploymentErrorsAndWarnings) GetMultiError() error {
	l := dew.getPlainErrorsList()
	return multierror.Append(nil, l...).ErrorOrNil()
}
