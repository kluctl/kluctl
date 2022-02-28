package versions

import (
	"fmt"
	"github.com/codablock/kluctl/pkg/utils"
	"regexp"
	"strconv"
)

type LatestVersionFilter interface {
	Match(version string) bool
	Latest(versions []string) string
	String() string
}

func Filter(lv LatestVersionFilter, versions []string) []string {
	var filtered []string
	for _, v := range versions {
		if lv.Match(v) {
			filtered = append(filtered, v)
		}
	}
	return filtered
}

type regexVersionFilter struct {
	patternStr string
	pattern *regexp.Regexp
}

func NewRegexVersionFilter(pattern string) (LatestVersionFilter, error) {
	p, err := regexp.Compile(fmt.Sprintf("^%s$", pattern))
	if err != nil {
		return nil, err
	}
	return &regexVersionFilter{
		patternStr: pattern,
		pattern: p,
	}, nil
}

func (f *regexVersionFilter) Match(version string) bool{
	return f.pattern.MatchString(version)
}

func (f *regexVersionFilter) Latest(versions []string) string {
	c := SortLooseVersionStrings(versions)
	return string(c[len(c)-1])
}

func (f *regexVersionFilter) String() string {
	return fmt.Sprintf(`regex(pattern="%s")`, f.patternStr)
}

type looseSemVerVersionFilter struct {
	allowNoNums bool
}

func NewLooseSemVerVersionFilter(allowNoNums bool) LatestVersionFilter {
	return &looseSemVerVersionFilter{allowNoNums: allowNoNums}
}

func (f *looseSemVerVersionFilter) Match(version string) bool {
	groups := looseSemverRegex.FindStringSubmatch(version)
	if groups == nil {
		return false
	}
	if !f.allowNoNums && groups[1] == "" {
		return false
	}
	return true
}

func (f *looseSemVerVersionFilter) Latest(versions []string) string {
	c := SortLooseVersionStrings(versions)
	return string(c[len(c)-1])
}

func (f *looseSemVerVersionFilter) String() string {
	return fmt.Sprintf(`semver(allow_no_nums=%s)`, strconv.FormatBool(f.allowNoNums))
}

type prefixVersionFilter struct {
	prefix  string
	suffix  LatestVersionFilter
	pattern *regexp.Regexp
}

func NewPrefixVersionFilter(prefix string, suffix LatestVersionFilter) (LatestVersionFilter, error) {
	if suffix == nil {
		suffix = NewLooseSemVerVersionFilter(false)
	}

	p, err := regexp.Compile(fmt.Sprintf(`^%s(.*)$`, prefix))
	if err != nil {
		return nil, err
	}
	return &prefixVersionFilter{
		prefix:  prefix,
		suffix:  suffix,
		pattern: p,
	}, nil
}

func (f *prefixVersionFilter) Match(version string) bool {
	groups := f.pattern.FindStringSubmatch(version)
	if groups == nil {
		return false
	}
	return f.suffix.Match(groups[1])
}

func (f *prefixVersionFilter) Latest(versions []string) string {
	var filteredVersions []string
	var suffixVersions []string
	for _, v := range versions {
		groups := f.pattern.FindStringSubmatch(v)
		if groups == nil {
			continue
		}
		filteredVersions = append(filteredVersions, v)
		suffixVersions = append(suffixVersions, groups[1])
	}
	latest := f.suffix.Latest(suffixVersions)
	i := utils.FindStrInSlice(suffixVersions, latest)
	return filteredVersions[i]
}

func (f *prefixVersionFilter) String() string {
	return fmt.Sprintf(`prefix(prefix="%s", suffix=%s)`, f.prefix, f.suffix.String())
}

type numberVersionFilter struct {
	LatestVersionFilter
}

func NewNumberVersionFilter() (LatestVersionFilter, error) {
	f, _ := NewRegexVersionFilter("[0-9]+")
	return &numberVersionFilter{f}, nil
}

func (f *numberVersionFilter) String() string {
	return "number()"
}
