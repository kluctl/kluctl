package webui

import (
	"context"
	"github.com/kluctl/kluctl/v2/pkg/deployment/commands"
	"github.com/kluctl/kluctl/v2/pkg/results"
	"github.com/kluctl/kluctl/v2/pkg/status"
	"github.com/kluctl/kluctl/v2/pkg/types/result"
	"sync"
	"time"
)

const shortValidationInterval = time.Second * 15
const longValidationInterval = time.Minute * 1

type validateResultHandler func(key ProjectTargetKey, r *result.ValidateResult)

type validatorManager struct {
	ctx context.Context

	store results.ResultStore
	cam   *clusterAccessorManager

	validators map[ProjectTargetKey]*validatorEntry
	handlers   map[int]validateResultHandler
	nextId     int
	mutex      sync.Mutex
}

type ProjectTargetKey struct {
	Project result.ProjectKey `json:"project"`
	Target  result.TargetKey  `json:"target"`
}

type validatorEntry struct {
	vm             *validatorManager
	key            ProjectTargetKey
	ch             chan bool
	validateResult *result.ValidateResult
	err            error
}

func newValidatorManager(ctx context.Context, store results.ResultStore, cam *clusterAccessorManager) *validatorManager {
	return &validatorManager{
		ctx:        ctx,
		store:      store,
		cam:        cam,
		validators: map[ProjectTargetKey]*validatorEntry{},
		handlers:   map[int]validateResultHandler{},
	}
}

func (vm *validatorManager) start() {
	runOnce := func() {
		summaries, err := vm.store.ListCommandResultSummaries(results.ListCommandResultSummariesOptions{})
		if err != nil {
			return
		}

		found := map[ProjectTargetKey]bool{}
		for _, rs := range summaries {
			k := ProjectTargetKey{
				Project: rs.ProjectKey,
				Target:  rs.TargetKey,
			}
			if _, ok := found[k]; ok {
				continue
			}
			vm.mutex.Lock()
			_, ok := vm.validators[k]
			if !ok {
				v := &validatorEntry{
					vm:  vm,
					key: k,
					ch:  make(chan bool),
				}
				vm.validators[k] = v
				go v.run()
			}
			vm.mutex.Unlock()

			found[k] = true
		}

		vm.mutex.Lock()
		for k, v := range vm.validators {
			if _, ok := found[k]; !ok {
				delete(vm.validators, k)
				close(v.ch)
			}
		}
		vm.mutex.Unlock()
	}

	go func() {
		for {
			runOnce()
			time.Sleep(5 * time.Second)
		}
	}()
}

func (vm *validatorManager) addHandler(h validateResultHandler) func() {
	vm.mutex.Lock()
	defer vm.mutex.Unlock()

	id := vm.nextId
	vm.nextId++

	vm.handlers[id] = h

	for _, v := range vm.validators {
		if v.validateResult != nil {
			h(v.key, v.validateResult)
		}
	}

	return func() {
		vm.mutex.Lock()
		defer vm.mutex.Unlock()
		delete(vm.handlers, id)
	}
}

func (vm *validatorManager) getValidateResult(key ProjectTargetKey) (*result.ValidateResult, error) {
	vm.mutex.Lock()
	defer vm.mutex.Unlock()
	v := vm.validators[key]
	if v == nil {
		return nil, nil
	}
	return v.validateResult, v.err
}

func (vm *validatorManager) validateNow(key ProjectTargetKey) bool {
	vm.mutex.Lock()
	defer vm.mutex.Unlock()
	v := vm.validators[key]
	if v == nil {
		return false
	}
	v.ch <- true
	return true
}

func (v *validatorEntry) run() {
	status.Info(v.vm.ctx, "Started validator for: %v", v.key)
	defer status.Info(v.vm.ctx, "Stopped validator for: %v", v.key)

	for {
		summaries, err := v.vm.store.ListCommandResultSummaries(results.ListCommandResultSummariesOptions{
			ProjectFilter: &v.key.Project,
		})
		if err != nil {
			return
		}

		var targetSummaries []result.CommandResultSummary
		for _, rs := range summaries {
			if rs.TargetKey == v.key.Target {
				targetSummaries = append(targetSummaries, rs)
			}
		}

		longWait := false
		if len(targetSummaries) != 0 {
			longWait = v.runOnce(targetSummaries)
		}

		waitTime := shortValidationInterval
		if longWait {
			waitTime = longValidationInterval
		}

		select {
		case _, ok := <-v.ch:
			if !ok {
				break
			}
			continue
		case <-time.After(waitTime):
		}
	}
}

func (v *validatorEntry) findFullProjectResult(summaries []result.CommandResultSummary) *result.CommandResultSummary {
	for _, summary := range summaries {
		if len(summary.Command.IncludeTags) != 0 || len(summary.Command.ExcludeTags) != 0 {
			continue
		}
		if summary.Command.Command == "deploy" || summary.Command.Command == "diff" {
			return &summary
		}
	}
	return nil
}

func (v *validatorEntry) runOnce(summaries []result.CommandResultSummary) bool {
	s := status.Start(v.vm.ctx, "Validating: %v", v.key)
	defer s.Failed()

	summary := v.findFullProjectResult(summaries)
	if summary == nil {
		s.FailedWithMessage("No result summaries for %v", v.key)
		return false
	}

	ca := v.vm.cam.getForClusterId(summary.ClusterInfo.ClusterId)
	if ca == nil {
		s.FailedWithMessage("No cluster accessor for %v", v.key)
		return false
	}
	k := ca.getK()
	if k == nil {
		return false
	}

	cr, err := v.vm.store.GetCommandResult(results.GetCommandResultOptions{
		Id:      summary.Id,
		Reduced: false,
	})
	if err != nil {
		s.FailedWithMessage("Failed to get command result for %v: %v", v.key, err)
		return false
	}

	discriminator := summary.Target.Discriminator

	cmd := commands.NewValidateCommand(v.vm.ctx, discriminator, nil, cr)
	vr, err := cmd.Run(v.vm.ctx, k)

	if err != nil {
		s.UpdateAndInfoFallback("Finished validation with error: %v", v.key)
	} else {
		s.UpdateAndInfoFallback("Finished validation: %v", v.key)
	}
	s.Success()

	v.vm.mutex.Lock()
	defer v.vm.mutex.Unlock()
	v.validateResult = vr
	v.err = err

	for _, h := range v.vm.handlers {
		h(v.key, vr)
	}

	return true
}
