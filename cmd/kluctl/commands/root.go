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
	"github.com/kluctl/kluctl/v2/pkg/status"
	"github.com/kluctl/kluctl/v2/pkg/utils"
	"github.com/kluctl/kluctl/v2/pkg/utils/uo"
	"github.com/kluctl/kluctl/v2/pkg/version"
	"github.com/kluctl/kluctl/v2/pkg/yaml"
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

	CpuProfile string `group:"global" help:"Enable CPU profiling and write the result to the given path"`
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
	Seal        sealCmd        `cmd:"" help:"Seal secrets based on target's sealingConfig"`
	Validate    validateCmd    `cmd:"" help:"Validates the already deployed deployment"`
	Controller  controllerCmd  `cmd:"" help:"Kluctl controller sub-commands"`
	Webui       webuiCmd       `cmd:"" help:"TODO"`

	Version versionCmd `cmd:"" help:"Print kluctl version"`
}

var flagGroups = []groupInfo{
	{group: "global", title: "Global arguments:"},
	{group: "project", title: "Project arguments:", description: "Define where and how to load the kluctl project and its components from."},
	{group: "images", title: "Image arguments:", description: "Control fixed images and update behaviour."},
	{group: "inclusion", title: "Inclusion/Exclusion arguments:", description: "Control inclusion/exclusion."},
	{group: "misc", title: "Misc arguments:", description: "Command specific arguments."},
	{group: "results", title: "Command Results:", description: "Configure how command results are stored."},
}

var origStderr = os.Stderr

func initStatusHandler(ctx context.Context, debug bool, noColor bool) context.Context {
	// we must determine isTerminal before we override os.Stderr
	isTerminal := isatty.IsTerminal(origStderr.Fd())
	var sh status.StatusHandler
	if !debug && isatty.IsTerminal(origStderr.Fd()) {
		sh = status.NewMultiLineStatusHandler(ctx, origStderr, isTerminal, !noColor, false)
	} else {
		sh = status.NewSimpleStatusHandler(func(message string) {
			_, _ = fmt.Fprintf(origStderr, "%s\n", message)
		}, isTerminal, false)
	}
	sh.SetTrace(debug)
	ctx = status.NewContext(ctx, sh)
	return ctx
}

func redirectLogsAndStderr(ctxGetter func() context.Context) {
	f := func(line string) {
		status.Info(ctxGetter(), line)
	}

	lr1 := status.NewLineRedirector(f)
	lr2 := status.NewLineRedirector(f)

	klog.LogToStderr(false)
	klog.SetOutput(lr1)
	log.SetOutput(lr2)
	ctrl.SetLogger(klog.NewKlogr())

	pr, pw, err := os.Pipe()
	if err != nil {
		panic(err)
	}

	go func() {
		x := status.NewLineRedirector(f)
		_, _ = io.Copy(x, pr)
	}()

	os.Stderr = pw
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

	versionCheckPath := filepath.Join(utils.GetTmpBaseDir(ctx), "version_check.yaml")
	var versionCheckState VersionCheckState
	err := yaml.ReadYamlFile(versionCheckPath, &versionCheckState)
	if err == nil {
		if time.Now().Sub(versionCheckState.LastVersionCheck) < time.Hour {
			return
		}
	}

	versionCheckState.LastVersionCheck = time.Now()
	_ = yaml.WriteYamlFile(versionCheckPath, &versionCheckState)

	s := status.Start(ctx, "Checking for new kluctl version")
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
		s.FailedWithMessage("Failed to parse latest version: %v", err)
		return
	}
	localVersion, err := semver.NewVersion(version.GetVersion())
	if err != nil {
		s.FailedWithMessage("Failed to parse local version: %v", err)
		return
	}
	if localVersion.LessThan(latestVersion) {
		s.Update(fmt.Sprintf("You are using an outdated version (%v) of kluctl. You should update soon to version %v", localVersion.String(), latestVersion.String()))
	} else {
		s.Update("Your kluctl version is up-to-date")
	}
	s.Success()
}

func (c *cli) Run(ctx context.Context) error {
	return flag.ErrHelp
}

func initViper(ctx context.Context) {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath("/etc/kluctl/")
	viper.AddConfigPath("$HOME/.kluctl")
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			status.Error(ctx, err.Error())
			os.Exit(1)
		}
	}
}

func Main() {
	colorable.EnableColorsStdout(nil)
	ctx := context.Background()

	ctx = initStatusHandler(ctx, false, true)
	redirectLogsAndStderr(func() context.Context {
		// ctx might be replaced later in preRun() of Execute()
		return ctx
	})

	initViper(ctx)

	err := Execute(ctx, os.Args[1:], func(ctxIn context.Context, cmd *cobra.Command, flags *GlobalFlags) (context.Context, error) {
		err := copyViperValuesToCobraCmd(cmd)
		if err != nil {
			return ctx, err
		}
		err = setupProfiling(flags.CpuProfile)
		if err != nil {
			return ctx, err
		}
		oldSh := status.FromContext(ctxIn)
		if oldSh != nil {
			oldSh.Stop()
		}
		ctx = initStatusHandler(ctxIn, flags.Debug, flags.NoColor)
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

	sh := status.FromContext(ctx)
	sh.Stop()

	if err != nil {
		os.Exit(1)
	}
}

func Execute(ctx context.Context, args []string, preRun func(ctx context.Context, rootCmd *cobra.Command, flags *GlobalFlags) (context.Context, error)) error {
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
		if preRun != nil {
			ctx, err = preRun(ctx, cmd, &root.GlobalFlags)
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

	err = rootCmd.Execute()
	if err != nil {
		status.Error(ctx, "%s", err.Error())
		return err
	}
	return nil
}
