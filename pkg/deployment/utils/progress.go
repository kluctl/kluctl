package utils

import (
	"fmt"
	"github.com/mattn/go-isatty"
	log "github.com/sirupsen/logrus"
	"github.com/vbauerster/mpb/v7"
	"github.com/vbauerster/mpb/v7/decor"
	"math"
	"os"
	"strings"
	"sync"
	"time"
)

type progressCtx struct {
	bar       *mpb.Bar
	total     int64
	name      string
	status    string
	startTime time.Time
	mutex     sync.Mutex
}

func NewProgressCtx(p *mpb.Progress, name string, maxNameWidth int) *progressCtx {
	pctx := &progressCtx{
		status:    "Initializing...",
		total:     -1,
		name:      name,
		startTime: time.Now(),
	}
	if !isatty.IsTerminal(os.Stderr.Fd()) || name == "" {
		return pctx
	}

	name += ":"
	if len(name) < maxNameWidth+1 {
		name += strings.Repeat(" ", maxNameWidth-len(name)+1)
	}
	name += " "

	if p != nil {
		pctx.bar = p.AddBar(-1,
			mpb.BarWidth(40),
			mpb.PrependDecorators(decor.Name(name)),
			mpb.PrependDecorators(decor.Any(pctx.ElapsedDecorFunc)),
			mpb.AppendDecorators(decor.Any(pctx.StatusDecorFunc)),
		)
	}
	return pctx
}

func (ctx *progressCtx) Logf(level log.Level, s string, args ...interface{}) {
	if ctx.bar == nil {
		s = fmt.Sprintf("%s: %s", ctx.name, s)
		log.StandardLogger().Logf(level, s, args...)
	}
}

func (ctx *progressCtx) Infof(s string, args ...interface{}) {
	ctx.Logf(log.InfoLevel, s, args...)
}

func (ctx *progressCtx) Warningf(s string, args ...interface{}) {
	ctx.Logf(log.WarnLevel, s, args...)
}

func (ctx *progressCtx) Debugf(s string, args ...interface{}) {
	ctx.Logf(log.DebugLevel, s, args...)
}

func (ctx *progressCtx) InfofAndStatus(s string, args ...interface{}) {
	ctx.Infof(s, args...)
	ctx.SetStatus(fmt.Sprintf(s, args...))
}

func (ctx *progressCtx) DebugfAndStatus(s string, args ...interface{}) {
	ctx.Debugf(s, args...)
	ctx.SetStatus(fmt.Sprintf(s, args...))
}

func (ctx *progressCtx) SetStatus(s string) {
	ctx.mutex.Lock()
	defer ctx.mutex.Unlock()
	ctx.status = s
}

func (ctx *progressCtx) StatusDecorFunc(st decor.Statistics) string {
	ctx.mutex.Lock()
	defer ctx.mutex.Unlock()
	return ctx.status
}

func (ctx *progressCtx) ElapsedDecorFunc(st decor.Statistics) string {
	s := fmt.Sprintf("%.3fs", time.Now().Sub(ctx.startTime).Seconds())
	if len(s) < 8 {
		s = strings.Repeat(" ", 8 - len(s)) + s
	}
	return s
}

func (ctx *progressCtx) SetTotal(total int64) {
	ctx.total = total
	if ctx.bar != nil {
		ctx.bar.SetTotal(total, false)
		ctx.bar.EnableTriggerComplete()
	}
}

func (ctx *progressCtx) Increment() {
	if ctx.bar != nil {
		ctx.bar.Increment()
	}
}

func (ctx *progressCtx) Finish() {
	if ctx.bar != nil {
		// make sure that the bar es rendered on top so that it can be properly popped
		ctx.bar.SetPriority(math.MinInt)
		ctx.bar.SetCurrent(ctx.total)
	}
}
