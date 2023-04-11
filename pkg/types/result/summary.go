package result

type CommandResultSummary struct {
	Id          string       `json:"id"`
	Project     ProjectKey   `json:"project"`
	Command     CommandInfo  `json:"commandInfo"`
	GitInfo     *GitInfo     `json:"gitInfo,omitempty"`
	ClusterInfo *ClusterInfo `json:"clusterInfo,omitempty"`

	RenderedObjects    int `json:"renderedObjects"`
	RemoteObjects      int `json:"remoteObjects"`
	AppliedObjects     int `json:"appliedObjects"`
	AppliedHookObjects int `json:"appliedHookObjects"`

	NewObjects     int `json:"newObjects"`
	ChangedObjects int `json:"changedObjects"`
	OrphanObjects  int `json:"orphanObjects"`
	DeletedObjects int `json:"deletedObjects"`
	Errors         int `json:"errors"`
	Warnings       int `json:"warnings"`

	TotalChanges int `json:"totalChanges"`
}

type ProjectSummary struct {
	Project ProjectKey `json:"project"`

	LastValidateResult *ValidateResult `json:"lastValidateResult,omitempty"`

	LastDeployCommand *CommandResultSummary `json:"lastDeployCommand,omitempty"`
	LastDeleteCommand *CommandResultSummary `json:"LastDeleteCommand,omitempty"`
	LastPruneCommand  *CommandResultSummary `json:"lastPruneCommand,omitempty"`
}

func (cr *CommandResult) BuildSummary() *CommandResultSummary {
	count := func(f func(o ResultObject) bool) int {
		cnt := 0
		for _, o := range cr.Objects {
			if f(o) {
				cnt++
			}
		}
		return cnt
	}

	ret := &CommandResultSummary{
		Id:                 cr.Id,
		Project:            cr.Project,
		Command:            cr.Command,
		GitInfo:            cr.GitInfo,
		ClusterInfo:        cr.ClusterInfo,
		RenderedObjects:    count(func(o ResultObject) bool { return o.Rendered != nil }),
		RemoteObjects:      count(func(o ResultObject) bool { return o.Remote != nil }),
		AppliedObjects:     count(func(o ResultObject) bool { return o.Applied != nil }),
		AppliedHookObjects: count(func(o ResultObject) bool { return o.Hook }),
		NewObjects:         count(func(o ResultObject) bool { return o.New }),
		ChangedObjects:     count(func(o ResultObject) bool { return len(o.Changes) != 0 }),
		OrphanObjects:      count(func(o ResultObject) bool { return o.Orphan }),
		DeletedObjects:     count(func(o ResultObject) bool { return o.Deleted }),
		Errors:             len(cr.Errors),
		Warnings:           len(cr.Warnings),
	}
	for _, o := range cr.Objects {
		ret.TotalChanges += len(o.Changes)
	}
	return ret
}
