package jinja2

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/kluctl/kluctl/pkg/python"
	"github.com/kluctl/kluctl/pkg/utils/uo"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type pythonJinja2Renderer struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout io.ReadCloser

	stdoutReader *bufio.Reader
}

func newPythonJinja2Renderer() (*pythonJinja2Renderer, error) {
	isOk := false
	j := &pythonJinja2Renderer{}
	defer func() {
		if !isOk {
			j.Close()
		}
	}()

	args := []string{filepath.Join(pythonSrcExtracted, "main.py")}
	j.cmd = python.PythonCmd(args)
	j.cmd.Stderr = os.Stderr
	j.cmd.Env = append(j.cmd.Env, fmt.Sprintf("PYTHONPATH=%s/wheel", pythonSrcExtracted))

	stdout, err := j.cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	j.stdout = stdout

	stdin, err := j.cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	j.stdin = stdin

	err = j.cmd.Start()
	if err != nil {
		return nil, err
	}

	j.stdoutReader = bufio.NewReader(j.stdout)

	isOk = true

	return j, nil
}

func (j *pythonJinja2Renderer) Close() {
	if j.stdin != nil {
		args := jinja2Args{Cmd: "exit"}
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
			_ = j.cmd.Process.Kill()
		}
		j.cmd = nil
	}
}

func (j *pythonJinja2Renderer) isMaybeTemplate(template string, searchDirs []string, isString bool) (bool, *string) {
	if isString {
		if strings.IndexRune(template, '{') == -1 {
			return false, &template
		}
	} else {
		for _, s := range searchDirs {
			b, err := ioutil.ReadFile(filepath.Join(s, template))
			if err != nil {
				continue
			}
			if bytes.IndexRune(b, '{') == -1 {
				x := string(b)
				return false, &x
			} else {
				return true, nil
			}
		}
	}
	return true, nil
}

type jinja2Args struct {
	Cmd        string   `json:"cmd"`
	Templates  []string `json:"templates"`
	SearchDirs []string `json:"searchDirs"`
	Vars       string   `json:"vars"`
}

type jinja2Result struct {
	Result *string `json:"result,omitempty"`
	Error  *string `json:"error,omitempty"`
}

func (j *pythonJinja2Renderer) renderHelper(jobs []*RenderJob, searchDirs []string, vars *uo.UnstructuredObject, isString bool) error {
	varsStr, err := json.Marshal(vars.Object)
	if err != nil {
		return err
	}

	var processedJobs []*RenderJob

	var jargs jinja2Args
	if isString {
		jargs.Cmd = "render-strings"
	} else {
		jargs.Cmd = "render-files"
	}
	jargs.Vars = string(varsStr)
	jargs.SearchDirs = searchDirs

	for _, job := range jobs {
		if ist, r := j.isMaybeTemplate(job.Template, searchDirs, isString); !ist {
			job.Result = r
			continue
		}
		processedJobs = append(processedJobs, job)
		jargs.Templates = append(jargs.Templates, job.Template)
	}
	if len(processedJobs) == 0 {
		return nil
	}

	b, err := json.Marshal(jargs)
	if err != nil {
		j.Close()
		return err
	}
	b = append(b, '\n')

	_, err = j.stdin.Write(b)
	if err != nil {
		j.Close()
		return err
	}

	line := bytes.NewBuffer(nil)
	for true {
		l, p, err := j.stdoutReader.ReadLine()
		if err != nil {
			return err
		}
		line.Write(l)
		if !p {
			break
		}
	}
	var result []jinja2Result
	err = json.Unmarshal(line.Bytes(), &result)
	if err != nil {
		return err
	}

	for i, item := range result {
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
