package commands

import (
	"context"
	"io"
	"os"
)

type overrideStdStreamsKey int
type overrideStdStreamsValue struct {
	stdout io.Writer
	stderr io.Writer
}

var overrideStdStreamsKeyInst overrideStdStreamsKey

func WithStdStreams(ctx context.Context, stdout io.Writer, stderr io.Writer) context.Context {
	return context.WithValue(ctx, overrideStdStreamsKeyInst, &overrideStdStreamsValue{
		stdout: stdout,
		stderr: stderr,
	})
}

func getStdStreams(ctx context.Context) (io.Writer, io.Writer) {
	v := ctx.Value(overrideStdStreamsKeyInst)
	if v == nil {
		return os.Stdout, os.Stderr
	}
	v2 := v.(*overrideStdStreamsValue)
	return v2.stdout, v2.stderr
}

type stringWriter struct {
	w io.Writer
}

func (w *stringWriter) WriteString(s string) (n int, err error) {
	return w.w.Write([]byte(s))
}

func (w *stringWriter) Write(b []byte) (n int, err error) {
	return w.w.Write(b)
}

func getStdout(ctx context.Context) *stringWriter {
	stdout, _ := getStdStreams(ctx)
	return &stringWriter{stdout}
}

func getStderr(ctx context.Context) *stringWriter {
	_, stderr := getStdStreams(ctx)
	return &stringWriter{stderr}
}
