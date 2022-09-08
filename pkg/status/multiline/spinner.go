package multiline

import (
	"time"
)

var spinner = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

func Spinner() string {
	factor := int64(100)
	frame := int((time.Now().UnixMilli() / factor) % int64(len(spinner)))
	return spinner[frame]
}
