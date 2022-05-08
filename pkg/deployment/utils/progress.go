package utils

import (
	"context"
	"fmt"
	"github.com/kluctl/kluctl/v2/pkg/status"
	"github.com/mattn/go-isatty"
	"github.com/vbauerster/mpb/v7"
	"github.com/vbauerster/mpb/v7/decor"
	"math"
	"os"
	"strings"
	"sync"
	"time"
)

type progressCtx struct {
	ctx       context.Context
	bar       *mpb.Bar
	doLog     bool
	total     int64
	name      string
	status    string
	startTime time.Time
	mutex     sync.Mutex
}

func NewProgressCtx(ctx context.Context, p *mpb.Progress, name string, maxNameWidth int, doLog bool) *progressCtx {
	pctx := &progressCtx{
		ctx:       ctx,
		status:    "Initializing...",
		total:     -1,
		name:      name,
		startTime: time.Now(),
	}
	if !isatty.IsTerminal(os.Stderr.Fd()) || p == nil {
		pctx.doLog = doLog
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

func (ctx *progressCtx) Infof(s string, args ...interface{}) {
	if ctx.doLog {
		status.Info(ctx.ctx, s, args...)
	}
}

func (ctx *progressCtx) Warningf(s string, args ...interface{}) {
	if ctx.doLog {
		status.Warning(ctx.ctx, s, args...)
	}
}

func (ctx *progressCtx) Debugf(s string, args ...interface{}) {
	if ctx.doLog {
		status.Trace(ctx.ctx, s, args...)
	}
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
		s = strings.Repeat(" ", 8-len(s)) + s
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
