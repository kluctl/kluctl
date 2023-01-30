package repocache

import (
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/kluctl/kluctl/v2/pkg/git"
	"github.com/kluctl/kluctl/v2/pkg/git/auth"
	git_url "github.com/kluctl/kluctl/v2/pkg/git/git-url"
	ssh_pool "github.com/kluctl/kluctl/v2/pkg/git/ssh-pool"
	"github.com/kluctl/kluctl/v2/pkg/status"
	"github.com/kluctl/kluctl/v2/pkg/utils"
	cp "github.com/otiai10/copy"
)

type GitRepoCache struct {
	ctx            context.Context
	authProviders  *auth.GitAuthProviders
	sshPool        *ssh_pool.SshPool
	updateInterval time.Duration

	repos      map[string]*CacheEntry
	reposMutex sync.Mutex

	repoOverrides []RepoOverride

	cleanupDirs       []string
	cleeanupDirsMutex sync.Mutex
}

type CacheEntry struct {
	rp         *GitRepoCache
	mr         *git.MirroredGitRepo
	defaultRef string
	refs       map[string]string

	clonedDirs   map[string]clonedDir
	updateMutex  sync.Mutex
	overridePath string
}

type RepoInfo struct {
	Url        git_url.GitUrl    `yaml:"url"`
	RemoteRefs map[string]string `yaml:"remoteRefs"`
	DefaultRef string            `yaml:"defaultRef"`
}

type RepoOverride struct {
	RepoUrl  git_url.GitUrl
	Ref      string
	Override string
	IsGroup  bool
}

type clonedDir struct {
	dir  string
	info git.CheckoutInfo
}

func NewGitRepoCache(ctx context.Context, sshPool *ssh_pool.SshPool, authProviders *auth.GitAuthProviders, repoOverrides []RepoOverride, updateInterval time.Duration) *GitRepoCache {
	return &GitRepoCache{
		ctx:            ctx,
		sshPool:        sshPool,
		authProviders:  authProviders,
		updateInterval: updateInterval,
		repos:          map[string]*CacheEntry{},
		repoOverrides:  repoOverrides,
	}
}

func (rp *GitRepoCache) Clear() {
	rp.cleeanupDirsMutex.Lock()
	defer rp.cleeanupDirsMutex.Unlock()

	for _, p := range rp.cleanupDirs {
		_ = os.RemoveAll(p)
	}
	rp.cleanupDirs = nil
}

func (rp *GitRepoCache) GetEntry(url git_url.GitUrl) (*CacheEntry, error) {
	rp.reposMutex.Lock()
	defer rp.reposMutex.Unlock()

	urlN := url.NormalizedRepoKey()

	// evaluate overrides
	for _, ro := range rp.repoOverrides {
		uO, err := git_url.Parse(ro.RepoKey)
		if err != nil {
			return nil, err
		}
		uN := uO.NormalizedRepoKey()

		// FIXME, what was the requested logic here?
		if uO.Hostname() != url.Normalize().Hostname() {
			continue
		}
		if uO.Path() != url.Normalize().Path() {
			continue
		}

		// FIXME, temp. to test raw logic
		if ro.IsGroup && strings.HasPrefix(urlN, uN) {
			u, err := git_url.Parse(strings.Replace(urlN, uN, ro.Override, 1))
			if err != nil {
				return nil, err
			}
			url.URL = u.URL

			if _, ok := rp.repos[urlN]; !ok {
				e := &CacheEntry{
					rp:           rp,
					mr:           nil,
					clonedDirs:   map[string]clonedDir{},
					overridePath: ro.Override,
				}
				rp.repos[urlN] = e
				return e, nil
			}
			return rp.repos[urlN], nil
		}

	}

	e, ok := rp.repos[urlN]
	if !ok {
		mr, err := git.NewMirroredGitRepo(rp.ctx, url, filepath.Join(utils.GetTmpBaseDir(rp.ctx), "git-cache"), rp.sshPool, rp.authProviders)
		if err != nil {
			return nil, err
		}
		e = &CacheEntry{
			rp:         rp,
			mr:         mr,
			clonedDirs: map[string]clonedDir{},
		}
		rp.repos[urlN] = e
	}
	err := e.Update()
	if err != nil {
		return nil, err
	}
	return e, nil
}

func (e *CacheEntry) Update() error {
	e.updateMutex.Lock()
	defer e.updateMutex.Unlock()

	if e.mr == nil {
		return nil
	}

	err := e.mr.Lock()
	if err != nil {
		return err
	}
	defer e.mr.Unlock()

	if !e.mr.HasUpdated() {
		if time.Now().Sub(e.mr.LastUpdateTime()) <= e.rp.updateInterval {
			e.mr.SetUpdated(true)
		} else {
			url := e.mr.Url()
			s := status.Start(e.rp.ctx, "Updating git cache for %s", url.String())
			defer s.Failed()
			err := e.mr.Update()
			if err != nil {
				s.FailedWithMessage(err.Error())
				return err
			}
			s.Success()
		}
	}

	e.refs, err = e.mr.RemoteRefHashesMap()
	if err != nil {
		return err
	}

	e.defaultRef, err = e.mr.DefaultRef()
	if err != nil {
		return err
	}

	return nil
}

func (e *CacheEntry) GetRepoInfo() RepoInfo {
	e.updateMutex.Lock()
	defer e.updateMutex.Unlock()

	info := RepoInfo{
		Url:        e.mr.Url(),
		RemoteRefs: e.refs,
		DefaultRef: e.defaultRef,
	}

	return info
}

func (e *CacheEntry) findCommit(ref string) (string, string, error) {
	switch {
	case strings.HasPrefix(ref, "refs/heads"), strings.HasPrefix(ref, "refs/tags"):
		c, ok := e.refs[ref]
		if !ok {
			return "", "", fmt.Errorf("ref %s not found", ref)
		}
		return ref, c, nil
	default:
		ref2 := "refs/heads/" + ref
		c, ok := e.refs[ref2]
		if ok {
			return ref2, c, nil
		}
		ref2 = "refs/tags/" + ref
		c, ok = e.refs[ref2]
		if ok {
			return ref2, c, nil
		}
		return "", "", fmt.Errorf("ref %s not found", ref)
	}
}

func (e *CacheEntry) GetClonedDir(ref string) (string, git.CheckoutInfo, error) {
	e.updateMutex.Lock()
	defer e.updateMutex.Unlock()

	tmpDir := filepath.Join(utils.GetTmpBaseDir(e.rp.ctx), "git-cloned")
	err := os.MkdirAll(tmpDir, 0700)
	if err != nil {
		return "", git.CheckoutInfo{}, err
	}

	url := e.mr.Url()
	repoName := path.Base(url.Normalize().Path) + "-"
	if ref == "" {
		repoName += "HEAD-"
	} else {
		repoName += ref + "-"
	}
	repoName = strings.ReplaceAll(repoName, "/", "-")

	p, err := os.MkdirTemp(tmpDir, repoName)
	if err != nil {
		return "", git.CheckoutInfo{}, err
	}

	e.rp.cleeanupDirsMutex.Lock()
	e.rp.cleanupDirs = append(e.rp.cleanupDirs, p)
	e.rp.cleeanupDirsMutex.Unlock()

	//  FIXME it went bit wrong here, it needs to split

	if e.mr == nil { // local override exist
		var foundRo *RepoOverride
		// for _, ro := range e.rp.repoOverrides {
		// 	u := e.mr.Url()
		// 	if ro.RepoKey == u.NormalizedRepoKey() {
		// 		if ro.Ref == "" || strings.HasSuffix(ref2, "/"+ro.Ref) {
		// 			foundRo = &ro
		// 			break
		// 		}
		// 	}
		// }
		// DRAFT HERE, possibly we shall store original url on CacheEntry as well? - so we cant easily print what was overriden.
		for _, ro := range e.rp.repoOverrides {
			if ro.Override == e.overridePath {
				foundRo = &ro
				break
			}
		}

		u := foundRo.RepoUrl

		status.WarningOnce(e.rp.ctx, fmt.Sprintf("git-override-%s|%s", foundRo.RepoUrl.NormalizedRepoKey(), foundRo.Ref), "Overriding git repo %s with local directory %s", u, foundRo.Override)

		//FIXME, may be someone dont want to use temp dir here?
		err := cp.Copy(foundRo.Override, p)
		if err != nil {
			return "", git.CheckoutInfo{}, err
		}
	}

	err = e.mr.Lock()
	if err != nil {
		return "", git.CheckoutInfo{}, err
	}
	defer e.mr.Unlock()

	if ref == "" {
		ref = e.defaultRef
	}

	// err = e.Update()
	// if err != nil {
	// 	return "", git.CheckoutInfo{}, err
	// }

	ref2, commit, err := e.findCommit(ref)
	if err != nil {
		return "", git.CheckoutInfo{}, err
	}

	err = e.mr.CloneProjectByCommit(commit, p)
	if err != nil {
		return "", git.CheckoutInfo{}, err
	}

	repoInfo, err := git.GetCheckoutInfo(p)
	if err != nil {
		return "", git.CheckoutInfo{}, err
	}

	repoInfo.CheckedOutRef = ref2

	e.clonedDirs[ref] = clonedDir{
		dir:  p,
		info: repoInfo,
	}
	return p, repoInfo, nil
}
