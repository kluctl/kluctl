package utils

import (
	"fmt"
	"github.com/mattn/go-isatty"
	log "github.com/sirupsen/logrus"
	"github.com/vbauerster/mpb/v7"
	"github.com/vbauerster/mpb/v7/decor"
	"os"
	"strings"
	"sync"
)

type progressCtx struct {
	bar    *mpb.Bar
	total  int64
	name   string
	status string
	mutex  sync.Mutex
}

func NewProgressCtx(p *mpb.Progress, name string) *progressCtx {
	pctx := &progressCtx{
		status: "Initializing...",
		total:  -1,
		name:   name,
	}
	if !isatty.IsTerminal(os.Stderr.Fd()) || name == "" {
		return pctx
	}
	if p != nil {
		pctx.bar = p.AddBar(-1,
			mpb.BarWidth(40),
			mpb.PrependDecorators(decor.Name(name+": ", decor.WCSyncSpaceR)),
			mpb.AppendDecorators(decor.Any(pctx.DecorFunc)),
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

func (ctx *progressCtx) DecorFunc(st decor.Statistics) string {
	ctx.mutex.Lock()
	defer ctx.mutex.Unlock()
	fillerLen := st.AvailableWidth - len(ctx.status) - 40
	if fillerLen < 0 {
		fillerLen = 0
	}
	return fmt.Sprintf("%s%s", ctx.status, strings.Repeat(" ", fillerLen))
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
		ctx.bar.SetCurrent(ctx.total)
	}
}
