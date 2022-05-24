package kluctl_project

import (
	"fmt"
	git_url "github.com/kluctl/kluctl/v2/pkg/git/git-url"
	types2 "github.com/kluctl/kluctl/v2/pkg/types"
	"sync"
)

func (c *LoadedKluctlProject) updateGitCaches() error {
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

	doUpdateGitProject := func(u git_url.GitUrl) error {
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()

			_, err := c.GRC.GetMirroredGitRepo(u, true, true, true)
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
