package kluctl_jinja2

import (
	"fmt"
	"github.com/hashicorp/go-multierror"
	"github.com/kluctl/go-jinja2"
	"strings"
)

func RenderConditionals(j *jinja2.Jinja2, vars map[string]any, conditionals []string) ([]string, error) {
	ret := make([]string, len(conditionals))
	jobs := make([]*jinja2.RenderJob, 0, len(conditionals))

	for _, c := range conditionals {
		job := &jinja2.RenderJob{
			Template: buildConditionalTemplate(c),
		}
		jobs = append(jobs, job)
	}
	err := j.RenderStrings(jobs, jinja2.WithGlobals(vars))
	if err != nil {
		return nil, err
	}

	for i, job := range jobs {
		if job.Error != nil {
			e := fmt.Errorf("unable to render conditional '%s': %w", conditionals[i], job.Error)
			err = multierror.Append(err, e)
		} else {
			r := strings.TrimSpace(*job.Result)
			if r != "" && r != "True" && r != "False" {
				err = multierror.Append(err, fmt.Errorf("unable to evaluate conditional: %s", conditionals[i]))
			} else {
				ret[i] = r
			}
		}
	}
	return ret, err
}

func RenderConditional(j *jinja2.Jinja2, vars map[string]any, conditional string) (string, error) {
	rendered, err := RenderConditionals(j, vars, []string{conditional})
	if err != nil {
		return "", err
	}
	return rendered[0], nil
}

func buildConditionalTemplate(c string) string {
	if c == "" {
		return ""
	}
	return fmt.Sprintf("{%% if %s %%} True {%% else %%} False {%% endif %%}", c)
}

func IsConditionalTrue(c string) bool {
	return c == "" || c == "True"
}
