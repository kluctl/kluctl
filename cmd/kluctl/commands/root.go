/*
Copyright © 2022 Alexander Block <ablock84@gmail.com>

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package commands

import (
	"context"
	"fmt"
	go_container_logs "github.com/google/go-containerregistry/pkg/logs"
	"github.com/google/gops/agent"
	status2 "github.com/kluctl/kluctl/lib/status"
	"github.com/kluctl/kluctl/lib/yaml"
	"github.com/kluctl/kluctl/v2/pkg/prompts"
	flag "github.com/spf13/pflag"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"runtime/pprof"
	ctrl "sigs.k8s.io/controller-runtime"
	"strings"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/kluctl/kluctl/v2/pkg/utils"
	"github.com/kluctl/kluctl/v2/pkg/utils/uo"
	"github.com/kluctl/kluctl/v2/pkg/version"
	"github.com/mattn/go-colorable"
	"github.com/mattn/go-isatty"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"k8s.io/klog/v2"
)

const latestReleaseUrl = "https://api.github.com/repos/kluctl/kluctl/releases/latest"

type GlobalFlags struct {
	Debug         bool `group:"global" help:"Enable debug logging"`
	NoUpdateCheck bool `group:"global" help:"Disable update check on startup"`
	NoColor       bool `group:"global" help:"Disable colored output"`

	CpuProfile    string `group:"global" help:"Enable CPU profiling and write the result to the given path"`
	GopsAgent     bool   `group:"global" help:"Start gops agent in the background"`
	GopsAgentAddr string `group:"global" help:"Specify the address:port to use for the gops agent" default:"127.0.0.1:0"`

	UseSystemPython bool `group:"global" help:"Use the system Python instead of the embedded Python."`
}

type cli struct {
	GlobalFlags

	Delete      deleteCmd      `cmd:"" help:"Delete a target (or parts of it) from the corresponding cluster"`
	Deploy      deployCmd      `cmd:"" help:"Deploys a target to the corresponding cluster"`
	Diff        diffCmd        `cmd:"" help:"Perform a diff between the locally rendered target and the already deployed target"`
	HelmPull    helmPullCmd    `cmd:"" help:"Recursively searches for 'helm-chart.yaml' files and pre-pulls the specified Helm charts"`
	HelmUpdate  helmUpdateCmd  `cmd:"" help:"Recursively searches for 'helm-chart.yaml' files and checks for new available versions"`
	ListImages  listImagesCmd  `cmd:"" help:"Renders the target and outputs all images used via 'images.get_image(...)"`
	ListTargets listTargetsCmd `cmd:"" help:"Outputs a yaml list with all targets"`
	PokeImages  pokeImagesCmd  `cmd:"" help:"Replace all images in target"`
	Prune       pruneCmd       `cmd:"" help:"Searches the target cluster for prunable objects and deletes them"`
	Render      renderCmd      `cmd:"" help:"Renders all resources and configuration files"`
	Validate    validateCmd    `cmd:"" help:"Validates the already deployed deployment"`
	Controller  controllerCmd  `cmd:"" help:"Kluctl controller sub-commands"`
	Gitops      gitopsCmd      `cmd:"" help:"GitOps sub-commands"`
	Webui       webuiCmd       `cmd:"" help:"Kluctl Webui sub-commands"`
	Oci         ociCmd         `cmd:"" help:"Oci sub-commands"`

	Version versionCmd `cmd:"" help:"Print kluctl version"`
}

var flagGroups = []groupInfo{
	{group: "global", title: "Global arguments:"},
	{group: "project", title: "Project arguments:", description: "Define where and how to load the kluctl project and its components from."},
	{group: "images", title: "Image arguments:", description: "Control fixed images and update behaviour."},
	{group: "inclusion", title: "Inclusion/Exclusion arguments:", description: "Control inclusion/exclusion."},
	{group: "gitops", title: "GitOps arguments:", description: "Specify gitops flags."},
	{group: "misc", title: "Misc arguments:", description: "Command specific arguments."},
	{group: "results", title: "Command Results:", description: "Configure how command results are stored."},
	{group: "logs", title: "Log arguments:", description: "Configure logging."},
	{group: "override", title: "GitOps overrides:", description: "Override settings for GitOps deployments."},
	{group: "helm", title: "Helm arguments:", description: "Configure Helm authentication."},
	{group: "registry", title: "Registry arguments:", description: "Configure OCI registry authentication."},
	{group: "auth", title: "Auth arguments:", description: "Configure authentication."},
}

var origStderr = os.Stderr

// we must determine isTerminal before we override os.Stderr
var isTerminal = isatty.IsTerminal(os.Stderr.Fd())

func initStatusHandlerAndPrompts(ctx context.Context, debug bool, noColor bool) context.Context {
	var sh status2.StatusHandler
	var pp prompts.PromptProvider
	if !debug && isTerminal {
		sh = status2.NewMultiLineStatusHandler(ctx, origStderr, isTerminal && !noColor, false)
		pp = &prompts.StatusAndStdinPromptProvider{}
	} else {
		sh = status2.NewSimpleStatusHandler(func(level status2.Level, message string) {
			_, _ = fmt.Fprintf(origStderr, "%s\n", message)
		}, debug)
		pp = &prompts.SimplePromptProvider{Out: origStderr}
	}
	ctx = status2.NewContext(ctx, sh)
	ctx = prompts.NewContext(ctx, pp)

	return ctx
}

func redirectLogsAndStderr(ctx context.Context) {
	f := func(line string) {
		status2.Info(ctx, line)
	}

	lr1 := status2.NewLineRedirector(f)
	lr2 := status2.NewLineRedirector(f)

	klog.LogToStderr(false)
	klog.SetOutput(lr1)
	log.SetOutput(lr2)
	ctrl.SetLogger(klog.NewKlogr())

	go_container_logs.Warn.SetOutput(status2.NewLineRedirector(func(line string) {
		status2.Warning(ctx, line)
	}))

	pr, pw, err := os.Pipe()
	if err != nil {
		panic(err)
	}

	go func() {
		x := status2.NewLineRedirector(f)
		_, _ = io.Copy(x, pr)
	}()

	os.Stderr = pw
}

func setupGops(flags *GlobalFlags) error {
	if !flags.GopsAgent {
		return nil
	}

	if err := agent.Listen(agent.Options{
		Addr: flags.GopsAgentAddr,
	}); err != nil {
		return err
	}
	return nil
}

var cpuProfileFile *os.File

func setupProfiling(cpuProfile string) error {
	var err error
	if cpuProfile != "" {
		cpuProfileFile, err = os.Create(cpuProfile)
		if err != nil {
			return fmt.Errorf("failed to create cpu profile file: %w", err)
		}
		err = pprof.StartCPUProfile(cpuProfileFile)
		if err != nil {
			return fmt.Errorf("failed to start cpu profiling: %w", err)
		}
	}
	return nil
}

type VersionCheckState struct {
	LastVersionCheck time.Time `json:"lastVersionCheck"`
}

func checkNewVersion(ctx context.Context) {
	if version.GetVersion() == "0.0.0" {
		return
	}

	versionCheckPath := filepath.Join(utils.GetCacheDir(ctx), "version_check.yaml")
	var versionCheckState VersionCheckState
	err := yaml.ReadYamlFile(versionCheckPath, &versionCheckState)
	if err == nil {
		if time.Now().Sub(versionCheckState.LastVersionCheck) < time.Hour {
			return
		}
	}

	versionCheckState.LastVersionCheck = time.Now()
	_ = yaml.WriteYamlFile(versionCheckPath, &versionCheckState)

	s := status2.Start(ctx, "Checking for new kluctl version")
	defer s.Failed()

	r, err := http.Get(latestReleaseUrl)
	if err != nil {
		return
	}
	defer r.Body.Close()

	var release uo.UnstructuredObject
	err = yaml.ReadYamlStream(r.Body, &release)
	if err != nil {
		return
	}

	latestVersionStr, ok, _ := release.GetNestedString("tag_name")
	if !ok {
		return
	}
	if strings.HasPrefix(latestVersionStr, "v") {
		latestVersionStr = latestVersionStr[1:]
	}
	latestVersion, err := semver.NewVersion(latestVersionStr)
	if err != nil {
		s.FailedWithMessagef("Failed to parse latest version: %v", err)
		return
	}
	localVersion, err := semver.NewVersion(version.GetVersion())
	if err != nil {
		s.FailedWithMessagef("Failed to parse local version: %v", err)
		return
	}
	if localVersion.LessThan(latestVersion) {
		s.Updatef("You are using an outdated version (%v) of kluctl. You should update soon to version %v", localVersion.String(), latestVersion.String())
	} else {
		s.Update("Your kluctl version is up-to-date")
	}
	s.Success()
}

func (c *cli) Run(ctx context.Context) error {
	return flag.ErrHelp
}

func initViper() {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath("/etc/kluctl/")
	viper.AddConfigPath("$HOME/.kluctl")
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			_, _ = fmt.Fprintf(os.Stderr, "%s\n", err.Error())
			os.Exit(1)
		}
	}
}

func Main() {
	colorable.EnableColorsStdout(nil)
	ctx := context.Background()

	didSetupStatusHandler := false

	err := Execute(ctx, os.Args[1:], func(ctxIn context.Context) (context.Context, error) {
		cmd := getCobraCommand(ctxIn)
		flags := getCobraGlobalFlags(ctxIn)

		err := setupGops(flags)
		if err != nil {
			return ctx, err
		}
		err = setupProfiling(flags.CpuProfile)
		if err != nil {
			return ctx, err
		}

		ctx = initStatusHandlerAndPrompts(ctxIn, flags.Debug, flags.NoColor)
		didSetupStatusHandler = true

		if cmd.Parent() == nil || (cmd.Name() != "run" && cmd.Parent().Name() != "controller") {
			redirectLogsAndStderr(ctx)
		}

		if !flags.NoUpdateCheck {
			if len(os.Args) < 2 || (os.Args[1] != "completion" && os.Args[1] != "__complete") {
				checkNewVersion(ctx)
			}
		}
		return ctx, nil
	})

	if cpuProfileFile != nil {
		pprof.StopCPUProfile()
		_ = cpuProfileFile.Close()
		cpuProfileFile = nil
	}

	if err != nil {
		if didSetupStatusHandler {
			status2.Error(ctx, err.Error())
		} else {
			_, _ = fmt.Fprintf(os.Stderr, "%s\n", err.Error())
		}
	}

	sh := status2.FromContext(ctx)
	if sh != nil {
		sh.Stop()
	}

	if err != nil {
		os.Exit(1)
	}
}

type cobraCmdContextKey struct{}
type cobraGlobalFlagsKey struct{}

func getCobraCommand(ctx context.Context) *cobra.Command {
	v := ctx.Value(cobraCmdContextKey{})
	if x, ok := v.(*cobra.Command); ok {
		return x
	}
	return nil
}

func getCobraGlobalFlags(ctx context.Context) *GlobalFlags {
	v := ctx.Value(cobraGlobalFlagsKey{})
	if x, ok := v.(*GlobalFlags); ok {
		return x
	}
	panic("missing global flags")
}

func Execute(ctx context.Context, args []string, preRun func(ctx context.Context) (context.Context, error)) error {
	root := cli{}
	rootCmd, err := buildRootCobraCmd(&root, "kluctl",
		"Deploy and manage complex deployments on Kubernetes",
		`The missing glue to put together large Kubernetes deployments,
composed of multiple smaller parts (Helm/Kustomize/...) in a manageable and unified way.`,
		flagGroups)

	if err != nil {
		return err
	}

	rootCmd.SetContext(ctx)
	rootCmd.SetArgs(args)
	rootCmd.Version = version.GetVersion()
	rootCmd.SilenceUsage = true
	rootCmd.SilenceErrors = true

	rootCmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		ctx = context.WithValue(ctx, cobraCmdContextKey{}, cmd)
		for c := cmd; c != nil; c = c.Parent() {
			c.SetContext(ctx)
		}

		err = copyViperValuesToCobraCmd(cmd)
		if err != nil {
			return err
		}

		ctx = context.WithValue(ctx, cobraGlobalFlagsKey{}, &root.GlobalFlags)
		for c := cmd; c != nil; c = c.Parent() {
			c.SetContext(ctx)
		}

		if preRun != nil {
			ctx, err = preRun(ctx)
			if ctx != nil {
				for c := cmd; c != nil; c = c.Parent() {
					c.SetContext(ctx)
				}
			}
			if err != nil {
				return err
			}
		}
		return nil
	}

	return rootCmd.Execute()
}

func init() {
	initViper()
}
