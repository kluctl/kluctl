package git

import (
	"context"
	"fmt"
	"github.com/kluctl/kluctl/v2/pkg/git/auth"
	git_url "github.com/kluctl/kluctl/v2/pkg/git/git-url"
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

func (g *MirroredGitRepoCollection) UnlockAll() {
	g.mutex.Lock()
	defer g.mutex.Unlock()

	for _, e := range g.repos {
		_ = e.mr.Unlock()
	}
}

func (g *MirroredGitRepoCollection) GetMirroredGitRepo(url git_url.GitUrl, allowCreate bool, lockRepo bool, update bool) (*MirroredGitRepo, error) {
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
				mr: mr,
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

	return e.mr, nil
}
