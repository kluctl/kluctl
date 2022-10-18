package args

import "time"

type ProjectFlags struct {
	ProjectConfig existingFileType `group:"project" short:"c" help:"Location of the .kluctl.yaml config file. Defaults to $PROJECT/.kluctl.yaml" exts:"yml,yaml"`

	Timeout                time.Duration `group:"project" help:"Specify timeout for all operations, including loading of the project, all external api calls and waiting for readiness." default:"10m"`
	GitCacheUpdateInterval time.Duration `group:"project" help:"Specify the time to wait between git cache updates. Defaults to not wait at all and always updating caches."`
}

type ArgsFlags struct {
	Arg          []string `group:"project" short:"a" help:"Passes a template argument in the form of name=value. Nested args can be set with the '-a my.nested.arg=value' syntax. Values are interpreted as yaml values, meaning that 'true' and 'false' will lead to boolean values and numbers will be treated as numbers. Use quotes if you want these to be treated as strings. If the value starts with @, it is treated as a file, meaning that the contents of the file will be loaded and treated as yaml."`
	ArgsFromFile []string `group:"project" help:"Loads a yaml file and makes it available as arguments, meaning that they will be available thought the global 'args' variable."`
}

type TargetFlags struct {
	Target string `group:"project" short:"t" help:"Target name to run command for. Target must exist in .kluctl.yaml."`
}
