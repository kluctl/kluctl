package git

import (
	"context"
	"fmt"
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

type MirroredGitRepoCollection struct {
	ctx            context.Context
	authProviders  *auth.GitAuthProviders
	updateInterval time.Duration
	repos          map[string]*entry
	mutex          sync.Mutex
}

type entry struct {
	mr          *MirroredGitRepo
	clonedDirs  map[string]string
	updateMutex sync.Mutex
}

func NewMirroredGitRepoCollection(ctx context.Context, authProviders *auth.GitAuthProviders, updateInterval time.Duration) *MirroredGitRepoCollection {
	return &MirroredGitRepoCollection{
		ctx:            ctx,
		authProviders:  authProviders,
		updateInterval: updateInterval,
		repos:          map[string]*entry{},
	}
}

func (g *MirroredGitRepoCollection) Clear() {
	g.mutex.Lock()
	defer g.mutex.Unlock()

	for _, e := range g.repos {
		for _, path := range e.clonedDirs {
			_ = os.RemoveAll(path)
		}

		if e.mr.IsLocked() {
			_ = e.mr.Unlock()
		}
	}

	g.repos = map[string]*entry{}
}

func (g *MirroredGitRepoCollection) getEntry(url git_url.GitUrl, allowCreate bool, lockRepo bool, update bool) (*entry, error) {
	e, err := func() (*entry, error) {
		g.mutex.Lock()
		defer g.mutex.Unlock()

		e, ok := g.repos[url.NormalizedRepoKey()]
		if !ok {
			if !allowCreate {
				return nil, fmt.Errorf("git repo %s not found", url.NormalizedRepoKey())
			}
			mr, err := NewMirroredGitRepo(g.ctx, url)
			if err != nil {
				return nil, err
			}
			e = &entry{
				mr:         mr,
				clonedDirs: map[string]string{},
			}
			g.repos[url.NormalizedRepoKey()] = e

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
		if time.Now().Sub(e.mr.LastUpdateTime()) <= g.updateInterval {
			e.mr.SetUpdated(true)
		} else {
			err = e.mr.Update(g.authProviders)
			if err != nil {
				return nil, err
			}
		}
	}

	return e, nil
}

func (g *MirroredGitRepoCollection) GetMirroredGitRepo(url git_url.GitUrl, allowCreate bool, lockRepo bool, update bool) (*MirroredGitRepo, error) {
	e, err := g.getEntry(url, allowCreate, lockRepo, update)
	if err != nil {
		return nil, err
	}
	return e.mr, nil
}

func (g *MirroredGitRepoCollection) GetClonedDir(url git_url.GitUrl, ref string, allowCreate bool, lockRepo bool, update bool) (string, error) {
	e, err := g.getEntry(url, allowCreate, lockRepo, update)
	if err != nil {
		return "", err
	}

	e.updateMutex.Lock()
	defer e.updateMutex.Unlock()

	p, ok := e.clonedDirs[ref]
	if ok {
		return p, nil
	}

	tmpDir := filepath.Join(utils.GetTmpBaseDir(), "git-cloned")
	err = os.MkdirAll(tmpDir, 0700)
	if err != nil {
		return "", err
	}

	repoName := path.Base(url.Normalize().Path)
	p, err = ioutil.TempDir(tmpDir, repoName+"-"+ref+"-")
	if err != nil {
		return "", err
	}

	err = e.mr.CloneProject(ref, p)
	if err != nil {
		return "", err
	}

	e.clonedDirs[ref] = p
	return p, nil
}
