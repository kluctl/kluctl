package jinja2_server

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/codablock/kluctl/pkg/utils"
	"github.com/codablock/kluctl/pkg/utils/uo"
	"github.com/gobwas/glob"
	log "github.com/sirupsen/logrus"
	"golang.org/x/sync/semaphore"
	"google.golang.org/grpc"
	"io/fs"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
)

type Jinja2Error struct {
	error string
}

func (m *Jinja2Error) Error() string {
	return m.error
}

type Jinja2Server struct {
	serverPath string
	pythonVenv string
	cmd        *exec.Cmd

	port   int
	conn   *grpc.ClientConn
	client Jinja2ServerClient
	sem    *semaphore.Weighted

	globCache      map[string]interface{}
	globCacheMutex sync.Mutex
}

func NewJinja2Server() (*Jinja2Server, error) {
	js := &Jinja2Server{
		sem:       semaphore.NewWeighted(8),
		globCache: map[string]interface{}{},
	}

	serverPath, ok := os.LookupEnv("JINJA2_SERVER")
	if !ok {
		executable, err := os.Executable()
		if err != nil {
			log.Fatal(err)
		}
		serverPath = path.Join(path.Dir(executable), "jinja2-server")
	}

	js.serverPath = serverPath
	js.pythonVenv = path.Join(js.serverPath, "venv")

	cmdName := path.Join(js.pythonVenv, "bin/python")
	args := []string{"main.py"}

	args = append(args, "serve")
	js.cmd = exec.Command(cmdName, args...)
	js.cmd.Dir = js.serverPath

	stdout, err := js.cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}

	err = js.cmd.Start()
	if err != nil {
		_ = stdout.Close()
		return nil, err
	}

	s := bufio.NewScanner(stdout)
	if !s.Scan() {
		_ = js.cmd.Process.Kill()
		return nil, fmt.Errorf("failed to determine jinja2-server port")
	}

	port, err := strconv.ParseInt(s.Text(), 10, 32)
	if err != nil {
		_ = js.cmd.Process.Kill()
		return nil, fmt.Errorf("failed to parse port: %w", err)
	}

	js.port = int(port)
	js.conn, err = grpc.Dial(fmt.Sprintf("localhost:%d", js.port), grpc.WithInsecure())
	if err != nil {
		_ = js.cmd.Process.Kill()
		return nil, err
	}
	js.client = NewJinja2ServerClient(js.conn)

	return js, nil
}

func (js *Jinja2Server) Stop() error {
	_ = js.conn.Close()
	return js.cmd.Process.Kill()
}

type RenderJob struct {
	Template string
	Result   *string
	Error    error

	target string
}

func (js *Jinja2Server) isMaybeTemplate(template string, searchDirs []string, isString bool) (bool, *string) {
	if isString {
		if strings.IndexRune(template, '{') == -1 {
			return false, &template
		}
	} else {
		for _, s := range searchDirs {
			b, err := ioutil.ReadFile(path.Join(s, template))
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

func (js *Jinja2Server) renderHelper(jobs []*RenderJob, searchDirs []string, vars *uo.UnstructuredObject, isString bool) error {
	varsStr, err := json.Marshal(vars.Object)
	if err != nil {
		return err
	}

	var processedJobs []*RenderJob
	var templates []string
	for _, job := range jobs {
		if ist, r := js.isMaybeTemplate(job.Template, searchDirs, isString); !ist {
			job.Result = r
			continue
		}
		processedJobs = append(processedJobs, job)
		templates = append(templates, job.Template)
	}
	if len(templates) == 0 {
		return nil
	}

	err = js.sem.Acquire(context.Background(), 1)
	if err != nil {
		return err
	}
	defer js.sem.Release(1)

	var result *JobResult
	if isString {
		request := &StringsJob{
			Vars:       string(varsStr),
			Templates:  templates,
			SearchDirs: searchDirs,
		}
		result, err = js.client.RenderStrings(context.Background(), request)
	} else {
		request := &FilesJob{
			Vars:       string(varsStr),
			Templates:  templates,
			SearchDirs: searchDirs,
		}
		result, err = js.client.RenderFiles(context.Background(), request)
	}
	if err != nil {
		return err
	}

	for i, r := range result.Results {
		if r.Error != nil {
			processedJobs[i].Error = &Jinja2Error{error: *r.Error}
		} else {
			processedJobs[i].Result = r.Result
		}
	}
	return nil
}

func (js *Jinja2Server) RenderStrings(jobs []*RenderJob, searchDirs []string, vars *uo.UnstructuredObject) error {
	return js.renderHelper(jobs, searchDirs, vars, true)
}

func (js *Jinja2Server) RenderFiles(jobs []*RenderJob, searchDirs []string, vars *uo.UnstructuredObject) error {
	return js.renderHelper(jobs, searchDirs, vars, false)
}

func (js *Jinja2Server) RenderString(template string, searchDirs []string, vars *uo.UnstructuredObject) (string, error) {
	jobs := []*RenderJob{{
		Template: template,
	}}
	err := js.RenderStrings(jobs, searchDirs, vars)
	if err != nil {
		return "", err
	}
	if jobs[0].Error != nil {
		return "", jobs[0].Error
	}
	return *jobs[0].Result, nil
}

func (js *Jinja2Server) RenderFile(template string, searchDirs []string, vars *uo.UnstructuredObject) (string, error) {
	jobs := []*RenderJob{{
		Template: template,
	}}
	err := js.RenderFiles(jobs, searchDirs, vars)
	if err != nil {
		return "", err
	}
	if jobs[0].Error != nil {
		return "", jobs[0].Error
	}
	return *jobs[0].Result, nil
}

func (js *Jinja2Server) RenderStruct(dst interface{}, src interface{}, vars *uo.UnstructuredObject) error {
	m, err := uo.FromStruct(src)

	type pk struct {
		parent interface{}
		key    interface{}
	}

	var jobs []*RenderJob
	var fields []pk
	err = m.NewIterator().IterateLeafs(func(it *uo.ObjectIterator) error {
		value := it.Value()
		if s, ok := value.(string); ok {
			jobs = append(jobs, &RenderJob{Template: s})
			fields = append(fields, pk{
				parent: it.Parent(),
				key:    it.Key(),
			})
		}
		return nil
	})
	if err != nil {
		return err
	}

	err = js.RenderStrings(jobs, nil, vars)
	if err != nil {
		return err
	}

	var errors []error
	for i, j := range jobs {
		if j.Error != nil {
			errors = append(errors, err)
		}

		err = uo.SetChild(fields[i].parent, fields[i].key, *j.Result)
		if err != nil {
			return err
		}
	}
	if len(errors) != 0 {
		return utils.NewErrorList(errors)
	}

	err = m.ToStruct(dst)
	if err != nil {
		return err
	}
	return nil
}

func (js *Jinja2Server) getGlob(pattern string) (glob.Glob, error) {
	js.globCacheMutex.Lock()
	defer js.globCacheMutex.Unlock()

	g, ok := js.globCache[pattern]
	if ok {
		if g2, ok := g.(glob.Glob); ok {
			return g2, nil
		} else {
			return nil, g2.(error)
		}
	}
	g, err := glob.Compile(pattern)
	if err != nil {
		js.globCache[pattern] = err
		return nil, err
	}
	js.globCache[pattern] = g
	return g.(glob.Glob), nil
}
func (js *Jinja2Server) needsRender(path string, excludedPatterns []string) bool {
	for _, p := range excludedPatterns {
		g, err := js.getGlob(p)
		if err != nil {
			return false
		}
		if g.Match(path) {
			return false
		}
	}
	return true
}

func (js *Jinja2Server) RenderDirectory(rootDir string, searchDirs []string, relSourceDir string, excludePatterns []string, subdir string, targetDir string, vars *uo.UnstructuredObject) error {
	walkDir := path.Join(rootDir, relSourceDir, subdir)

	var jobs []*RenderJob

	err := filepath.WalkDir(walkDir, func(p string, d fs.DirEntry, err error) error {
		relPath, err := filepath.Rel(walkDir, p)
		if err != nil {
			return err
		}
		if d.IsDir() {
			err = os.MkdirAll(path.Join(targetDir, relPath), 0o777)
			if err != nil {
				return err
			}
			return nil
		}

		sourcePath := path.Clean(path.Join(subdir, relPath))
		targetPath := path.Join(targetDir, relPath)

		if strings.Index(sourcePath, ".sealme") != -1 {
			sourcePath += ""
		}

		if !js.needsRender(sourcePath, excludePatterns) {
			return utils.CopyFile(p, targetPath)
		}

		// jinja2 templates are using / even on Windows
		sourcePath = strings.ReplaceAll(sourcePath, "\\", "/")

		job := &RenderJob{
			Template: sourcePath,
			target:   targetPath,
		}
		jobs = append(jobs, job)
		return nil
	})
	if err != nil {
		return err
	}

	err = js.RenderFiles(jobs, searchDirs, vars)
	if err != nil {
		return err
	}

	var errors []error
	for _, job := range jobs {
		if job.Error != nil {
			errors = append(errors, job.Error)
			continue
		}

		err = ioutil.WriteFile(job.target, []byte(*job.Result), 0o666)
		if err != nil {
			return err
		}
	}
	if len(errors) != 0 {
		return utils.NewErrorList(errors)
	}

	return nil
}
