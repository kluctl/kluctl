//go:build !windows

package multiline

import (
	"fmt"
	"strings"
)

// clear the line and move the cursor up
var clear = fmt.Sprintf("%c[%dA%c[2K", ESC, 1, ESC)

func (ml *MultiLinePrinter) clearLines(lines int) {
	_, _ = fmt.Fprint(ml.w, strings.Repeat(clear, lines))
}
