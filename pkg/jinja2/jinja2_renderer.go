package jinja2

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/codablock/kluctl/pkg/python"
	"github.com/codablock/kluctl/pkg/utils/uo"
	"io/ioutil"
	"log"
	"path/filepath"
	"strings"
	"sync"
)

type pythonJinja2Renderer struct {
	p        *python.PythonInterpreter
	renderer *python.PyObject
}

var addSrcToPathOnce sync.Once

func addSrcToPath() {
	addSrcToPathOnce.Do(func() {
		p, err := python.MainPythonInterpreter()
		if err != nil {
			log.Fatal(err)
		}
		err = p.AppendSysPath(pythonSrcExtracted)
		if err != nil {
			log.Fatal(err)
		}
		err = p.AppendSysPath(filepath.Join(pythonSrcExtracted, "wheel"))
		if err != nil {
			log.Fatal(err)
		}
	})
}

func newPythonJinja2Renderer() (*pythonJinja2Renderer, error) {
	addSrcToPath()

	isOk := false
	p, err := python.MainPythonInterpreter()
	if err != nil {
		return nil, err
	}
	defer func() {
		if !isOk {
			p.Stop()
		}
	}()

	j := &pythonJinja2Renderer{
		p: p,
	}

	err = p.Run(func() error {
		mod := python.PythonWrapper.PyImport_ImportModule("jinja2_renderer")
		if mod == nil {
			python.PythonWrapper.PyErr_Print()
			return fmt.Errorf("unexpected error")
		}
		defer mod.DecRef()

		clazz := mod.GetAttrString("Jinja2Renderer")
		if clazz == nil {
			python.PythonWrapper.PyErr_Print()
			return fmt.Errorf("unexpected error")
		}
		defer clazz.DecRef()

		renderer := clazz.CallObject(nil)
		if renderer == nil {
			python.PythonWrapper.PyErr_Print()
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

func (j *pythonJinja2Renderer) Close() {
	_ = j.p.Run(func() error {
		if j.renderer != nil {
			j.renderer.DecRef()
			j.renderer = nil
		}
		return nil
	})

	j.p.Stop()
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

func (j *pythonJinja2Renderer) renderHelper(jobs []*RenderJob, searchDirs []string, vars *uo.UnstructuredObject, isString bool) error {
	return j.p.Run(func() error {
		return j.renderHelper2(jobs, searchDirs, vars, isString)
	})
}

func (j *pythonJinja2Renderer) renderHelper2(jobs []*RenderJob, searchDirs []string, vars *uo.UnstructuredObject, isString bool) error {
	varsStr, err := json.Marshal(vars.Object)
	if err != nil {
		return err
	}

	var processedJobs []*RenderJob

	pyTemplates := python.PyList_New(0)
	pySearchDirs := python.PyList_New(0)
	pyVars := python.PythonWrapper.PyUnicode_FromString(string(varsStr))

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
		pyTemplates.Append(python.PythonWrapper.PyUnicode_FromString(job.Template))
	}
	if len(processedJobs) == 0 {
		return nil
	}

	for _, sd := range searchDirs {
		pySearchDirs.Append(python.PythonWrapper.PyUnicode_FromString(sd))
	}

	var pyFuncName *python.PyObject
	var pyResult *python.PyList
	if isString {
		pyFuncName = python.PythonWrapper.PyUnicode_FromString("RenderStrings")
	} else {
		pyFuncName = python.PythonWrapper.PyUnicode_FromString("RenderFiles")
	}
	defer pyFuncName.DecRef()
	pyResult = python.PyList_FromObject(j.renderer.CallMethodObjArgs(pyFuncName, pyTemplates, pySearchDirs, pyVars))
	if pyResult == nil {
		python.PythonWrapper.PyErr_Print()
		return fmt.Errorf("unexpected exception while calling python code: %w", err)
	}

	for i := 0; i < pyResult.Length(); i++ {
		item := python.PyDict_FromObject(pyResult.GetItem(i))
		r := item.GetItemString("result")
		if r != nil {
			resultStr := python.PythonWrapper.PyUnicode_AsUTF8(r)
			processedJobs[i].Result = &resultStr
		} else {
			e := item.GetItemString("error")
			if e == nil {
				return fmt.Errorf("missing result and error from item at index %d", i)
			}
			eStr := python.PythonWrapper.PyUnicode_AsUTF8(e)
			processedJobs[i].Error = &Jinja2Error{eStr}
		}
	}

	return nil
}
