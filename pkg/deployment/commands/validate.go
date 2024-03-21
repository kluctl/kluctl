package commands

import (
	"context"
	"fmt"
	utils2 "github.com/kluctl/kluctl/v2/pkg/deployment/utils"
	"github.com/kluctl/kluctl/v2/pkg/kluctl_project/target-context"
	k8s2 "github.com/kluctl/kluctl/v2/pkg/types/k8s"
	"github.com/kluctl/kluctl/v2/pkg/types/result"
	"github.com/kluctl/kluctl/v2/pkg/utils/uo"
	"github.com/kluctl/kluctl/v2/pkg/validation"
)

type ValidateCommand struct {
	targetCtx     *target_context.TargetContext
	discriminator string

	dew *utils2.DeploymentErrorsAndWarnings
	ru  *utils2.RemoteObjectUtils
}

func NewValidateCommand(discriminator string, targetCtx *target_context.TargetContext) *ValidateCommand {
	cmd := &ValidateCommand{
		targetCtx:     targetCtx,
		discriminator: discriminator,
		dew:           utils2.NewDeploymentErrorsAndWarnings(),
	}
	cmd.ru = utils2.NewRemoteObjectsUtil(targetCtx.SharedContext.Ctx, cmd.dew)
	return cmd
}

func (cmd *ValidateCommand) Run(ctx context.Context) *result.ValidateResult {
	startTime := cmd.targetCtx.KluctlProject.LoadTime

	cmd.dew.Init()

	ret := newValidateCommandResult(cmd.targetCtx, startTime)
	ret.Ready = true

	defer func() {
		finishValidateResult(ret, cmd.targetCtx, cmd.dew)
	}()

	var refs []k8s2.ObjectRef
	var renderedObjects []*uo.UnstructuredObject
	discriminator := cmd.discriminator

	for _, d := range cmd.targetCtx.DeploymentCollection.Deployments {
		for _, o := range d.Objects {
			ref := o.GetK8sRef()
			refs = append(refs, ref)
			renderedObjects = append(renderedObjects, o)
		}
	}
	if discriminator == "" {
		discriminator = cmd.targetCtx.Target.Discriminator
	}

	err := cmd.ru.UpdateRemoteObjects(cmd.targetCtx.SharedContext.K, &discriminator, refs, true)
	if err != nil {
		cmd.dew.AddError(k8s2.ObjectRef{}, err)
		return ret
	}

	ad := utils2.NewApplyDeploymentsUtil(ctx, cmd.dew, cmd.ru, cmd.targetCtx.SharedContext.K, &utils2.ApplyUtilOptions{})
	for _, o := range renderedObjects {
		if o.GetK8sAnnotationBoolNoError("kluctl.io/delete", false) {
			if cmd.ru.GetRemoteObject(o.GetK8sRef()) != nil {
				cmd.dew.AddError(o.GetK8sRef(), fmt.Errorf("object is marked for deletion but still exists on the target cluster"))
			}
			continue
		}

		au := ad.NewApplyUtil(ctx, nil)
		h := utils2.NewHooksUtil(au)
		if hook := h.GetHook(o); hook != nil {
			if !hook.IsPersistent() {
				// the hook is not expected to exist after deployment
				continue
			}
			if hook.IsOnlyDelete() || hook.IsOnlyRollback() {
				// the hook is not expected to be executed
				continue
			}
		}

		ref := o.GetK8sRef()

		remoteObject := cmd.ru.GetRemoteObject(ref)
		if remoteObject == nil {
			ret.Errors = append(ret.Errors, result.DeploymentError{Ref: ref, Message: "object not found"})
			continue
		}
		r := validation.ValidateObject(ctx, cmd.targetCtx.SharedContext.K, remoteObject, true, false)
		if !r.Ready {
			ret.Ready = false
		}
		ret.Errors = append(ret.Errors, r.Errors...)
		ret.Warnings = append(ret.Warnings, r.Warnings...)
		ret.Results = append(ret.Results, r.Results...)
	}

	return ret
}

func (cmd *ValidateCommand) ForgetRemoteObject(ref k8s2.ObjectRef) {
	cmd.ru.ForgetRemoteObject(ref)
}
