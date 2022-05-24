package repoprovider

import (
	"context"
	"fmt"
	"github.com/kluctl/kluctl/v2/pkg/git"
	"github.com/kluctl/kluctl/v2/pkg/git/auth"
	git_url "github.com/kluctl/kluctl/v2/pkg/git/git-url"
	"github.com/kluctl/kluctl/v2/pkg/utils"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"sync"
	"time"
)

type LiveRepoProvider struct {
	ctx            context.Context
	authProviders  *auth.GitAuthProviders
	updateInterval time.Duration
	repos          map[string]*entry
	mutex          sync.Mutex
}

type entry struct {
	mr          *git.MirroredGitRepo
	clonedDirs  map[string]clonedDir
	updateMutex sync.Mutex
}

type clonedDir struct {
	dir  string
	info git.CheckoutInfo
}

func NewLiveRepoProvider(ctx context.Context, authProviders *auth.GitAuthProviders, updateInterval time.Duration) RepoProvider {
	return &LiveRepoProvider{
		ctx:            ctx,
		authProviders:  authProviders,
		updateInterval: updateInterval,
		repos:          map[string]*entry{},
	}
}

func (rp *LiveRepoProvider) Clear() {
	rp.mutex.Lock()
	defer rp.mutex.Unlock()

	for _, e := range rp.repos {
		for _, cd := range e.clonedDirs {
			_ = os.RemoveAll(cd.dir)
		}

		if e.mr.IsLocked() {
			_ = e.mr.Unlock()
		}
	}

	rp.repos = map[string]*entry{}
}

func (rp *LiveRepoProvider) getEntry(url git_url.GitUrl, allowCreate bool, lockRepo bool, update bool) (*entry, error) {
	e, err := func() (*entry, error) {
		rp.mutex.Lock()
		defer rp.mutex.Unlock()

		e, ok := rp.repos[url.NormalizedRepoKey()]
		if !ok {
			if !allowCreate {
				return nil, fmt.Errorf("git repo %s not found", url.NormalizedRepoKey())
			}
			mr, err := git.NewMirroredGitRepo(rp.ctx, url)
			if err != nil {
				return nil, err
			}
			e = &entry{
				mr:         mr,
				clonedDirs: map[string]clonedDir{},
			}
			rp.repos[url.NormalizedRepoKey()] = e

			if lockRepo {
				err = e.mr.Lock()
				if err != nil {
					return nil, err
				}
			}
		}
		return e, nil
	}()
	if err != nil {
		return nil, err
	}

	e.updateMutex.Lock()
	defer e.updateMutex.Unlock()

	if update && !e.mr.HasUpdated() {
		if time.Now().Sub(e.mr.LastUpdateTime()) <= rp.updateInterval {
			e.mr.SetUpdated(true)
		} else {
			err = e.mr.Update(rp.authProviders)
			if err != nil {
				return nil, err
			}
		}
	}

	return e, nil
}

func (rp *LiveRepoProvider) GetRepoInfo(url git_url.GitUrl) (RepoInfo, error) {
	e, err := rp.getEntry(url, true, true, true)
	if err != nil {
		return RepoInfo{}, err
	}

	remoteRefs, err := e.mr.RemoteRefHashesMap()
	if err != nil {
		return RepoInfo{}, err
	}

	defaultRef, err := e.mr.DefaultRef()
	if err != nil {
		return RepoInfo{}, err
	}

	info := RepoInfo{
		Url:        url,
		RemoteRefs: remoteRefs,
		DefaultRef: defaultRef,
	}

	return info, nil
}

func (rp *LiveRepoProvider) GetClonedDir(url git_url.GitUrl, ref string) (string, git.CheckoutInfo, error) {
	e, err := rp.getEntry(url, true, true, true)
	if err != nil {
		return "", git.CheckoutInfo{}, err
	}

	e.updateMutex.Lock()
	defer e.updateMutex.Unlock()

	cd, ok := e.clonedDirs[ref]
	if ok {
		return cd.dir, cd.info, nil
	}

	tmpDir := filepath.Join(utils.GetTmpBaseDir(), "git-cloned")
	err = os.MkdirAll(tmpDir, 0700)
	if err != nil {
		return "", git.CheckoutInfo{}, err
	}

	repoName := path.Base(url.Normalize().Path) + "-"
	if ref == "" {
		repoName += "HEAD-"
	} else {
		repoName += ref + "-"
	}
	p, err := ioutil.TempDir(tmpDir, repoName)
	if err != nil {
		return "", git.CheckoutInfo{}, err
	}

	err = e.mr.CloneProject(ref, p)
	if err != nil {
		return "", git.CheckoutInfo{}, err
	}

	repoInfo, err := git.GetCheckoutInfo(p)
	if err != nil {
		return "", git.CheckoutInfo{}, err
	}

	e.clonedDirs[ref] = clonedDir{
		dir:  p,
		info: repoInfo,
	}
	return p, repoInfo, nil
}
