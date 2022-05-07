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
	c   *deployment.DeploymentCollection
	dew *utils2.DeploymentErrorsAndWarnings
	ru  *utils2.RemoteObjectUtils
}

func NewValidateCommand(c *deployment.DeploymentCollection) *ValidateCommand {
	cmd := &ValidateCommand{
		c:   c,
		dew: utils2.NewDeploymentErrorsAndWarnings(),
	}
	cmd.ru = utils2.NewRemoteObjectsUtil(cmd.dew)
	return cmd
}

func (cmd *ValidateCommand) Run(ctx context.Context, k *k8s.K8sCluster) (*types.ValidateResult, error) {
	var result types.ValidateResult

	cmd.dew.Init()

	err := cmd.ru.UpdateRemoteObjects(k, cmd.c.Project.GetCommonLabels(), cmd.c.LocalObjectRefs())
	if err != nil {
		return nil, err
	}

	ad := utils2.NewApplyDeploymentsUtil(ctx, cmd.dew, cmd.c.Deployments, cmd.ru, k, &utils2.ApplyUtilOptions{})
	for _, d := range cmd.c.Deployments {
		if !d.CheckInclusionForDeploy() {
			continue
		}
		au := ad.NewApplyUtil(utils2.NewProgressCtx(nil, d.RelToProjectItemDir, 0, true))
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
			r := validation.ValidateObject(k, remoteObject, true)
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
