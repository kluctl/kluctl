package utils

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
)

func ParseEnvBool(name string, def bool) (bool, error) {
	if x, ok := os.LookupEnv(name); ok {
		b, err := strconv.ParseBool(x)
		if err != nil {
			return def, err
		}
		return b, nil
	}
	return def, nil
}

func parseEnv(prefix string, withIndex bool, withSuffix bool) map[int]map[string]string {
	ret := make(map[int]map[string]string)

	rs := prefix
	curGroup := 1
	indexGroup := -1
	suffixGroup := -1
	if withIndex {
		rs += `(_\d+)?`
		indexGroup = curGroup
		curGroup++
	}
	if withSuffix {
		rs += `_(.*)`
		suffixGroup = curGroup
		curGroup++
	}
	rs = fmt.Sprintf("^%s$", rs)
	r := regexp.MustCompile(rs)

	for _, e := range os.Environ() {
		eq := strings.Index(e, "=")
		if eq == -1 {
			panic(fmt.Sprintf("unexpected env var %s", e))
		}
		n := e[:eq]
		v := e[eq+1:]

		idx := -1
		suffix := ""

		m := r.FindStringSubmatch(n)
		if m == nil {
			continue
		}

		if withIndex {
			idxStr := m[indexGroup]
			if idxStr != "" {
				idxStr = idxStr[1:] // remove leading _
				x, err := strconv.ParseInt(idxStr, 10, 32)
				if err != nil {
					continue
				}
				idx = int(x)
			}
		}
		if withSuffix {
			suffix = m[suffixGroup]
		}

		if _, ok := ret[idx]; !ok {
			ret[idx] = map[string]string{}
		}
		ret[idx][suffix] = v
	}
	return ret
}

func ParseEnvConfigSets(prefix string) map[int]map[string]string {
	return parseEnv(prefix, true, true)
}

func ParseEnvConfigList(prefix string) map[int]string {
	ret := make(map[int]string)

	for idx, m := range parseEnv(prefix, true, false) {
		ret[idx] = m[""]
	}
	return ret
}
