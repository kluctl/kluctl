package multiline

import (
	"bytes"
	"fmt"
	"github.com/gosuri/uilive"
	"io"
	"sync"
	"time"
)

type MultiLinePrinter struct {
	topLines []*Line
	lines    []*Line

	uil         *uilive.Writer
	lastWritten []byte

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
	ml.uil = uilive.New()
	ml.uil.Out = w
	ml.uil.Start()

	ml.ticker = time.NewTicker(100 * time.Millisecond)
	ml.tdone = make(chan bool)

	go ml.loop()
}

func (ml *MultiLinePrinter) Stop() {
	ml.Flush()
	ml.tdone <- true
	<-ml.tdone
	ml.uil.Stop()
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

	hadTopLines := false
	if len(ml.topLines) != 0 {
		hadTopLines = true
		topBuf := bytes.NewBuffer(nil)
		for _, l := range ml.topLines {
			_, _ = fmt.Fprintf(topBuf, "%s\n", l.s())
		}

		_, _ = ml.uil.Bypass().Write(topBuf.Bytes())
		ml.topLines = nil
	}

	linesBuf := bytes.NewBuffer(nil)
	for _, l := range ml.lines {
		_, _ = fmt.Fprintf(linesBuf, "%s\n", l.s())
	}
	newBytes := linesBuf.Bytes()
	if hadTopLines || !bytes.Equal(newBytes, ml.lastWritten) {
		_, _ = ml.uil.Write(newBytes)
		ml.lastWritten = newBytes
	}
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
