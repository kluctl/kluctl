package status

import (
	"context"
	"fmt"
	"github.com/kluctl/kluctl/v2/pkg/utils"
	"github.com/vbauerster/mpb/v7"
	"github.com/vbauerster/mpb/v7/decor"
	"io"
	"math"
)

type MultiLineStatusHandler struct {
	out      io.Writer
	progress *mpb.Progress
	stop     chan any
	trace    bool
}

type statusLine struct {
	slh     *MultiLineStatusHandler
	bar     *mpb.Bar
	filler  mpb.BarFiller
	message string

	finished bool
	success  bool
}

func NewMultiLineStatusHandler(ctx context.Context, out io.Writer, trace bool) *MultiLineStatusHandler {
	sh := &MultiLineStatusHandler{
		out:   out,
		trace: trace,
		stop:  make(chan any),
	}

	sh.progress = mpb.NewWithContext(
		ctx,
		mpb.WithWidth(utils.GetTermWidth()),
		mpb.WithOutput(out),
		mpb.PopCompletedMode(),
	)

	return sh
}

func (s *MultiLineStatusHandler) StartStatus(message string) StatusLine {
	sl := &statusLine{
		slh:     s,
		message: message,
	}
	sl.filler = mpb.SpinnerStyle().PositionLeft().Build()
	sl.bar = s.progress.Add(1, sl,
		mpb.BarWidth(1),
		mpb.AppendDecorators(decor.Any(sl.DecorMessage, decor.WCSyncWidthR)),
	)

	s.writeLog(sl.message)
	return sl
}

func (sl *statusLine) DecorMessage(s decor.Statistics) string {
	return sl.message
}

func (sl *statusLine) Fill(w io.Writer, reqWidth int, stat decor.Statistics) {
	if !sl.finished {
		sl.filler.Fill(w, reqWidth, stat)
	} else if sl.success {
		fmt.Fprintf(w, "✓")
	} else {
		fmt.Fprintf(w, "✗")
	}
}

func (s *MultiLineStatusHandler) writeLog(message string) {
	//_, _ = fmt.Fprintf(s.out, "%s\n", message)
}

func (s *MultiLineStatusHandler) Info(message string) {
	s.writeLog(message)
}

func (s *MultiLineStatusHandler) Warning(message string) {
	s.writeLog(message)
}

func (s *MultiLineStatusHandler) Error(message string) {
	s.writeLog(message)
}

func (s *MultiLineStatusHandler) Trace(message string) {
	if s.trace {
		s.writeLog(message)
	}
}

func (s *MultiLineStatusHandler) PlainText(text string) {
	s.writeLog(text)
}

func (sl *statusLine) Update(message string) {
	sl.message = message
	sl.slh.writeLog(sl.message)
}

func (sl *statusLine) End(success bool) {
	sl.finished = true
	sl.success = success
	// make sure that the bar es rendered on top so that it can be properly popped
	sl.bar.SetPriority(math.MinInt)
	sl.bar.Increment()
}
