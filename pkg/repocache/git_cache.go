package repocache

import (
	"context"
	"fmt"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/kluctl/kluctl/v2/pkg/sourceoverride"
	"github.com/kluctl/kluctl/v2/pkg/types"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/kluctl/kluctl/v2/pkg/git"
	"github.com/kluctl/kluctl/v2/pkg/git/auth"
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

	repos      map[types.RepoKey]*GitCacheEntry
	reposMutex sync.Mutex

	repoOverrides sourceoverride.Resolver

	cleanupDirs      []string
	cleanupDirsMutex sync.Mutex
}

type GitCacheEntry struct {
	rp         *GitRepoCache
	url        types.GitUrl
	mr         *git.MirroredGitRepo
	defaultRef types.GitRef
	refs       map[string]string

	clonedDirs   map[types.GitRef]clonedDir
	updateMutex  sync.Mutex
	overridePath string
}

type RepoInfo struct {
	Url        types.GitUrl      `json:"url"`
	RemoteRefs map[string]string `json:"remoteRefs"`
	DefaultRef types.GitRef      `json:"defaultRef"`
}

type clonedDir struct {
	dir  string
	info git.CheckoutInfo
}

func NewGitRepoCache(ctx context.Context, sshPool *ssh_pool.SshPool, authProviders *auth.GitAuthProviders, repoOverrides sourceoverride.Resolver, updateInterval time.Duration) *GitRepoCache {
	return &GitRepoCache{
		ctx:            ctx,
		sshPool:        sshPool,
		authProviders:  authProviders,
		updateInterval: updateInterval,
		repos:          map[types.RepoKey]*GitCacheEntry{},
		repoOverrides:  repoOverrides,
	}
}

func (rp *GitRepoCache) Clear() {
	rp.cleanupDirsMutex.Lock()
	defer rp.cleanupDirsMutex.Unlock()

	for _, p := range rp.cleanupDirs {
		_ = os.RemoveAll(p)
	}
	rp.cleanupDirs = nil
}

func (rp *GitRepoCache) GetEntry(url string) (*GitCacheEntry, error) {
	rp.reposMutex.Lock()
	defer rp.reposMutex.Unlock()

	u, err := types.ParseGitUrl(url)
	if err != nil {
		return nil, err
	}

	repoKey := u.RepoKey()

	var overridePath string
	if rp.repoOverrides != nil {
		overridePath, err = rp.repoOverrides.ResolveOverride(rp.ctx, repoKey)
		if err != nil {
			return nil, err
		}
	}

	if overridePath != "" {
		status.WarningOncef(rp.ctx, fmt.Sprintf("git-override-%s", repoKey), "Overriding git repo %s with local directory %s", url, overridePath)

		e := &GitCacheEntry{
			rp:           rp,
			url:          *u,
			mr:           nil, // mark as overridden
			clonedDirs:   map[types.GitRef]clonedDir{},
			overridePath: overridePath,
		}
		rp.repos[repoKey] = e
		return e, nil
	}

	e, ok := rp.repos[repoKey]
	if !ok {
		mr, err := git.NewMirroredGitRepo(rp.ctx, *u, filepath.Join(utils.GetCacheDir(rp.ctx), "git-cache"), rp.sshPool, rp.authProviders)
		if err != nil {
			return nil, err
		}
		e = &GitCacheEntry{
			rp:         rp,
			url:        *u,
			mr:         mr,
			clonedDirs: map[types.GitRef]clonedDir{},
		}
		rp.repos[repoKey] = e
	}
	err = e.Update()
	if err != nil {
		return nil, err
	}
	return e, nil
}

func (e *GitCacheEntry) Update() error {
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
			s := status.Startf(e.rp.ctx, "Updating git cache for %s", url.String())
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

	defaultRefStr, err := e.mr.DefaultRef()
	if err != nil {
		return err
	}

	defaultRef, err := types.ParseGitRef(defaultRefStr)
	if err != nil {
		return err
	}
	e.defaultRef = defaultRef

	return nil
}

func (e *GitCacheEntry) GetRepoInfo() RepoInfo {
	e.updateMutex.Lock()
	defer e.updateMutex.Unlock()

	info := RepoInfo{
		Url:        e.url,
		RemoteRefs: e.refs,
		DefaultRef: e.defaultRef,
	}

	return info
}

func (e *GitCacheEntry) findCommit(ref string) (string, string, error) {
	ref, objectHash, err := e.findRef(ref)
	if err != nil {
		return "", "", err
	}

	o, err := e.mr.GetObjectByHash(objectHash)
	if err != nil {
		return "", "", err
	}

	if o.Type() == plumbing.CommitObject {
		return ref, objectHash, nil
	} else if o.Type() == plumbing.TagObject {
		o2 := o.(*object.Tag)
		return ref, o2.Target.String(), nil
	} else {
		return "", "", fmt.Errorf("unsupported object type %s", o.Type().String())
	}
}

func (e *GitCacheEntry) findRef(ref string) (string, string, error) {
	switch {
	case strings.HasPrefix(ref, "refs/heads"), strings.HasPrefix(ref, "refs/tags"):
		c, ok := e.refs[ref]
		if !ok {
			return "", "", fmt.Errorf("ref %s not found", ref)
		}
		return ref, c, nil
	default:
		// TODO remove this compatibility code
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

func (e *GitCacheEntry) GetClonedDir(ref *types.GitRef) (string, git.CheckoutInfo, error) {
	e.updateMutex.Lock()
	defer e.updateMutex.Unlock()

	tmpDir := filepath.Join(utils.GetTmpBaseDir(e.rp.ctx), "git-cloned")
	err := os.MkdirAll(tmpDir, 0700)
	if err != nil {
		return "", git.CheckoutInfo{}, err
	}

	url := e.url
	repoName := path.Base(url.Normalize().Path) + "-"
	if ref == nil {
		repoName += "HEAD-"
	} else {
		repoName += ref.String() + "-"
	}
	repoName = strings.ReplaceAll(repoName, "/", "-")

	p, err := os.MkdirTemp(tmpDir, repoName)
	if err != nil {
		return "", git.CheckoutInfo{}, err
	}

	e.rp.cleanupDirsMutex.Lock()
	e.rp.cleanupDirs = append(e.rp.cleanupDirs, p)
	e.rp.cleanupDirsMutex.Unlock()

	if e.mr == nil { // local override exist
		err = cp.Copy(e.overridePath, p)
		if err != nil {
			return "", git.CheckoutInfo{}, err
		}
		return p, git.CheckoutInfo{}, err
	}

	err = e.mr.Lock()
	if err != nil {
		return "", git.CheckoutInfo{}, err
	}
	defer e.mr.Unlock()

	if ref == nil {
		ref = &e.defaultRef
	}

	var commit string
	var checkoutInfo git.CheckoutInfo
	if ref.Commit != "" {
		commit = ref.Commit
		checkoutInfo.CheckedOutRef = *ref
		checkoutInfo.CheckedOutCommit = ref.Commit
	} else {
		var ref2 string
		ref2, commit, err = e.findCommit(ref.String())
		if err != nil {
			return "", git.CheckoutInfo{}, err
		}
		checkoutInfo.CheckedOutRef, err = types.ParseGitRef(ref2)
		if err != nil {
			return "", git.CheckoutInfo{}, err
		}
		checkoutInfo.CheckedOutCommit = commit
	}

	err = e.mr.CloneProjectByCommit(commit, p)
	if err != nil {
		return "", git.CheckoutInfo{}, err
	}

	e.clonedDirs[*ref] = clonedDir{
		dir:  p,
		info: checkoutInfo,
	}
	return p, checkoutInfo, nil
}
