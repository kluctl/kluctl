package commands

import (
	"github.com/codablock/kluctl/pkg/deployment"
	utils2 "github.com/codablock/kluctl/pkg/deployment/utils"
	"github.com/codablock/kluctl/pkg/k8s"
	"github.com/codablock/kluctl/pkg/types"
	"github.com/codablock/kluctl/pkg/validation"
)

type ValidateCommand struct {
	c *deployment.DeploymentCollection
}

func NewValidateCommand(c *deployment.DeploymentCollection) *ValidateCommand {
	return &ValidateCommand{
		c: c,
	}
}

func (cmd *ValidateCommand) Run(k *k8s.K8sCluster) *types.ValidateResult {
	var result types.ValidateResult

	dew := utils2.NewDeploymentErrorsAndWarnings()

	a := utils2.NewApplyUtil(dew, cmd.c, k, utils2.ApplyUtilOptions{})
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

			remoteObject := cmd.c.GetRemoteObject(ref)
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

	result.Warnings = append(result.Warnings, dew.GetWarningsList()...)
	result.Errors = append(result.Errors, dew.GetErrorsList()...)

	return &result
}
