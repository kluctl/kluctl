package commands

import (
	"context"
	"github.com/google/uuid"
	utils2 "github.com/kluctl/kluctl/v2/pkg/deployment/utils"
	"github.com/kluctl/kluctl/v2/pkg/kluctl_project"
	k8s2 "github.com/kluctl/kluctl/v2/pkg/types/k8s"
	"github.com/kluctl/kluctl/v2/pkg/types/result"
	"github.com/kluctl/kluctl/v2/pkg/utils/uo"
	"github.com/kluctl/kluctl/v2/pkg/validation"
)

type ValidateCommand struct {
	targetCtx     *kluctl_project.TargetContext
	r             *result.CommandResult
	discriminator string

	dew *utils2.DeploymentErrorsAndWarnings
	ru  *utils2.RemoteObjectUtils
}

func NewValidateCommand(ctx context.Context, discriminator string, targetCtx *kluctl_project.TargetContext, r *result.CommandResult) *ValidateCommand {
	cmd := &ValidateCommand{
		targetCtx:     targetCtx,
		r:             r,
		discriminator: discriminator,
		dew:           utils2.NewDeploymentErrorsAndWarnings(),
	}
	cmd.ru = utils2.NewRemoteObjectsUtil(ctx, cmd.dew)
	return cmd
}

func (cmd *ValidateCommand) Run(ctx context.Context) (*result.ValidateResult, error) {
	ret := result.ValidateResult{
		Id:    uuid.New().String(),
		Ready: true,
	}

	cmd.dew.Init()

	var refs []k8s2.ObjectRef
	var renderedObjects []*uo.UnstructuredObject
	appliedObjects := map[k8s2.ObjectRef]*uo.UnstructuredObject{}
	discriminator := cmd.discriminator

	if cmd.r != nil {
		for _, o := range cmd.r.Objects {
			refs = append(refs, o.Ref)
			if o.Hook {
				continue
			}
			if o.Rendered != nil {
				renderedObjects = append(renderedObjects, o.Rendered)
			}
			if o.Applied != nil {
				appliedObjects[o.Ref] = o.Applied
			}
		}
		if discriminator == "" {
			discriminator = cmd.r.TargetKey.Discriminator
		}
	} else {
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
	}

	err := cmd.ru.UpdateRemoteObjects(cmd.targetCtx.SharedContext.K, &discriminator, refs, true)
	if err != nil {
		return nil, err
	}

	ad := utils2.NewApplyDeploymentsUtil(ctx, cmd.dew, cmd.ru, cmd.targetCtx.SharedContext.K, &utils2.ApplyUtilOptions{})
	for _, o := range renderedObjects {
		au := ad.NewApplyUtil(ctx, nil)
		h := utils2.NewHooksUtil(au)
		hook := h.GetHook(o)
		if hook != nil && !hook.IsPersistent() {
			continue
		}

		ref := o.GetK8sRef()

		remoteObject := cmd.ru.GetRemoteObject(ref)
		if remoteObject == nil {
			ret.Errors = append(ret.Errors, result.DeploymentError{Ref: ref, Message: "object not found"})
			continue
		}
		r := validation.ValidateObject(cmd.targetCtx.SharedContext.K, remoteObject, true, false)
		if !r.Ready {
			ret.Ready = false
		}
		ret.Errors = append(ret.Errors, r.Errors...)
		ret.Warnings = append(ret.Warnings, r.Warnings...)
		ret.Results = append(ret.Results, r.Results...)
	}

	ret.Warnings = append(ret.Warnings, cmd.dew.GetWarningsList()...)
	ret.Errors = append(ret.Errors, cmd.dew.GetErrorsList()...)

	err = addValidateCommandInfoToResult(cmd.targetCtx, &ret)
	if err != nil {
		return nil, err
	}

	return &ret, nil
}

func (cmd *ValidateCommand) ForgetRemoteObject(ref k8s2.ObjectRef) {
	cmd.ru.ForgetRemoteObject(ref)
}
