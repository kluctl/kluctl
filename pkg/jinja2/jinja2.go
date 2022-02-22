package jinja2

import "C"
import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/codablock/kluctl/pkg/python"
	"github.com/codablock/kluctl/pkg/utils"
	"github.com/codablock/kluctl/pkg/utils/uo"
	"github.com/gobwas/glob"
	"golang.org/x/sync/semaphore"
	"io/fs"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

type Jinja2 struct {
	srcDir   string
	p        *python.PythonInterpreter
	renderer *python.PyObject

	sem            *semaphore.Weighted
	globCache      map[string]interface{}
	globCacheMutex sync.Mutex
}

type RenderJob struct {
	Template string
	Result   *string
	Error    error

	target string
}

type Jinja2Error struct {
	error string
}

func (m *Jinja2Error) Error() string {
	return m.error
}

func NewJinja2() (*Jinja2, error) {
	srcDir, err := extractSource()
	if err != nil {
		return nil, err
	}
	isOk := false
	defer func() {
		if !isOk {
			_ = os.RemoveAll(srcDir)
		}
	}()

	p, err := python.NewPythonInterpreter()
	if err != nil {
		return nil, err
	}
	defer func() {
		if !isOk {
			p.Stop()
		}
	}()

	err = p.AppendSysPath(srcDir)
	if err != nil {
		return nil, err
	}

	j := &Jinja2{
		srcDir:    srcDir,
		p:         p,
		sem:       semaphore.NewWeighted(1),
		globCache: map[string]interface{}{},
	}

	err = p.Run(func() error {
		mod := python.PyImport_ImportModule("jinja2_renderer")
		if mod == nil {
			python.PyErr_Print()
			return fmt.Errorf("unexpected error")
		}
		defer mod.DecRef()

		clazz := mod.GetAttrString("Jinja2Renderer")
		if clazz == nil {
			python.PyErr_Print()
			return fmt.Errorf("unexpected error")
		}
		defer clazz.DecRef()

		renderer := clazz.CallObject(nil)
		if renderer == nil {
			python.PyErr_Print()
			return fmt.Errorf("unexpected error")
		}

		j.renderer = renderer
		return nil
	})
	if err != nil {
		return nil, err
	}

	isOk = true
	return j, nil
}

func (j *Jinja2) Close() {
	_ = j.p.Run(func() error {
		if j.renderer != nil {
			j.renderer.DecRef()
			j.renderer = nil
		}
		return nil
	})

	j.p.Stop()

	_ = os.RemoveAll(j.srcDir)
}

func (j *Jinja2) isMaybeTemplate(template string, searchDirs []string, isString bool) (bool, *string) {
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

func (j *Jinja2) renderHelper(jobs []*RenderJob, searchDirs []string, vars *uo.UnstructuredObject, isString bool) error {
	err := j.sem.Acquire(context.Background(), 1)
	if err != nil {
		return err
	}
	defer j.sem.Release(1)

	return j.p.Run(func() error {
		return j.renderHelperNoLock(jobs, searchDirs, vars, isString)
	})
}

func (j *Jinja2) renderHelperNoLock(jobs []*RenderJob, searchDirs []string, vars *uo.UnstructuredObject, isString bool) error {
	varsStr, err := json.Marshal(vars.Object)
	if err != nil {
		return err
	}

	var processedJobs []*RenderJob

	pyTemplates := python.PyList_New(0)
	pySearchDirs := python.PyList_New(0)
	pyVars := python.PyUnicode_FromString(string(varsStr))

	defer func() {
		for i := 0; i < pyTemplates.Length(); i++ {
			pyTemplates.GetItem(i).DecRef()
		}
		for i := 0; i < pySearchDirs.Length(); i++ {
			pySearchDirs.GetItem(i).DecRef()
		}
		pyTemplates.DecRef()
		pySearchDirs.DecRef()
		pyVars.DecRef()
	}()

	for _, job := range jobs {
		if ist, r := j.isMaybeTemplate(job.Template, searchDirs, isString); !ist {
			job.Result = r
			continue
		}
		processedJobs = append(processedJobs, job)
		pyTemplates.Append(python.PyUnicode_FromString(job.Template))
	}
	if len(processedJobs) == 0 {
		return nil
	}

	for _, sd := range searchDirs {
		pySearchDirs.Append(python.PyUnicode_FromString(sd))
	}

	var pyFuncName *python.PyObject
	var pyResult *python.PyObject
	if isString {
		pyFuncName = python.PyUnicode_FromString("RenderStrings")
	} else {
		pyFuncName = python.PyUnicode_FromString("RenderFiles")
	}
	defer pyFuncName.DecRef()
	pyResult = python.PyList_FromObject(j.renderer.CallMethodObjArgs(pyFuncName, pyTemplates, pySearchDirs, pyVars))
	if pyResult == nil {
		python.PyErr_Print()
		return fmt.Errorf("unexpected exception while calling python code: %w", err)
	}

	for i := 0; i < pyResult.Length(); i++ {
		item := python.PyDict_FromObject(pyResult.GetItem(i))
		r := item.GetItemString("result")
		if r != nil {
			resultStr := python.PyUnicode_AsUTF8(r)
			processedJobs[i].Result = &resultStr
		} else {
			e := item.GetItemString("error")
			if e == nil {
				return fmt.Errorf("missing result and error from item at index %d", i)
			}
			eStr := python.PyUnicode_AsUTF8(e)
			processedJobs[i].Error = &Jinja2Error{eStr}
		}
	}

	return nil
}

func (j *Jinja2) RenderStrings(jobs []*RenderJob, searchDirs []string, vars *uo.UnstructuredObject) error {
	return j.renderHelper(jobs, searchDirs, vars, true)
}

func (j *Jinja2) RenderFiles(jobs []*RenderJob, searchDirs []string, vars *uo.UnstructuredObject) error {
	return j.renderHelper(jobs, searchDirs, vars, false)
}

func (j *Jinja2) RenderString(template string, searchDirs []string, vars *uo.UnstructuredObject) (string, error) {
	jobs := []*RenderJob{{
		Template: template,
	}}
	err := j.RenderStrings(jobs, searchDirs, vars)
	if err != nil {
		return "", err
	}
	if jobs[0].Error != nil {
		return "", jobs[0].Error
	}
	return *jobs[0].Result, nil
}

func (j *Jinja2) RenderFile(template string, searchDirs []string, vars *uo.UnstructuredObject) (string, error) {
	jobs := []*RenderJob{{
		Template: template,
	}}
	err := j.RenderFiles(jobs, searchDirs, vars)
	if err != nil {
		return "", err
	}
	if jobs[0].Error != nil {
		return "", jobs[0].Error
	}
	return *jobs[0].Result, nil
}

func (j *Jinja2) RenderStruct(dst interface{}, src interface{}, vars *uo.UnstructuredObject) error {
	m, err := uo.FromStruct(src)
	if err != nil {
		return err
	}

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

	err = j.RenderStrings(jobs, nil, vars)
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

func (j *Jinja2) getGlob(pattern string) (glob.Glob, error) {
	j.globCacheMutex.Lock()
	defer j.globCacheMutex.Unlock()

	g, ok := j.globCache[pattern]
	if ok {
		if g2, ok := g.(glob.Glob); ok {
			return g2, nil
		} else {
			return nil, g2.(error)
		}
	}
	g, err := glob.Compile(pattern)
	if err != nil {
		j.globCache[pattern] = err
		return nil, err
	}
	j.globCache[pattern] = g
	return g.(glob.Glob), nil
}
func (j *Jinja2) needsRender(path string, excludedPatterns []string) bool {
	for _, p := range excludedPatterns {
		g, err := j.getGlob(p)
		if err != nil {
			return false
		}
		if g.Match(path) {
			return false
		}
	}
	return true
}

func (j *Jinja2) RenderDirectory(rootDir string, searchDirs []string, relSourceDir string, excludePatterns []string, subdir string, targetDir string, vars *uo.UnstructuredObject) error {
	walkDir := filepath.Join(rootDir, relSourceDir, subdir)

	var jobs []*RenderJob

	err := filepath.WalkDir(walkDir, func(p string, d fs.DirEntry, err error) error {
		relPath, err := filepath.Rel(walkDir, p)
		if err != nil {
			return err
		}
		if d.IsDir() {
			err = os.MkdirAll(filepath.Join(targetDir, relPath), 0o700)
			if err != nil {
				return err
			}
			return nil
		}

		sourcePath := filepath.Clean(filepath.Join(subdir, relPath))
		targetPath := filepath.Join(targetDir, relPath)

		if strings.Index(sourcePath, ".sealme") != -1 {
			sourcePath += ""
		}

		if !j.needsRender(sourcePath, excludePatterns) {
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

	err = j.RenderFiles(jobs, searchDirs, vars)
	if err != nil {
		return err
	}

	var errors []error
	for _, job := range jobs {
		if job.Error != nil {
			errors = append(errors, job.Error)
			continue
		}

		err = ioutil.WriteFile(job.target, []byte(*job.Result), 0o600)
		if err != nil {
			return err
		}
	}
	if len(errors) != 0 {
		return utils.NewErrorList(errors)
	}

	return nil
}
