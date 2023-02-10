package commands

import (
	"context"
	"github.com/kluctl/kluctl/v2/pkg/deployment"
	utils2 "github.com/kluctl/kluctl/v2/pkg/deployment/utils"
	"github.com/kluctl/kluctl/v2/pkg/k8s"
	"github.com/kluctl/kluctl/v2/pkg/types"
	k8s2 "github.com/kluctl/kluctl/v2/pkg/types/k8s"
	"github.com/kluctl/kluctl/v2/pkg/validation"
)

type ValidateCommand struct {
	c             *deployment.DeploymentCollection
	discriminator string

	dew *utils2.DeploymentErrorsAndWarnings
	ru  *utils2.RemoteObjectUtils
}

func NewValidateCommand(ctx context.Context, discriminator string, c *deployment.DeploymentCollection) *ValidateCommand {
	cmd := &ValidateCommand{
		c:             c,
		discriminator: discriminator,
		dew:           utils2.NewDeploymentErrorsAndWarnings(),
	}
	cmd.ru = utils2.NewRemoteObjectsUtil(ctx, cmd.dew)
	return cmd
}

func (cmd *ValidateCommand) Run(ctx context.Context, k *k8s.K8sCluster) (*types.ValidateResult, error) {
	var result types.ValidateResult
	result.Ready = true

	cmd.dew.Init()

	err := cmd.ru.UpdateRemoteObjects(k, &cmd.discriminator, cmd.c.LocalObjectRefs(), true)
	if err != nil {
		return nil, err
	}

	ad := utils2.NewApplyDeploymentsUtil(ctx, cmd.dew, cmd.c.Deployments, cmd.ru, k, &utils2.ApplyUtilOptions{})
	for _, d := range cmd.c.Deployments {
		au := ad.NewApplyUtil(ctx, nil)
		h := utils2.NewHooksUtil(au)
		for _, o := range d.Objects {
			hook := h.GetHook(o)
			if hook != nil && !hook.IsPersistent() {
				continue
			}

			ref := o.GetK8sRef()

			remoteObject := cmd.ru.GetRemoteObject(ref)
			if remoteObject == nil {
				result.Errors = append(result.Errors, types.DeploymentError{Ref: ref, Error: "object not found"})
				continue
			}
			r := validation.ValidateObject(k, remoteObject, true, false)
			if !r.Ready {
				result.Ready = false
			}
			result.Errors = append(result.Errors, r.Errors...)
			result.Warnings = append(result.Warnings, r.Warnings...)
			result.Results = append(result.Results, r.Results...)
		}
	}

	result.Warnings = append(result.Warnings, cmd.dew.GetWarningsList()...)
	result.Errors = append(result.Errors, cmd.dew.GetErrorsList()...)

	return &result, nil
}

func (cmd *ValidateCommand) ForgetRemoteObject(ref k8s2.ObjectRef) {
	cmd.ru.ForgetRemoteObject(ref)
}
