package args

import (
	"github.com/kluctl/kluctl/v2/pkg/deployment"
	"github.com/kluctl/kluctl/v2/pkg/utils/uo"
	"os"
	"path/filepath"
	"time"
)

type ProjectDir struct {
	ProjectDir ExistingDirType `group:"project" help:"Specify the project directory. Defaults to the current working directory."`
}

func (a ProjectDir) GetProjectDir() (string, error) {
	if a.ProjectDir != "" {
		return filepath.Abs(a.ProjectDir.String())
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	return cwd, nil
}

type ProjectFlags struct {
	ProjectDir
	SourceOverrides

	ProjectConfig ExistingFileType `group:"project" short:"c" help:"Location of the .kluctl.yaml config file. Defaults to $PROJECT/.kluctl.yaml" exts:"yml,yaml"`

	Timeout                time.Duration `group:"project" help:"Specify timeout for all operations, including loading of the project, all external api calls and waiting for readiness." default:"10m"`
	GitCacheUpdateInterval time.Duration `group:"project" help:"Specify the time to wait between git cache updates. Defaults to not wait at all and always updating caches."`
}

type ArgsFlags struct {
	Arg          []string `group:"project" short:"a" help:"Passes a template argument in the form of name=value. Nested args can be set with the '-a my.nested.arg=value' syntax. Values are interpreted as yaml values, meaning that 'true' and 'false' will lead to boolean values and numbers will be treated as numbers. Use quotes if you want these to be treated as strings. If the value starts with @, it is treated as a file, meaning that the contents of the file will be loaded and treated as yaml."`
	ArgsFromFile []string `group:"project" help:"Loads a yaml file and makes it available as arguments, meaning that they will be available thought the global 'args' variable."`
}

func (a *ArgsFlags) LoadArgs() (*uo.UnstructuredObject, error) {
	if a == nil {
		return uo.New(), nil
	}

	var args *uo.UnstructuredObject
	optionArgs, err := deployment.ParseArgs(a.Arg)
	if err != nil {
		return nil, err
	}
	args, err = deployment.ConvertArgsToVars(optionArgs, true)
	if err != nil {
		return nil, err
	}
	for _, a := range a.ArgsFromFile {
		optionArgs2, err := uo.FromFile(a)
		if err != nil {
			return nil, err
		}
		args.Merge(optionArgs2)
	}
	return args, nil
}

type TargetFlagsBase struct {
	Target             string `group:"project" short:"t" help:"Target name to run command for. Target must exist in .kluctl.yaml."`
	TargetNameOverride string `group:"project" short:"T" help:"Overrides the target name. If -t is used at the same time, then the target will be looked up based on -t <name> and then renamed to the value of -T. If no target is specified via -t, then the no-name target is renamed to the value of -T."`
}

type TargetFlags struct {
	TargetFlagsBase
	Context string `group:"project" help:"Overrides the context name specified in the target. If the selected target does not specify a context or the no-name target is used, --context will override the currently active context."`
}

type CommandResultReadOnlyFlags struct {
	CommandResultNamespace string `group:"results" help:"Override the namespace to be used when writing command results." default:"kluctl-results"`
}

type CommandResultWriteFlags struct {
	WriteCommandResult       bool `group:"results" help:"Enable writing of command results into the cluster. This is enabled by default." default:"true"`
	ForceWriteCommandResult  bool `group:"results" help:"Force writing of command results, even if the command is run in dry-run mode."`
	KeepCommandResultsCount  int  `group:"results" help:"Configure how many old command results to keep." default:"5"`
	KeepValidateResultsCount int  `group:"results" help:"Configure how many old validate results to keep." default:"2"`
}

type CommandResultFlags struct {
	CommandResultReadOnlyFlags
	CommandResultWriteFlags
}
