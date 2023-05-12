package result

import (
	"sort"
)

type CommandResultSummary struct {
	Id          string      `json:"id"`
	Project     ProjectKey  `json:"project"`
	Target      TargetKey   `json:"target"`
	Command     CommandInfo `json:"commandInfo"`
	GitInfo     *GitInfo    `json:"gitInfo,omitempty"`
	ClusterInfo ClusterInfo `json:"clusterInfo,omitempty"`

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

type TargetSummary struct {
	Target TargetKey `json:"target"`

	LastValidateResult *ValidateResult        `json:"lastValidateResult,omitempty"`
	CommandResults     []CommandResultSummary `json:"commandResults,omitempty"`
}

type ProjectSummary struct {
	Project ProjectKey `json:"project"`

	Targets []*TargetSummary `json:"targets"`
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
		Project:            cr.ProjectKey,
		Target:             cr.TargetKey,
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

func BuildProjectSummaries(summaries []CommandResultSummary) []*ProjectSummary {
	m := map[ProjectKey]*ProjectSummary{}
	for _, rs := range summaries {
		p, ok := m[rs.Project]
		if !ok {
			p = &ProjectSummary{Project: rs.Project}
			m[rs.Project] = p
		}

		var target *TargetSummary
		for i, t := range p.Targets {
			if t.Target == rs.Target {
				target = p.Targets[i]
				break
			}
		}
		if target == nil {
			target = &TargetSummary{
				Target: rs.Target,
			}
			p.Targets = append(p.Targets, target)
		}

		target.CommandResults = append(target.CommandResults, rs)
	}

	ret := make([]*ProjectSummary, 0, len(m))
	for _, p := range m {
		sort.Slice(p.Targets, func(i, j int) bool {
			return p.Targets[i].Target.Less(p.Targets[j].Target)
		})
		ret = append(ret, p)
	}

	sort.Slice(ret, func(i, j int) bool {
		return ret[i].Project.Less(ret[j].Project)
	})

	return ret
}
