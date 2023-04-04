package result

type CommandResultSummary struct {
	Id      string       `json:"id"`
	Command *CommandInfo `json:"commandInfo"`

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
