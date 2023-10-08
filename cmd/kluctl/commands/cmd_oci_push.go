package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/kluctl/kluctl/v2/cmd/kluctl/args"
	"github.com/kluctl/kluctl/v2/pkg/git"
	"github.com/kluctl/kluctl/v2/pkg/status"
	"github.com/kluctl/kluctl/v2/pkg/yaml"
	"os"
	"strings"
	"time"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/google/go-containerregistry/pkg/name"
	reg "github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/remote/transport"

	"github.com/kluctl/kluctl/v2/pkg/oci/auth/login"
	"github.com/kluctl/kluctl/v2/pkg/oci/client"
	"github.com/kluctl/kluctl/v2/pkg/oci/sourceignore"
)

var excludeOCI = append(strings.Split(sourceignore.ExcludeVCS, ","), strings.Split(sourceignore.ExcludeExt, ",")...)

type ociPushCmd struct {
	args.ProjectDir

	Url        string   `group:"misc" help:"Specifies the artifact URL. This argument is required." required:"true"`
	Creds      string   `group:"misc" help:"credentials for OCI registry in the format <username>[:<password>] if --provider is generic"`
	Provider   string   `group:"misc" help:"the OCI provider name, available options are: (generic, aws, azure, gcp)" default:"generic"`
	IgnorePath []string `group:"misc" help:"set paths to ignore in .gitignore format."`
	Annotation []string `group:"misc" help:"Set custom OCI annotations in the format '<key>=<value>'"`
	Output     string   `group:"misc" help:"the format in which the artifact digest should be printed, can be 'json' or 'yaml'"`

	Timeout time.Duration `group:"misc" help:"Specify timeout for all operations, including loading of the project, all external api calls and waiting for readiness." default:"10m"`
}

func (cmd *ociPushCmd) Help() string {
	return `The push command creates a tarball from the current project and uploads the
artifact to an OCI repository. The command can read the credentials from '~/.docker/config.json' but they can also be
passed with --creds. It can also login to a supported provider with the --provider flag.`
}

func (cmd *ociPushCmd) Run(ctx context.Context) error {
	if cmd.IgnorePath == nil {
		cmd.IgnorePath = excludeOCI
	}

	if cmd.Provider != "generic" &&
		cmd.Provider != "azure" &&
		cmd.Provider != "aws" &&
		cmd.Provider != "gcp" {
		return fmt.Errorf("unknown provider %s", cmd.Provider)
	}

	url, err := client.ParseArtifactURL(cmd.Url)
	if err != nil {
		return err
	}

	ref, err := name.ParseReference(url)
	if err != nil {
		return err
	}

	path, err := cmd.ProjectDir.GetProjectDir()
	if err != nil {
		return err
	}

	repoRoot, err := git.DetectGitRepositoryRoot(path)
	if err != nil {
		return err
	}
	gitInfo, _, err := git.BuildGitInfo(ctx, repoRoot, path)
	if err != nil {
		return err
	}

	if _, err := os.Stat(path); err != nil {
		return fmt.Errorf("invalid path '%s', must point to an existing project: %w", path, err)
	}

	annotations := map[string]string{}
	for _, annotation := range cmd.Annotation {
		kv := strings.Split(annotation, "=")
		if len(kv) != 2 {
			return fmt.Errorf("invalid annotation %s, must be in the format key=value", annotation)
		}
		annotations[kv[0]] = kv[1]
	}

	annotations["io.kluctl.image.git_info"], err = yaml.WriteJsonString(&gitInfo)
	if err != nil {
		return err
	}

	meta := client.Metadata{
		Source:      gitInfo.Url.String(),
		Revision:    gitInfo.Commit,
		Annotations: annotations,
	}

	ctx, cancel := context.WithTimeout(ctx, cmd.Timeout)
	defer cancel()

	var auth authn.Authenticator
	opts := client.DefaultOptions()
	if cmd.Provider == "generic" && cmd.Creds != "" {
		status.Info(ctx, "Logging in to registry with credentials")
		auth, err = client.GetAuthFromCredentials(cmd.Creds)
		if err != nil {
			return fmt.Errorf("could not login with credentials: %w", err)
		}
		opts = append(opts, crane.WithAuth(auth))
	}

	if cmd.Provider != "generic" {
		status.Info(ctx, "Logging in to registry with provider credentials")

		auth, err = login.NewManager().Login(ctx, url, ref, getProviderLoginOption(cmd.Provider))
		if err != nil {
			return fmt.Errorf("error during login with provider: %w", err)
		}
		opts = append(opts, crane.WithAuth(auth))
	}

	if cmd.Timeout != 0 {
		backoff := remote.Backoff{
			Duration: 1.0 * time.Second,
			Factor:   3,
			Jitter:   0.1,
			// timeout happens when the cap is exceeded or number of steps is reached
			// 10 steps is big enough that most reasonable cap(under 30min) will be exceeded before
			// the number of steps are completed.
			Steps: 10,
			Cap:   cmd.Timeout,
		}

		if auth == nil {
			auth, err = authn.DefaultKeychain.Resolve(ref.Context())
			if err != nil {
				return err
			}
		}
		transportOpts, err := client.WithRetryTransport(ctx, ref, auth, backoff, []string{ref.Context().Scope(transport.PushScope)})
		if err != nil {
			return fmt.Errorf("error setting up transport: %w", err)
		}
		opts = append(opts, transportOpts, client.WithRetryBackOff(backoff))
	}

	var st *status.StatusContext
	if cmd.Output == "" {
		st = status.Startf(ctx, "Pushing artifact to %s", url)
		defer st.Failed()
	}

	ociClient := client.NewClient(opts)
	digestURL, err := ociClient.Push(ctx, url, path, meta, cmd.IgnorePath)
	if err != nil {
		return fmt.Errorf("pushing artifact failed: %w", err)
	}

	digest, err := reg.NewDigest(digestURL)
	if err != nil {
		return fmt.Errorf("artifact digest parsing failed: %w", err)
	}

	tag, err := reg.NewTag(url)
	if err != nil {
		return fmt.Errorf("artifact tag parsing failed: %w", err)
	}

	info := struct {
		URL        string `json:"url"`
		Repository string `json:"repository"`
		Tag        string `json:"tag"`
		Digest     string `json:"digest"`
	}{
		URL:        fmt.Sprintf("oci://%s", digestURL),
		Repository: digest.Repository.Name(),
		Tag:        tag.TagStr(),
		Digest:     digest.DigestStr(),
	}

	if cmd.Output == "" {
		st.UpdateAndInfoFallbackf("Artifact successfully pushed to %s", digestURL)
	}

	st.Success()
	status.Flush(ctx)

	switch cmd.Output {
	case "json":
		marshalled, err := json.MarshalIndent(&info, "", "  ")
		if err != nil {
			return fmt.Errorf("artifact digest JSON conversion failed: %w", err)
		}
		marshalled = append(marshalled, "\n"...)
		_, _ = getStdout(ctx).Write(marshalled)
	case "yaml":
		marshalled, err := yaml.WriteYamlBytes(&info)
		if err != nil {
			return fmt.Errorf("artifact digest YAML conversion failed: %w", err)
		}
		_, _ = getStdout(ctx).Write(marshalled)
	}

	return nil
}

func getProviderLoginOption(provider string) login.ProviderOptions {
	var opts login.ProviderOptions
	switch provider {
	case "azure":
		opts.AzureAutoLogin = true
	case "aws":
		opts.AwsAutoLogin = true
	case "gcp":
		opts.GcpAutoLogin = true
	}
	return opts
}
