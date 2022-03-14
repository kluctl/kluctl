package commands

import (
	"github.com/codablock/kluctl/pkg/deployment"
	utils2 "github.com/codablock/kluctl/pkg/deployment/utils"
	"github.com/codablock/kluctl/pkg/k8s"
	"github.com/codablock/kluctl/pkg/types"
	k8s2 "github.com/codablock/kluctl/pkg/types/k8s"
	"github.com/codablock/kluctl/pkg/validation"
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

func (cmd *ValidateCommand) Run(k *k8s.K8sCluster) (*types.ValidateResult, error) {
	var result types.ValidateResult

	cmd.dew.Init()

	err := cmd.ru.UpdateRemoteObjects(k, cmd.c.Project.GetCommonLabels(), cmd.c.LocalObjectRefs())
	if err != nil {
		return nil, err
	}

	a := utils2.NewApplyUtil(cmd.dew, cmd.c, cmd.ru, k, utils2.ApplyUtilOptions{})
	h := utils2.NewHooksUtil(a)
	for _, d := range cmd.c.Deployments {
		if !d.CheckInclusionForDeploy() {
			continue
		}
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
			r := validation.ValidateObject(remoteObject, true)
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
