package kluctl_project

import (
	"fmt"
	git_url "github.com/kluctl/kluctl/v2/pkg/git/git-url"
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

			_, err := c.RP.GetEntry(u)
			if err != nil {
				doError(fmt.Errorf("failed to update git project %v: %v", u.String(), err))
			}
		}()

		return nil
	}

	for _, target := range c.Config.Targets {
		if target.TargetConfig == nil || target.TargetConfig.Project == nil {
			continue
		}

		err := doUpdateGitProject(target.TargetConfig.Project.Url)
		if err != nil {
			waitGroup.Wait()
			return err
		}
	}

	waitGroup.Wait()
	return firstError
}
