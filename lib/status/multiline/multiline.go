package multiline

import (
	"fmt"
	"github.com/acarl005/stripansi"
	"github.com/kluctl/kluctl/lib/term"
	"github.com/mattn/go-runewidth"
	"io"
	"strings"
	"sync"
	"time"
)

type MultiLinePrinter struct {
	topLines []*Line
	lines    []*Line

	w         io.Writer
	prevLines []string

	ticker *time.Ticker
	tdone  chan bool

	mutex sync.Mutex
}

type LineFunc func() string

type Line struct {
	ml *MultiLinePrinter
	s  LineFunc
}

func (ml *MultiLinePrinter) Start(w io.Writer) {
	ml.w = w

	ml.ticker = time.NewTicker(100 * time.Millisecond)
	ml.tdone = make(chan bool)

	go ml.loop()
}

func (ml *MultiLinePrinter) Stop() {
	ml.Flush()
	ml.tdone <- true
	<-ml.tdone
}

func (ml *MultiLinePrinter) loop() {
	for {
		select {
		case <-ml.ticker.C:
			if ml.ticker != nil {
				ml.Flush()
			}
		case <-ml.tdone:
			ml.mutex.Lock()
			ml.ticker.Stop()
			ml.ticker = nil
			ml.mutex.Unlock()
			close(ml.tdone)
			return
		}
	}
}

func (ml *MultiLinePrinter) Flush() {
	ml.mutex.Lock()
	defer ml.mutex.Unlock()

	tw := term.GetWidth()

	// Count the number of lines that need to be cleared. We need to take wrapping into account as well
	prevTotalLines := 0
	for _, line := range ml.prevLines {
		for _, sl := range strings.Split(line, "\n") {
			prevTotalLines += ml.countWrappedLines(sl, tw)
		}
	}
	if prevTotalLines > 0 {
		ml.clearLines(prevTotalLines)
	}

	if len(ml.topLines) != 0 {
		for _, l := range ml.topLines {
			_, _ = fmt.Fprintf(ml.w, "%s\n", l.s())
		}
		ml.topLines = nil
	}

	ml.prevLines = nil
	for _, l := range ml.lines {
		s := l.s()
		ml.prevLines = append(ml.prevLines, s)
		_, _ = fmt.Fprintf(ml.w, "%s\n", s)
	}
}

func (ml *MultiLinePrinter) countWrappedLines(s string, tw int) int {
	s = stripansi.Strip(s)
	w := runewidth.StringWidth(s)
	cnt := 1
	for w > tw {
		cnt++
		s = s[tw:]
		w = runewidth.StringWidth(s)
	}
	return cnt
}

func (ml *MultiLinePrinter) NewTopLine(s LineFunc) {
	ml.mutex.Lock()
	defer ml.mutex.Unlock()

	l := &Line{
		ml: ml,
		s:  s,
	}
	ml.topLines = append(ml.topLines, l)
}

func (ml *MultiLinePrinter) NewLine(s LineFunc) *Line {
	ml.mutex.Lock()
	defer ml.mutex.Unlock()

	l := &Line{
		ml: ml,
		s:  s,
	}
	ml.lines = append(ml.lines, l)

	return l
}

func (l *Line) Remove(pushToTop bool) {
	l.ml.mutex.Lock()
	defer l.ml.mutex.Unlock()

	for i, l2 := range l.ml.lines {
		if l2 == l {
			l.ml.lines = append(l.ml.lines[:i], l.ml.lines[i+1:]...)

			if pushToTop {
				l.ml.topLines = append(l.ml.topLines, l)
			}
			break
		}
	}
}
