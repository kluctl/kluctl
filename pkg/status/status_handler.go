package status

import (
	"context"
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
	total   int
	bar     *mpb.Bar
	filler  mpb.BarFiller
	message string

	barOverride string
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

func (s *MultiLineStatusHandler) Stop() {
	s.progress.Wait()
}

func (s *MultiLineStatusHandler) StartStatus(total int, message string) StatusLine {
	return s.startStatus(total, message, 0, "")
}

func (s *MultiLineStatusHandler) startStatus(total int, message string, priority int, barOverride string) *statusLine {
	sl := &statusLine{
		slh:         s,
		total:       total,
		message:     message,
		barOverride: barOverride,
	}
	sl.filler = mpb.SpinnerStyle().PositionLeft().Build()

	opts := []mpb.BarOption{
		mpb.BarWidth(1),
		mpb.AppendDecorators(decor.Any(sl.DecorMessage, decor.WCSyncWidthR)),
	}
	if priority != 0 {
		opts = append(opts, mpb.BarPriority(priority))
	}

	sl.bar = s.progress.Add(int64(total), sl, opts...)

	return sl
}

func (sl *statusLine) DecorMessage(s decor.Statistics) string {
	return sl.message
}

func (sl *statusLine) Fill(w io.Writer, reqWidth int, stat decor.Statistics) {
	if sl.barOverride != "" {
		_, _ = io.WriteString(w, sl.barOverride)
		return
	}

	sl.filler.Fill(w, reqWidth, stat)
}

func (s *MultiLineStatusHandler) Info(message string) {
	s.startStatus(1, message, math.MinInt, "ⓘ").end("ⓘ")
}

func (s *MultiLineStatusHandler) InfoFallback(message string) {
	// no fallback needed
}

func (s *MultiLineStatusHandler) Warning(message string) {
	s.startStatus(1, message, math.MinInt, "⚠").end("⚠")
}

func (s *MultiLineStatusHandler) Error(message string) {
	s.startStatus(1, message, math.MinInt, "✗").end("✗")
}

func (s *MultiLineStatusHandler) Trace(message string) {
	if s.trace {
		s.Info(message)
	}
}

func (s *MultiLineStatusHandler) PlainText(text string) {
	s.Info(text)
}

func (sl *statusLine) SetTotal(total int) {
	sl.total = total
	sl.bar.SetTotal(int64(total), false)
	sl.bar.EnableTriggerComplete()
}

func (sl *statusLine) Increment() {
	sl.bar.Increment()
}

func (sl *statusLine) Update(message string) {
	sl.message = message
}

func (sl *statusLine) end(barOverride string) {
	sl.barOverride = barOverride
	// make sure that the bar es rendered on top so that it can be properly popped
	sl.bar.SetPriority(math.MinInt)
	sl.bar.SetCurrent(int64(sl.total))
}

func (sl *statusLine) End(success bool) {
	if success {
		sl.end("✓")
	} else {
		sl.end("✗")
	}
}
