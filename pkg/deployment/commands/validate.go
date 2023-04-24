package commands

import (
	"context"
	"fmt"
	"github.com/google/uuid"
	"github.com/kluctl/kluctl/v2/pkg/deployment"
	utils2 "github.com/kluctl/kluctl/v2/pkg/deployment/utils"
	"github.com/kluctl/kluctl/v2/pkg/k8s"
	k8s2 "github.com/kluctl/kluctl/v2/pkg/types/k8s"
	"github.com/kluctl/kluctl/v2/pkg/types/result"
	"github.com/kluctl/kluctl/v2/pkg/utils/uo"
	"github.com/kluctl/kluctl/v2/pkg/validation"
)

type ValidateCommand struct {
	c             *deployment.DeploymentCollection
	r             *result.CommandResult
	discriminator string

	dew *utils2.DeploymentErrorsAndWarnings
	ru  *utils2.RemoteObjectUtils
}

func NewValidateCommand(ctx context.Context, discriminator string, c *deployment.DeploymentCollection, r *result.CommandResult) *ValidateCommand {
	cmd := &ValidateCommand{
		c:             c,
		r:             r,
		discriminator: discriminator,
		dew:           utils2.NewDeploymentErrorsAndWarnings(),
	}
	cmd.ru = utils2.NewRemoteObjectsUtil(ctx, cmd.dew)
	return cmd
}

func (cmd *ValidateCommand) Run(ctx context.Context, k *k8s.K8sCluster) (*result.ValidateResult, error) {
	ret := result.ValidateResult{
		Id:    uuid.New().String(),
		Ready: true,
	}

	cmd.dew.Init()

	var refs []k8s2.ObjectRef
	var renderedObjects []*uo.UnstructuredObject
	appliedObjects := map[k8s2.ObjectRef]*uo.UnstructuredObject{}
	var discriminator string

	if cmd.c != nil && cmd.r != nil {
		return nil, fmt.Errorf("passing both deployment collection and command result is not allowed")
	} else if cmd.c != nil {
		for _, d := range cmd.c.Deployments {
			for _, o := range d.Objects {
				ref := o.GetK8sRef()
				refs = append(refs, ref)
				renderedObjects = append(renderedObjects, o)
			}
		}
	} else if cmd.r != nil {
		for _, o := range cmd.r.Objects {
			refs = append(refs, o.Ref)
			if o.Rendered != nil {
				renderedObjects = append(renderedObjects, o.Rendered)
			}
			if o.Applied != nil {
				appliedObjects[o.Ref] = o.Applied
			}
		}
		discriminator = cmd.r.Target.Discriminator
	} else {
		return nil, fmt.Errorf("either deployment collection or command result must be passed")
	}

	if discriminator == "" {
		discriminator = cmd.discriminator
	}

	err := cmd.ru.UpdateRemoteObjects(k, &cmd.discriminator, refs, true)
	if err != nil {
		return nil, err
	}

	ad := utils2.NewApplyDeploymentsUtil(ctx, cmd.dew, cmd.ru, k, &utils2.ApplyUtilOptions{})
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
		r := validation.ValidateObject(k, remoteObject, true, false)
		if !r.Ready {
			ret.Ready = false
		}
		ret.Errors = append(ret.Errors, r.Errors...)
		ret.Warnings = append(ret.Warnings, r.Warnings...)
		ret.Results = append(ret.Results, r.Results...)
	}

	du := utils2.NewDiffUtil(cmd.dew, cmd.ru, appliedObjects)
	du.Swapped = true
	du.DiffObjects(renderedObjects)

	ret.Warnings = append(ret.Warnings, cmd.dew.GetWarningsList()...)
	ret.Errors = append(ret.Errors, cmd.dew.GetErrorsList()...)

	if len(du.ChangedObjects) != 0 {
		ret.Drift = du.ChangedObjects
	}

	return &ret, nil
}

func (cmd *ValidateCommand) ForgetRemoteObject(ref k8s2.ObjectRef) {
	cmd.ru.ForgetRemoteObject(ref)
}
