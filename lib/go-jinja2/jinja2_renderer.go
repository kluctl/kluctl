package jinja2

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/jinzhu/copier"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type pythonJinja2Renderer struct {
	j2 *Jinja2

	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout io.ReadCloser

	stdoutReader *bufio.Reader
}

func newPythonJinja2Renderer(j2 *Jinja2) (*pythonJinja2Renderer, error) {
	isOk := false
	j2r := &pythonJinja2Renderer{
		j2: j2,
	}
	defer func() {
		if !isOk {
			j2r.Close()
		}
	}()

	args := []string{filepath.Join(j2.rendererSrc.GetExtractedPath(), "main.py")}

	var err error
	j2r.cmd, err = j2.ep.PythonCmd(args...)
	if err != nil {
		return nil, err
	}
	j2r.cmd.Stderr = os.Stderr

	stdout, err := j2r.cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	j2r.stdout = stdout

	stdin, err := j2r.cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	j2r.stdin = stdin

	err = j2r.cmd.Start()
	if err != nil {
		return nil, fmt.Errorf("failed to start python for Jinja2: %w", err)
	}

	j2r.stdoutReader = bufio.NewReader(j2r.stdout)

	_, err = j2r.runCmd(&jinja2Cmd{
		Cmd: "init",
	})
	if err != nil {
		return nil, fmt.Errorf("failed to initialize renderer: %w", err)
	}

	isOk = true

	return j2r, nil
}

func (j *pythonJinja2Renderer) Close() {
	if j.stdin != nil {
		args := jinja2Cmd{Cmd: "exit"}
		_ = json.NewEncoder(j.stdin).Encode(args)

		_ = j.stdin.Close()
		j.stdin = nil
	}
	if j.stdout != nil {
		_ = j.stdout.Close()
		j.stdout = nil
	}
	if j.cmd != nil {
		if j.cmd.Process != nil {
			timer := time.AfterFunc(5*time.Second, func() {
				_ = j.cmd.Process.Kill()
			})
			_ = j.cmd.Wait()
			timer.Stop()
		}
		j.cmd = nil
	}
}

func (j *pythonJinja2Renderer) resolveAbsolutePath(template string, searchDirs []string) (string, bool) {
	if filepath.IsAbs(template) {
		return template, true
	}
	for _, s := range searchDirs {
		p := filepath.Join(s, template)
		st, err := os.Stat(p)
		if err != nil {
			continue
		}
		if !st.Mode().IsRegular() {
			continue
		}
		return p, true
	}
	return "", false
}

func isMaybeTemplateString(template string) bool {
	return strings.IndexRune(template, '{') != -1
}

func isMaybeTemplateBytes(template []byte) bool {
	return bytes.IndexRune(template, '{') != -1
}

func isMaybeTemplate(template string, isString bool) (bool, *string, error) {
	if isString {
		if !isMaybeTemplateString(template) {
			return false, &template, nil
		}
	} else {
		b, err := os.ReadFile(template)
		if err != nil {
			return false, nil, err
		}
		if !isMaybeTemplateBytes(b) {
			x := string(b)
			return false, &x, nil
		} else {
			return true, nil, nil
		}
	}
	return true, nil, nil
}

type jinja2Cmd struct {
	Cmd       string   `json:"cmd"`
	Templates []string `json:"templates"`

	Opts *jinja2Options `json:"opts"`
}

type jinja2TemplateResult struct {
	Result *string `json:"result,omitempty"`
	Error  *string `json:"error,omitempty"`
}

type jinja2CmdResult struct {
	TemplateResults []jinja2TemplateResult `json:"templateResults,omitempty"`
}

func (j *pythonJinja2Renderer) renderHelper(jobs []*RenderJob, isString bool, opts []Jinja2Opt) error {
	var jargs jinja2Cmd
	if isString {
		jargs.Cmd = "render-strings"
	} else {
		jargs.Cmd = "render-files"
	}

	jargs.Opts = &jinja2Options{}
	err := copier.CopyWithOption(jargs.Opts, &j.j2.defaultOptions, copier.Option{
		DeepCopy: true,
	})
	if err != nil {
		return err
	}

	for _, o := range opts {
		o(jargs.Opts)
	}

	var processedJobs []*RenderJob

	for _, job := range jobs {
		t := job.Template
		if !isString {
			p, ok := j.resolveAbsolutePath(t, jargs.Opts.SearchDirs)
			if !ok {
				job.Error = fmt.Errorf("absolute path of %s could not be resolved", t)
				continue
			}
			t = p
		}

		ist, r, err := isMaybeTemplate(t, isString)
		if err == nil && !ist {
			job.Result = r
			continue
		}

		processedJobs = append(processedJobs, job)
		jargs.Templates = append(jargs.Templates, t)
	}
	if len(processedJobs) == 0 {
		return nil
	}

	cmdResult, err := j.runCmd(&jargs)
	if err != nil {
		return fmt.Errorf("failed to run jinja2 cmd: %w", err)
	}

	for i, item := range cmdResult.TemplateResults {
		if item.Result != nil {
			processedJobs[i].Result = item.Result
		} else {
			if item.Error == nil {
				return fmt.Errorf("missing result and error from item at index %d", i)
			}
			processedJobs[i].Error = &Jinja2Error{*item.Error}
		}
	}

	return nil
}

func (j *pythonJinja2Renderer) runCmd(jargs *jinja2Cmd) (*jinja2CmdResult, error) {
	b, err := json.Marshal(jargs)
	if err != nil {
		j.Close()
		return nil, fmt.Errorf("failed to marshal jinja2 cmd args: %w", err)
	}
	b = append(b, '\n')

	if jargs.Opts != nil && jargs.Opts.traceJsonSend != nil {
		var m map[string]any
		_ = json.Unmarshal(b, &m)
		jargs.Opts.traceJsonSend(m)
	}

	_, err = j.stdin.Write(b)
	if err != nil {
		j.Close()
		return nil, fmt.Errorf("failed to write jinja2 cmd args: %w", err)
	}

	line, err := j.stdoutReader.ReadBytes('\n')
	if err != nil && err != io.EOF {
		return nil, fmt.Errorf("failed to read jinja2 cmd result: %w", err)
	}

	if jargs.Opts != nil && jargs.Opts.traceJsonReceive != nil {
		var m map[string]any
		_ = json.Unmarshal(line, &m)
		jargs.Opts.traceJsonReceive(m)
	}

	var result jinja2CmdResult
	err = json.Unmarshal(line, &result)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal jinja2 cmd result: %w", err)
	}

	return &result, nil
}
