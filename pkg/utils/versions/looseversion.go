package versions

import (
	"github.com/codablock/kluctl/pkg/utils"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

var looseSemverRegex = regexp.MustCompile(`^(([0-9]+)(\.[0-9]+)*)?(.*)$`)
var suffixComponentRegex = regexp.MustCompile(`\d+|[a-zA-Z]+|\.`)

// Allows to compare ints and strings. Strings are always considered less than ints.
type LooseVersionSuffixElement struct {
	v interface{}
}

func (x LooseVersionSuffixElement) Less(b LooseVersionSuffixElement) bool {
	ia, iaOk := x.v.(int64)
	ib, ibOk := b.v.(int64)
	sa, _ := x.v.(string)
	sb, _ := b.v.(string)
	if ibOk == iaOk {
		if iaOk {
			return ia < ib
		} else {
			return sa < sb
		}
	}
	if iaOk {
		return false
	}
	return true
}

type LooseVersion string

func (lv LooseVersion) SplitVersion() ([]int, string) {
	m := looseSemverRegex.FindStringSubmatch(string(lv))
	numsStr := m[1]
	suffix := m[4]

	var nums []int
	for _, x := range strings.Split(numsStr, ".") {
		i, _ := strconv.ParseInt(x, 10, 32)
		nums = append(nums, int(i))
	}
	return nums, suffix
}

func regexSplitLikePython(s string, re *regexp.Regexp) []string {
	ret := []string{s}
	indexes := re.FindAllStringIndex(s, -1)
	start := 0
	for _, se := range indexes {
		m := s[start:se[0]]
		if len(m) != 0 {
			ret = append(ret, m)
		}
		m = s[se[0]:se[1]]
		if len(m) != 0 {
			ret = append(ret, m)
		}
		start = se[1]
	}
	if start < len(s) {
		ret = append(ret, s[start:])
	}
	return ret
}

func splitSuffix(suffix string) []LooseVersionSuffixElement {
	var components []LooseVersionSuffixElement
	for i, x := range regexSplitLikePython(suffix, suffixComponentRegex) {
		if i == 0 {
			continue
		}
		if x != "." {
			y, err := strconv.ParseInt(x, 10, 32)
			if err == nil {
				components = append(components, LooseVersionSuffixElement{v: y})
			} else {
				components = append(components, LooseVersionSuffixElement{v: x})
			}
		}
	}
	return components
}

func (lv LooseVersion) Less(b LooseVersion, preferLongSuffix bool) bool {
	aNums, aSuffixStr := lv.SplitVersion()
	bNums, bSuffixStr := b.SplitVersion()

	cmp := func(a []int, b []int) bool {
		l := utils.IntMin(len(a), len(b))
		for i := 0; i < l; i++ {
			if a[i] < b[i] {
				return true
			}
			if b[i] < a[i] {
				return false
			}
		}
		if len(a) < len(b) {
			return true
		}
		return false
	}

	if cmp(aNums, bNums) {
		return true
	}
	if cmp(bNums, aNums) {
		return false
	}
	if len(aSuffixStr) == 0 && len(bSuffixStr) != 0 {
		return false
	} else if len(aSuffixStr) != 0 && len(bSuffixStr) == 0 {
		return true
	}

	aSuffix := splitSuffix(aSuffixStr)
	bSuffix := splitSuffix(bSuffixStr)
	l := utils.IntMin(len(aSuffix), len(bSuffix))

	for i := 0; i < l; i++ {
		if aSuffix[i].Less(bSuffix[i]) {
			return true
		} else if bSuffix[i].Less(aSuffix[i]) {
			return false
		}
	}

	if preferLongSuffix {
		if len(aSuffix) < len(bSuffix) {
			return true
		}
	} else {
		if len(bSuffix) < len(aSuffix) {
			return true
		}
	}
	return false
}

func (lv LooseVersion) Compare(b LooseVersion) int {
	if lv.Less(b, true) {
		return -1
	} else if b.Less(lv, true) {
		return 1
	}
	return 0
}

type LooseVersionSlice []LooseVersion

func (x LooseVersionSlice) Less(i, j int) bool {
	return x[i].Less(x[j], true)
}
func (x LooseVersionSlice) Len() int      { return len(x) }
func (x LooseVersionSlice) Swap(i, j int) { x[i], x[j] = x[j], x[i] }

func SortLooseVersionStrings(versions []string) LooseVersionSlice {
	var c LooseVersionSlice
	for _, v := range versions {
		c = append(c, LooseVersion(v))
	}
	sort.Stable(c)
	return c
}
