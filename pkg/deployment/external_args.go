package deployment

import (
	"fmt"
	"github.com/codablock/kluctl/pkg/types"
	"github.com/codablock/kluctl/pkg/utils"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"path"
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

		s := strings.SplitN(arg, "=", 1)
		name := s[0]
		value := s[1]
		args[name] = value
	}
	return args, nil
}

func ConvertArgsToVars(args map[string]string) map[string]interface{} {
	m := make(map[string]interface{})
	for n, v := range args {
		p := strings.Split(n, ".")
		_ = unstructured.SetNestedField(m, v, p...)
	}
	return m
}

func CheckRequiredDeployArgs(dir string, varsCtx *VarsCtx, deployArgs map[string]interface{}) error {
	// First try to load the config without templating to avoid getting errors while rendering because required
	// args were not set. Otherwise we won't be able to iterator through the 'args' array in the deployment.yml
	// when the rendering error is actually args related.

	var conf types.DeploymentProjectConfig

	err := utils.ReadYamlFile(path.Join(dir, "deployment.yml"), &conf)
	if err != nil {
		// If that failed, it might be that conditional jinja blocks are present in the config, so lets try loading
		// the config in rendered form. If it fails due to missing args now, we can't help much with better error
		// messages anymore.
		varsCtx2 := varsCtx.Copy()
		varsCtx2.UpdateChild("args", deployArgs)
		err = varsCtx2.renderYamlFile("deployment.yml", []string{dir}, &conf)
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

func checkRequiredArgs(argsDef []*types.DeploymentArg, args map[string]interface{}) error {
	for _, a := range argsDef {
		p := strings.Split(a.Name, ".")
		_, found, _ := unstructured.NestedFieldNoCopy(args, p...)
		if !found {
			if a.Default == nil {
				return fmt.Errorf("required argument %s not set", a.Name)
			} else {
				err := unstructured.SetNestedField(args, a.Default, p...)
				if err != nil {
					return err
				}
			}
		}
	}

	return nil
}
