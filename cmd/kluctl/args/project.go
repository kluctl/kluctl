package args

import (
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

	ProjectConfig ExistingFileType `group:"project" short:"c" help:"Location of the .kluctl.yaml config file. Defaults to $PROJECT/.kluctl.yaml" exts:"yml,yaml"`

	Timeout                time.Duration `group:"project" help:"Specify timeout for all operations, including loading of the project, all external api calls and waiting for readiness." default:"10m"`
	GitCacheUpdateInterval time.Duration `group:"project" help:"Specify the time to wait between git cache updates. Defaults to not wait at all and always updating caches."`
	LocalGitOverride       []string      `group:"project" help:"Specify a single repository local git override in the form of 'github.com:my-org/my-repo=/local/path/to/override'. This will cause kluctl to not use git to clone for the specified repository but instead use the local directory. This is useful in case you need to test out changes in external git repositories without pushing them."`
	LocalGitGroupOverride  []string      `group:"project" help:"Same as --local-git-override, but for a whole group prefix instead of a single repository. All repositories that have the given prefix will be overridden with the given local path and the repository suffix appended. For example, 'gitlab.com:some-org/sub-org=/local/path/to/my-forks' will override all repositories below 'gitlab.com:some-org/sub-org/' with the repositories found in '/local/path/to/my-forks'. It will however only perform an override if the given repository actually exists locally and otherwise revert to the actual (non-overridden) repository."`
}

type ArgsFlags struct {
	Arg          []string `group:"project" short:"a" help:"Passes a template argument in the form of name=value. Nested args can be set with the '-a my.nested.arg=value' syntax. Values are interpreted as yaml values, meaning that 'true' and 'false' will lead to boolean values and numbers will be treated as numbers. Use quotes if you want these to be treated as strings. If the value starts with @, it is treated as a file, meaning that the contents of the file will be loaded and treated as yaml."`
	ArgsFromFile []string `group:"project" help:"Loads a yaml file and makes it available as arguments, meaning that they will be available thought the global 'args' variable."`
}

type TargetFlags struct {
	Target             string `group:"project" short:"t" help:"Target name to run command for. Target must exist in .kluctl.yaml."`
	TargetNameOverride string `group:"project" short:"T" help:"Overrides the target name. If -t is used at the same time, then the target will be looked up based on -t <name> and then renamed to the value of -T. If no target is specified via -t, then the no-name target is renamed to the value of -T."`
	Context            string `group:"project" help:"Overrides the context name specified in the target. If the selected target does not specify a context or the no-name target is used, --context will override the currently active context."`
}

type CommandResultFlags struct {
	WriteCommandResult      bool   `group:"results" help:"Enable writing of command results into the cluster. This is enabled by default." default:"true"`
	ForceWriteCommandResult bool   `group:"results" help:"Force writing of command results, even if the command is run in dry-run mode."`
	CommandResultNamespace  string `group:"results" help:"Override the namespace to be used when writing command results." default:"kluctl-results"`
	KeepCommandResultsCount int    `group:"results" help:"Configure how many old command results to keep." default:"10"`
}
