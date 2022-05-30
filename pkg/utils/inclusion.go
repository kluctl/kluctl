package utils

type InclusionEntry struct {
	Type  string
	Value string
}

type Inclusion struct {
	includes map[InclusionEntry]bool
	excludes map[InclusionEntry]bool
}

func NewInclusion() *Inclusion {
	return &Inclusion{
		includes: map[InclusionEntry]bool{},
		excludes: map[InclusionEntry]bool{},
	}
}

func (inc *Inclusion) AddInclude(typ string, value string) {
	inc.includes[InclusionEntry{typ, value}] = true
}

func (inc *Inclusion) AddExclude(typ string, value string) {
	inc.excludes[InclusionEntry{typ, value}] = true
}

func (inc *Inclusion) HasType(typ string) bool {
	if inc == nil {
		return false
	}
	for e, _ := range inc.includes {
		if e.Type == typ {
			return true
		}
	}
	for e, _ := range inc.excludes {
		if e.Type == typ {
			return true
		}
	}
	return false
}

func (inc *Inclusion) checkList(l []InclusionEntry, m map[InclusionEntry]bool) bool {
	for _, e := range l {
		if _, ok := m[e]; ok {
			return true
		}
	}
	return false
}

func (inc *Inclusion) CheckIncluded(l []InclusionEntry, excludeIfNotIncluded bool) bool {
	if inc == nil {
		return true
	}
	if len(inc.includes) == 0 && len(inc.excludes) == 0 {
		return true
	}

	isIncluded := inc.checkList(l, inc.includes)
	isExcluded := inc.checkList(l, inc.excludes)

	if excludeIfNotIncluded {
		if !isIncluded {
			return false
		}
	}
	if isExcluded {
		return false
	}
	return len(inc.includes) == 0 || isIncluded
}
