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
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"runtime/pprof"
	"strings"
	"time"

	"github.com/kluctl/kluctl/v2/pkg/status"
	"github.com/kluctl/kluctl/v2/pkg/utils"
	"github.com/kluctl/kluctl/v2/pkg/utils/uo"
	"github.com/kluctl/kluctl/v2/pkg/utils/versions"
	"github.com/kluctl/kluctl/v2/pkg/version"
	"github.com/kluctl/kluctl/v2/pkg/yaml"
	"github.com/mattn/go-colorable"
	"github.com/mattn/go-isatty"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"k8s.io/klog/v2"
)

const latestReleaseUrl = "https://api.github.com/repos/kluctl/kluctl/releases/latest"

type cli struct {
	Debug         bool `group:"global" help:"Enable debug logging"`
	NoUpdateCheck bool `group:"global" help:"Disable update check on startup"`
	NoColor       bool `group:"global" help:"Disable colored output"`

	CpuProfile string `group:"global" help:"Enable CPU profiling and write the result to the given path"`

	CheckImageUpdates checkImageUpdatesCmd `cmd:"" help:"Render deployment and check if any images have new tags available"`
	Delete            deleteCmd            `cmd:"" help:"Delete a target (or parts of it) from the corresponding cluster"`
	Deploy            deployCmd            `cmd:"" help:"Deploys a target to the corresponding cluster"`
	Diff              diffCmd              `cmd:"" help:"Perform a diff between the locally rendered target and the already deployed target"`
	HelmPull          helmPullCmd          `cmd:"" help:"Recursively searches for 'helm-chart.yaml' files and pulls the specified Helm charts"`
	HelmUpdate        helmUpdateCmd        `cmd:"" help:"Recursively searches for 'helm-chart.yaml' files and checks for new available versions"`
	ListImages        listImagesCmd        `cmd:"" help:"Renders the target and outputs all images used via 'images.get_image(...)"`
	ListTargets       listTargetsCmd       `cmd:"" help:"Outputs a yaml list with all target, including dynamic targets"`
	PokeImages        pokeImagesCmd        `cmd:"" help:"Replace all images in target"`
	Prune             pruneCmd             `cmd:"" help:"Searches the target cluster for prunable objects and deletes them"`
	Render            renderCmd            `cmd:"" help:"Renders all resources and configuration files"`
	Seal              sealCmd              `cmd:"" help:"Seal secrets based on target's sealingConfig"`
	Validate          validateCmd          `cmd:"" help:"Validates the already deployed deployment"`
	Flux              fluxCmd              `cmd:"" help:"Flux sub-commands"`

	Version versionCmd `cmd:"" help:"Print kluctl version"`
}

var flagGroups = []groupInfo{
	{group: "global", title: "Global arguments:"},
	{group: "project", title: "Project arguments:", description: "Define where and how to load the kluctl project and its components from."},
	{group: "images", title: "Image arguments:", description: "Control fixed images and update behaviour."},
	{group: "inclusion", title: "Inclusion/Exclusion arguments:", description: "Control inclusion/exclusion."},
	{group: "misc", title: "Misc arguments:", description: "Command specific arguments."},
	{group: "flux", title: "Flux arguments:", description: "EXPERIMENTAL: Subcommands for interaction with flux-kluctl-controller"},
}

var cliCtx = context.Background()
var didSetupStatusHandler bool

func setupStatusHandler(debug bool, noColor bool) {
	didSetupStatusHandler = true

	origStderr := os.Stderr

	// we must determine isTerminal before we override os.Stderr
	isTerminal := isatty.IsTerminal(os.Stderr.Fd())
	var sh status.StatusHandler
	if !debug && isatty.IsTerminal(os.Stderr.Fd()) {
		sh = status.NewMultiLineStatusHandler(cliCtx, os.Stderr, isTerminal, !noColor, false)
	} else {
		sh = status.NewSimpleStatusHandler(func(message string) {
			_, _ = fmt.Fprintf(origStderr, "%s\n", message)
		}, isTerminal, false)
	}
	sh.SetTrace(debug)
	cliCtx = status.NewContext(cliCtx, sh)

	klog.LogToStderr(false)
	klog.SetOutput(status.NewLineRedirector(sh.Info))
	log.SetOutput(status.NewLineRedirector(sh.Info))

	pr, pw, err := os.Pipe()
	if err != nil {
		panic(err)
	}

	go func() {
		x := status.NewLineRedirector(sh.Info)
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
	LastVersionCheck time.Time `yaml:"lastVersionCheck"`
}

func (c *cli) checkNewVersion() {
	if c.NoUpdateCheck {
		return
	}
	if version.GetVersion() == "0.0.0" {
		return
	}

	versionCheckPath := filepath.Join(utils.GetTmpBaseDir(), "version_check.yaml")
	var versionCheckState VersionCheckState
	err := yaml.ReadYamlFile(versionCheckPath, &versionCheckState)
	if err == nil {
		if time.Now().Sub(versionCheckState.LastVersionCheck) < time.Hour {
			return
		}
	}

	versionCheckState.LastVersionCheck = time.Now()
	_ = yaml.WriteYamlFile(versionCheckPath, &versionCheckState)

	s := status.Start(cliCtx, "Checking for new kluctl version")
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
	latestVersion := versions.LooseVersion(latestVersionStr)
	localVersion := versions.LooseVersion(version.GetVersion())
	if localVersion.Less(latestVersion, true) {
		s.Update(fmt.Sprintf("You are using an outdated version (%v) of kluctl. You should update soon to version %v", localVersion, latestVersion))
	} else {
		s.Update("Your kluctl version is up-to-date")
	}
	s.Success()
}

func (c *cli) preRun() error {
	err := setupProfiling(c.CpuProfile)
	if err != nil {
		return err
	}
	setupStatusHandler(c.Debug, c.NoColor)
	c.checkNewVersion()
	return nil
}

func (c *cli) Run() error {
	return nil
}

func initViper() {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath("/etc/kluctl/")
	viper.AddConfigPath("$HOME/.kluctl")
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			status.Error(cliCtx, err.Error())
			os.Exit(1)
		}
	}

	viper.SetEnvPrefix("kluctl")
	viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))
	viper.AutomaticEnv()
}

func Execute() {
	colorable.EnableColorsStdout(nil)

	root := cli{}
	rootCmd, err := buildRootCobraCmd(&root, "kluctl",
		"Deploy and manage complex deployments on Kubernetes",
		`The missing glue to put together large Kubernetes deployments,
composed of multiple smaller parts (Helm/Kustomize/...) in a manageable and unified way.`,
		flagGroups)

	if err != nil {
		panic(err)
	}

	rootCmd.Version = version.GetVersion()
	rootCmd.SilenceUsage = true
	rootCmd.SilenceErrors = true

	rootCmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		err := copyViperValuesToCobraCmd(cmd)
		if err != nil {
			return err
		}
		return root.preRun()
	}

	initViper()

	err = rootCmd.ExecuteContext(cliCtx)
	if !didSetupStatusHandler {
		setupStatusHandler(false, true)
	}

	if cpuProfileFile != nil {
		pprof.StopCPUProfile()
		_ = cpuProfileFile.Close()
		cpuProfileFile = nil
	}

	sh := status.FromContext(cliCtx)

	if err != nil {
		status.Error(cliCtx, "%s", err.Error())
		sh.Stop()
		os.Exit(1)
	}
	sh.Stop()
}
