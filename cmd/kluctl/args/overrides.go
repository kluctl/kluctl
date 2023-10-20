package args

import (
	"context"
	"fmt"
	"github.com/kluctl/kluctl/v2/pkg/repocache"
	"github.com/kluctl/kluctl/v2/pkg/status"
	"github.com/kluctl/kluctl/v2/pkg/types"
	"strings"
)

type SourceOverrides struct {
	LocalGitOverride      []string `group:"project" help:"Specify a single repository local git override in the form of 'github.com/my-org/my-repo=/local/path/to/override'. This will cause kluctl to not use git to clone for the specified repository but instead use the local directory. This is useful in case you need to test out changes in external git repositories without pushing them."`
	LocalGitGroupOverride []string `group:"project" help:"Same as --local-git-override, but for a whole group prefix instead of a single repository. All repositories that have the given prefix will be overridden with the given local path and the repository suffix appended. For example, 'gitlab.com/some-org/sub-org=/local/path/to/my-forks' will override all repositories below 'gitlab.com/some-org/sub-org/' with the repositories found in '/local/path/to/my-forks'. It will however only perform an override if the given repository actually exists locally and otherwise revert to the actual (non-overridden) repository."`
	LocalOciOverride      []string `group:"project" help:"Same as --local-git-override, but for OCI repositories."`
	LocalOciGroupOverride []string `group:"project" help:"Same as --local-git-group-override, but for OCI repositories."`
}

func (a *SourceOverrides) ParseOverrides(ctx context.Context) ([]repocache.RepoOverride, []repocache.RepoOverride, error) {
	var gitRepoOverrides []repocache.RepoOverride
	var ociRepoOverrides []repocache.RepoOverride
	for _, x := range a.LocalGitOverride {
		ro, err := a.parseRepoOverride(ctx, x, false, "git", true)
		if err != nil {
			return nil, nil, fmt.Errorf("invalid --local-git-override: %w", err)
		}
		gitRepoOverrides = append(gitRepoOverrides, ro)
	}
	for _, x := range a.LocalGitGroupOverride {
		ro, err := a.parseRepoOverride(ctx, x, true, "git", true)
		if err != nil {
			return nil, nil, fmt.Errorf("invalid --local-git-group-override: %w", err)
		}
		gitRepoOverrides = append(gitRepoOverrides, ro)
	}
	for _, x := range a.LocalOciOverride {
		ro, err := a.parseRepoOverride(ctx, x, false, "oci", false)
		if err != nil {
			return nil, nil, fmt.Errorf("invalid --local-oci-override: %w", err)
		}
		ociRepoOverrides = append(ociRepoOverrides, ro)
	}
	for _, x := range a.LocalOciGroupOverride {
		ro, err := a.parseRepoOverride(ctx, x, true, "oci", false)
		if err != nil {
			return nil, nil, fmt.Errorf("invalid --local-oci-group-override: %w", err)
		}
		ociRepoOverrides = append(ociRepoOverrides, ro)
	}
	return gitRepoOverrides, ociRepoOverrides, nil
}

func (a *SourceOverrides) parseRepoOverride(ctx context.Context, s string, isGroup bool, type_ string, allowLegacy bool) (repocache.RepoOverride, error) {
	sp := strings.SplitN(s, "=", 2)
	if len(sp) != 2 {
		return repocache.RepoOverride{}, fmt.Errorf("%s", s)
	}

	repoKey, err := types.ParseRepoKey(sp[0], type_)
	if err != nil {
		if !allowLegacy {
			return repocache.RepoOverride{}, err
		}

		// try as legacy repo key
		u, err2 := types.ParseGitUrl(sp[0])
		if err2 != nil {
			// return original error
			return repocache.RepoOverride{}, err
		}

		x := u.Host
		if !strings.HasPrefix(u.Path, "/") {
			x += "/"
		}
		x += u.Path
		repoKey, err2 = types.ParseRepoKey(x, type_)
		if err2 != nil {
			// return original error
			return repocache.RepoOverride{}, err
		}

		status.Deprecation(ctx, "old-repo-override", "Passing --local-git-override/--local-git-override-group in the example.com:path form is deprecated and will not be supported in future versions of Kluctl. Please use the example.com/path form.")
	}

	return repocache.RepoOverride{
		RepoKey:  repoKey,
		IsGroup:  isGroup,
		Override: sp[1],
	}, nil
}
