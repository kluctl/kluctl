package term

import (
	"os"
	"strconv"
)

var origStdout = os.Stdout

func GetWidth() int {
	if c, ok := os.LookupEnv("COLUMNS"); ok {
		tw, err := strconv.ParseInt(c, 10, 32)
		if err == nil {
			return int(tw)
		}
	}
	w, _, err := GetSize(int(origStdout.Fd()))
	if err != nil || w == 0 {
		return 80
	}
	return w
}
