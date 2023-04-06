package result

type CommandResultSummary struct {
	Project ProjectKey  `json:"project"`
	Command CommandInfo `json:"commandInfo"`
	GitInfo *GitInfo    `json:"gitInfo,omitempty"`

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
