package utils

import (
	"fmt"
	log "github.com/sirupsen/logrus"
	"os"
	"regexp"
	"strconv"
	"strings"
)

func ParseEnvConfigSets(prefix string) map[int]map[string]string {
	ret := make(map[int]map[string]string)

	r := regexp.MustCompile(fmt.Sprintf(`^%s_(\d+)_(.*)$`, prefix))
	r2 := regexp.MustCompile(fmt.Sprintf(`^%s_(.*)$`, prefix))

	for _, e := range os.Environ() {
		eq := strings.Index(e, "=")
		if eq == -1 {
			log.Panicf("unexpected env var %s", e)
		}
		n := e[:eq]
		v := e[eq+1:]

		idx := -1
		key := ""

		m := r.FindStringSubmatch(n)
		if m != nil {
			x, _ := strconv.ParseInt(m[1], 10, 32)
			idx = int(x)
			key = m[2]
		} else {
			m = r2.FindStringSubmatch(n)
			if m != nil {
				key = m[1]
			}
		}

		if key != "" {
			if _, ok := ret[idx]; !ok {
				ret[idx] = make(map[string]string)
			}
			ret[idx][key] = v
		}
	}
	return ret
}
