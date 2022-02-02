package kluctl_project

import (
	"fmt"
	"github.com/codablock/kluctl/pkg/git"
	git_url "github.com/codablock/kluctl/pkg/git/git-url"
	types2 "github.com/codablock/kluctl/pkg/types"
	"github.com/codablock/kluctl/pkg/utils"
	"os"
	"path"
	"reflect"
	"strings"
	"sync"
)

func (c *KluctlProjectContext) updateGitCaches() error {
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
		return repo.WithLock(func() error {
			if !repo.HasUpdated() {
				return repo.Update()
			}
			return nil
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
		if p == nil {
			return nil
		}
		return doUpdateGitProject(p.Project.Url)
	}

	err := doUpdateExternalProject(c.config.Deployment)
	if err != nil {
		waitGroup.Wait()
		return err
	}
	err = doUpdateExternalProject(c.config.SealedSecrets)
	if err != nil {
		waitGroup.Wait()
		return err
	}
	for _, ep := range c.config.Clusters.Projects {
		err = doUpdateExternalProject(&ep)
		if err != nil {
			waitGroup.Wait()
			return err
		}
	}

	for _, target := range c.config.Targets {
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

type gitProjectInfo struct {
	url    git_url.GitUrl
	ref    string
	commit string
	dir    string
}

func (c *KluctlProjectContext) buildCloneDir(u git_url.GitUrl, ref string) (string, error) {
	if ref == "" {
		ref = "HEAD"
	}
	ref = strings.ReplaceAll(ref, "/", "-")
	ref = strings.ReplaceAll(ref, "\\", "-")
	baseName := path.Base(u.Path)
	urlHash := utils.Sha256String(fmt.Sprintf("%s:%s", u.Host, u.Path))
	return path.Join(c.TmpDir, fmt.Sprintf("%s-%s", baseName, urlHash), ref), nil
}

func (c *KluctlProjectContext) cloneGitProject(gitProject types2.ExternalProject, defaultGitSubDir string, doAddInvolvedRepo bool, doLock bool) (result gitProjectInfo, err error) {
	err = os.MkdirAll(path.Join(c.TmpDir, "git"), 0o777)
	if err != nil {
		return
	}

	targetDir, err := c.buildCloneDir(gitProject.Project.Url, gitProject.Project.Ref)
	if err != nil {
		return
	}

	subDir := gitProject.Project.SubDir
	if subDir == "" {
		subDir = defaultGitSubDir
	}
	dir := targetDir
	if subDir != "" {
		dir = path.Join(dir, subDir)
	}

	mr, ok := c.mirroredRepos[gitProject.Project.Url.NormalizedRepoKey()]
	if !ok {
		mr, err = git.NewMirroredGitRepo(gitProject.Project.Url)
		if err != nil {
			return
		}
		c.mirroredRepos[gitProject.Project.Url.NormalizedRepoKey()] = mr
		err = mr.Lock()
		if err != nil {
			return
		}
		defer mr.Unlock()
	}

	err = mr.MaybeWithLock(doLock, func() error {
		if !mr.HasUpdated() {
			err = mr.Update()
			if err != nil {
				return err
			}
		}
		return mr.CloneProject(gitProject.Project.Ref, targetDir)
	})
	if err != nil {
		return
	}

	ri, err := git.GetGitRepoInfo(targetDir)
	if err != nil {
		return
	}

	result.url = gitProject.Project.Url
	result.ref = ri.CheckedOutRef
	result.commit = ri.CheckedOutCommit
	result.dir = dir

	if doAddInvolvedRepo {
		c.addInvolvedRepo(result.url, result.ref, map[string]string{
			result.ref: result.commit,
		})
	}

	return
}

func (c *KluctlProjectContext) localProject(dir string) gitProjectInfo {
	return gitProjectInfo{
		dir: dir,
	}
}

func (c *KluctlProjectContext) cloneKluctlProject() (gitProjectInfo, error) {
	if c.loadArgs.ProjectUrl == nil {
		p, err := os.Getwd()
		if err != nil {
			return gitProjectInfo{}, err
		}
		return c.localProject(p), err
	}
	return c.cloneGitProject(types2.ExternalProject{
		Project: types2.GitProject{
			Url: *c.loadArgs.ProjectUrl,
			Ref: c.loadArgs.ProjectRef,
		},
	}, "", true, true)
}

func (c *KluctlProjectContext) addInvolvedRepo(u git_url.GitUrl, refPattern string, refs map[string]string) {
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
	}
}
