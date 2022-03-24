package deployment

import (
	"fmt"
	"github.com/kluctl/kluctl/pkg/jinja2"
	"github.com/kluctl/kluctl/pkg/types"
	"github.com/kluctl/kluctl/pkg/utils/uo"
	"github.com/kluctl/kluctl/pkg/yaml"
	"path/filepath"
	"regexp"
	"strings"
)

var argPattern = regexp.MustCompile("^[a-zA-Z0-9_./-]*=.*$")

func ParseArgs(argsList []string) (map[string]string, error) {
	args := make(map[string]string)
	for _, arg := range argsList {
		if !argPattern.MatchString(arg) {
			return nil, fmt.Errorf("invalid --arg argument. Must be --arg=some_var_name=value")
		}

		s := strings.SplitN(arg, "=", 2)
		name := s[0]
		value := s[1]
		args[name] = value
	}
	return args, nil
}

func ConvertArgsToVars(args map[string]string) *uo.UnstructuredObject {
	vars := uo.New()
	for n, v := range args {
		var p []interface{}
		for _, x := range strings.Split(n, ".") {
			p = append(p, x)
		}
		_ = vars.SetNestedField(v, p...)
	}
	return vars
}

func CheckRequiredDeployArgs(dir string, varsCtx *jinja2.VarsCtx, deployArgs *uo.UnstructuredObject) error {
	// First try to load the config without templating to avoid getting errors while rendering because required
	// args were not set. Otherwise we won't be able to iterator through the 'args' array in the deployment.yml
	// when the rendering error is actually args related.

	var conf types.DeploymentProjectConfig

	err := yaml.ReadYamlFile(yaml.FixPathExt(filepath.Join(dir, "deployment.yml")), &conf)
	if err != nil {
		// If that failed, it might be that conditional jinja blocks are present in the config, so lets try loading
		// the config in rendered form. If it fails due to missing args now, we can't help much with better error
		// messages anymore.
		varsCtx2 := varsCtx.Copy()
		varsCtx2.UpdateChild("args", deployArgs)
		err = varsCtx2.RenderYamlFile(yaml.FixNameExt(dir, "deployment.yml"), []string{dir}, &conf)
		if err != nil {
			return err
		}
	}

	if len(conf.Args) == 0 {
		return nil
	}

	err = checkRequiredArgs(conf.Args, deployArgs)
	if err != nil {
		return err
	}
	return nil
}

func checkRequiredArgs(argsDef []*types.DeploymentArg, args *uo.UnstructuredObject) error {
	for _, a := range argsDef {
		var p []interface{}
		for _, x := range strings.Split(a.Name, ".") {
			p = append(p, x)
		}
		_, found, _ := args.GetNestedField(p...)
		if !found {
			if a.Default == nil {
				return fmt.Errorf("required argument %s not set", a.Name)
			} else {
				err := args.SetNestedField(a.Default, p...)
				if err != nil {
					return err
				}
			}
		}
	}

	return nil
}
