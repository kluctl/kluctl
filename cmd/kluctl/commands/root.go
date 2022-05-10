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
	"github.com/kluctl/kluctl/v2/pkg/status"
	"github.com/kluctl/kluctl/v2/pkg/utils"
	"github.com/kluctl/kluctl/v2/pkg/utils/uo"
	"github.com/kluctl/kluctl/v2/pkg/utils/versions"
	"github.com/kluctl/kluctl/v2/pkg/version"
	"github.com/kluctl/kluctl/v2/pkg/yaml"
	"github.com/mattn/go-isatty"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"k8s.io/klog/v2"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const latestReleaseUrl = "https://api.github.com/repos/kluctl/kluctl/releases/latest"

type cli struct {
	Debug         bool `group:"global" help:"Enable debug logging"`
	NoUpdateCheck bool `group:"global" help:"Disable update check on startup"`

	Archive           archiveCmd           `cmd:"" help:"Write project and all related components into single tgz"`
	CheckImageUpdates checkImageUpdatesCmd `cmd:"" help:"Render deployment and check if any images have new tags available"`
	Delete            deleteCmd            `cmd:"" help:"Delete a target (or parts of it) from the corresponding cluster"`
	Deploy            deployCmd            `cmd:"" help:"Deploys a target to the corresponding cluster"`
	Diff              diffCmd              `cmd:"" help:"Perform a diff between the locally rendered target and the already deployed target"`
	Downscale         downscaleCmd         `cmd:"" help:"Downscale all deployments"`
	HelmPull          helmPullCmd          `cmd:"" help:"Recursively searches for 'helm-chart.yml' files and pulls the specified Helm charts"`
	HelmUpdate        helmUpdateCmd        `cmd:"" help:"Recursively searches for 'helm-chart.yml'' files and checks for new available versions"`
	ListImages        listImagesCmd        `cmd:"" help:"Renders the target and outputs all images used via 'images.get_image(...)"`
	ListTargets       listTargetsCmd       `cmd:"" help:"Outputs a yaml list with all target, including dynamic targets"`
	PokeImages        pokeImagesCmd        `cmd:"" help:"Replace all images in target"`
	Prune             pruneCmd             `cmd:"" help:"Searches the target cluster for prunable objects and deletes them"`
	Render            renderCmd            `cmd:"" help:"Renders all resources and configuration files"`
	Seal              sealCmd              `cmd:"" help:"Seal secrets based on target's sealingConfig"`
	Validate          validateCmd          `cmd:"" help:"Validates the already deployed deployment"`

	Version versionCmd `cmd:"" help:"Print kluctl version"`
}

var flagGroups = []groupInfo{
	{group: "global", title: "Global arguments:"},
	{group: "project", title: "Project arguments:", description: "Define where and how to load the kluctl project and its components from."},
	{group: "images", title: "Image arguments:", description: "Control fixed images and update behaviour."},
	{group: "inclusion", title: "Inclusion/Exclusion arguments:", description: "Control inclusion/exclusion."},
	{group: "misc", title: "Misc arguments:", description: "Command specific arguments."},
}

var cliCtx = context.Background()

func (c *cli) setupStatusHandler() error {
	var sh status.StatusHandler
	if isatty.IsTerminal(os.Stderr.Fd()) {
		sh = status.NewMultiLineStatusHandler(cliCtx, os.Stderr, c.Debug)
	} else {
		sh = status.NewSimpleStatusHandler(func(message string) {
			_, _ = fmt.Fprintf(os.Stderr, "%s\n", message)
		}, c.Debug)
	}
	cliCtx = status.NewContext(cliCtx, sh)

	klog.LogToStderr(false)
	klog.SetOutput(status.NewLineRedirector(sh.Info))

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

	versionCheckPath := filepath.Join(utils.GetTmpBaseDir(), "version_check.yml")
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
	if err := c.setupStatusHandler(); err != nil {
		return err
	}
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

	rootCmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		err := copyViperValuesToCobraCmd(cmd)
		if err != nil {
			return err
		}
		return root.preRun()
	}

	initViper()

	err = rootCmd.Execute()

	sh := status.FromContext(cliCtx)

	if err != nil {
		status.Error(cliCtx, err.Error())
		sh.Stop()
		os.Exit(1)
	}
	sh.Stop()
}
