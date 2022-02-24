package jinja2

import "C"
import (
	"github.com/codablock/kluctl/pkg/utils"
	"github.com/codablock/kluctl/pkg/utils/uo"
	"github.com/gobwas/glob"
	"io/fs"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

//go:generate bash ./python_src/pip-wheel.sh

type Jinja2 struct {
	pj        *pythonJinja2Renderer
	globCache map[string]interface{}
	mutex     sync.Mutex
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
	pj, err := newPythonJinja2Renderer()
	if err != nil {
		return nil, err
	}

	j := &Jinja2{
		pj:        pj,
		globCache: map[string]interface{}{},
	}

	return j, nil
}

func (j *Jinja2) Close() {
	j.pj.Close()
}

func (j *Jinja2) RenderStrings(jobs []*RenderJob, searchDirs []string, vars *uo.UnstructuredObject) error {
	j.mutex.Lock()
	defer j.mutex.Unlock()
	return j.pj.renderHelper(jobs, searchDirs, vars, true)
}

func (j *Jinja2) RenderFiles(jobs []*RenderJob, searchDirs []string, vars *uo.UnstructuredObject) error {
	j.mutex.Lock()
	defer j.mutex.Unlock()
	return j.pj.renderHelper(jobs, searchDirs, vars, false)
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
			errors = append(errors, j.Error)
		} else {
			err = uo.SetChild(fields[i].parent, fields[i].key, *j.Result)
			if err != nil {
				return err
			}
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
	j.mutex.Lock()
	defer j.mutex.Unlock()

	g, ok := j.globCache[pattern]
	if ok {
		if g2, ok := g.(glob.Glob); ok {
			return g2, nil
		} else {
			return nil, g2.(error)
		}
	}
	g, err := glob.Compile(pattern, '/')
	if err != nil {
		j.globCache[pattern] = err
		return nil, err
	}
	j.globCache[pattern] = g
	return g.(glob.Glob), nil
}
func (j *Jinja2) needsRender(path string, excludedPatterns []string) bool {
	path = strings.ReplaceAll(path, string(os.PathSeparator), "/")

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
