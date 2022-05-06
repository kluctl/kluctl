package kluctl_project

import (
	"context"
	"fmt"
	"github.com/kluctl/kluctl/v2/pkg/git"
	git_url "github.com/kluctl/kluctl/v2/pkg/git/git-url"
	types2 "github.com/kluctl/kluctl/v2/pkg/types"
	"github.com/kluctl/kluctl/v2/pkg/utils"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"sync"
	"time"
)

func (c *LoadedKluctlProject) updateGitCache(ctx context.Context, mr *git.MirroredGitRepo) error {
	if mr.HasUpdated() {
		return nil
	}
	if time.Now().Sub(mr.LastUpdateTime()) <= c.loadArgs.GitUpdateInterval {
		mr.SetUpdated(true)
		return nil
	}
	return mr.Update(ctx, c.loadArgs.GitAuthProviders)
}

func (c *LoadedKluctlProject) updateGitCaches(ctx context.Context) error {
	var waitGroup sync.WaitGroup
	var firstError error
	var firstErrorLock sync.Mutex

	doError := func(err error) {
		firstErrorLock.Lock()
		defer firstErrorLock.Unlock()
		if firstError == nil {
			firstError = err
		}
	}

	doUpdateRepo := func(repo *git.MirroredGitRepo) error {
		return repo.WithLock(ctx, func() error {
			return c.updateGitCache(ctx, repo)
		})
	}
	doUpdateGitProject := func(u git_url.GitUrl) error {
		mr, ok := c.mirroredRepos[u.NormalizedRepoKey()]
		if ok {
			return nil
		}
		mr, err := git.NewMirroredGitRepo(u)
		if err != nil {
			return err
		}
		c.mirroredRepos[u.NormalizedRepoKey()] = mr

		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			err := doUpdateRepo(mr)
			if err != nil {
				doError(fmt.Errorf("failed to update git project %v: %v", u.String(), err))
			}
		}()

		return nil
	}
	doUpdateExternalProject := func(p *types2.ExternalProject) error {
		if p == nil || p.Project == nil {
			return nil
		}
		return doUpdateGitProject(p.Project.Url)
	}

	err := doUpdateExternalProject(c.Config.Deployment)
	if err != nil {
		waitGroup.Wait()
		return err
	}
	err = doUpdateExternalProject(c.Config.SealedSecrets)
	if err != nil {
		waitGroup.Wait()
		return err
	}
	for _, ep := range c.Config.Clusters.Projects {
		err = doUpdateExternalProject(&ep)
		if err != nil {
			waitGroup.Wait()
			return err
		}
	}

	for _, target := range c.Config.Targets {
		if target.TargetConfig == nil || target.TargetConfig.Project == nil {
			continue
		}

		err = doUpdateGitProject(target.TargetConfig.Project.Url)
		if err != nil {
			waitGroup.Wait()
			return err
		}
	}

	waitGroup.Wait()
	return firstError
}

func (c *LoadedKluctlProject) cloneGitProject(ctx context.Context, gitProject *types2.GitProject, targetDir string, doLock bool) (info git.GitRepoInfo, err error) {
	err = os.MkdirAll(filepath.Join(c.TmpDir, "git"), 0o700)
	if err != nil {
		return
	}

	mr, ok := c.mirroredRepos[gitProject.Url.NormalizedRepoKey()]
	if !ok {
		mr, err = git.NewMirroredGitRepo(gitProject.Url)
		if err != nil {
			return
		}
		c.mirroredRepos[gitProject.Url.NormalizedRepoKey()] = mr
		err = mr.Lock(ctx)
		if err != nil {
			return
		}
		defer mr.Unlock()
	}

	err = mr.MaybeWithLock(ctx, doLock, func() error {
		err := c.updateGitCache(ctx, mr)
		if err != nil {
			return err
		}
		return mr.CloneProject(ctx, gitProject.Ref, targetDir)
	})
	if err != nil {
		return
	}

	info, err = git.GetGitRepoInfo(targetDir)
	return
}

func (c *LoadedKluctlProject) buildCloneDir(u git_url.GitUrl, ref string) (string, error) {
	baseDir := c.TmpDir
	if c.archiveDir != "" {
		baseDir = c.archiveDir
	}
	baseDir = filepath.Join(baseDir, "git")

	if ref == "" {
		ref = "HEAD"
	}
	ref = strings.ReplaceAll(ref, "/", "-")
	ref = strings.ReplaceAll(ref, "\\", "-")
	urlPath := filepath.FromSlash(u.Path)
	baseName := filepath.Base(urlPath)
	urlHash := utils.Sha256String(fmt.Sprintf("%s:%s", u.Host, u.Path))[:16]
	cloneDir := filepath.Join(fmt.Sprintf("%s-%s", baseName, urlHash), ref)
	cloneDir = filepath.Join(baseDir, cloneDir)
	err := utils.CheckInDir(baseDir, cloneDir)
	if err != nil {
		return "", err
	}
	return cloneDir, nil
}

func (c *LoadedKluctlProject) addInvolvedRepo(u git_url.GitUrl, refPattern string, refs map[string]string) {
	repoKey := u.NormalizedRepoKey()
	irs, _ := c.involvedRepos[repoKey]
	e := &types2.InvolvedRepo{
		RefPattern: refPattern,
		Refs:       refs,
	}
	found := false
	for _, ir := range irs {
		if reflect.DeepEqual(ir, e) {
			found = true
			break
		}
	}
	if !found {
		c.involvedRepos[repoKey] = append(c.involvedRepos[repoKey], *e)
		s := c.involvedRepos[repoKey]
		sort.SliceStable(s, func(i, j int) bool {
			return s[i].RefPattern < s[j].RefPattern
		})
	}
}
