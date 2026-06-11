package repocache

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/google/go-containerregistry/pkg/v1/cache"
	"github.com/kluctl/kluctl/lib/git"
	gittypes "github.com/kluctl/kluctl/lib/git/types"
	"github.com/kluctl/kluctl/lib/status"
	"github.com/kluctl/kluctl/v2/pkg/oci"
	"github.com/kluctl/kluctl/v2/pkg/oci/auth_provider"
	"github.com/kluctl/kluctl/v2/pkg/sourceoverride"
	"github.com/kluctl/kluctl/v2/pkg/types"
	"github.com/kluctl/kluctl/v2/pkg/utils"
	cp "github.com/otiai10/copy"
)

type OciRepoCache struct {
	ctx            context.Context
	updateInterval time.Duration

	ociAuthProvider auth_provider.OciAuthProvider
	ociCache        cache.Cache

	repos      map[gittypes.RepoKey]*OciCacheEntry
	reposMutex sync.Mutex

	repoOverrides sourceoverride.Resolver

	cleanupDirs      []string
	cleanupDirsMutex sync.Mutex
}

type OciCacheEntry struct {
	rp           *OciRepoCache
	url          url.URL
	craneOptions []crane.Option

	pulledDirs   map[types.OciRef]clonedDir
	updateMutex  sync.Mutex
	overridePath string
}

func NewOciRepoCache(ctx context.Context, ociAuthProvider auth_provider.OciAuthProvider, repoOverrides sourceoverride.Resolver, updateInterval time.Duration) *OciRepoCache {
	cacheDir := filepath.Join(utils.GetCacheDir(ctx), "oci")
	c := cache.NewFilesystemCache(cacheDir)

	return &OciRepoCache{
		ctx:             ctx,
		updateInterval:  updateInterval,
		ociAuthProvider: ociAuthProvider,
		ociCache:        c,
		repos:           map[gittypes.RepoKey]*OciCacheEntry{},
		repoOverrides:   repoOverrides,
	}
}

func (rp *OciRepoCache) Clear() {
	rp.cleanupDirsMutex.Lock()
	defer rp.cleanupDirsMutex.Unlock()

	for _, p := range rp.cleanupDirs {
		_ = os.RemoveAll(p)
	}
	rp.cleanupDirs = nil
}

func (rp *OciRepoCache) GetEntry(urlIn string) (*OciCacheEntry, error) {
	rp.reposMutex.Lock()
	defer rp.reposMutex.Unlock()

	urlN, err := url.Parse(urlIn)
	if err != nil {
		return nil, err
	}
	if urlN.Scheme != "oci" {
		return nil, fmt.Errorf("unsupported scheme %s, must be oci://", urlN.Scheme)
	}

	repoKey := gittypes.NewRepoKey("oci", urlN.Host, urlN.Path)

	var overridePath string
	if rp.repoOverrides != nil {
		overridePath, err = rp.repoOverrides.ResolveOverride(rp.ctx, repoKey)
		if err != nil {
			return nil, err
		}
	}

	if overridePath != "" {
		status.WarningOncef(rp.ctx, fmt.Sprintf("git-override-%s", repoKey), "Overriding oci repo %s with local directory %s", urlIn, overridePath)

		e := &OciCacheEntry{
			rp:           rp,
			url:          *urlN,
			pulledDirs:   map[types.OciRef]clonedDir{},
			overridePath: overridePath,
		}
		rp.repos[repoKey] = e
		return e, nil
	}

	e, ok := rp.repos[repoKey]
	if ok {
		return e, nil
	}

	var authOpts []crane.Option
	if rp.ociAuthProvider != nil {
		auth, err := rp.ociAuthProvider.FindAuthEntry(rp.ctx, urlN.String())
		if err != nil {
			return nil, err
		}
		authOpts, err = auth.BuildCraneOptions()
		if err != nil {
			return nil, err
		}
	}

	e = &OciCacheEntry{
		rp:           rp,
		url:          *urlN,
		craneOptions: authOpts,
		pulledDirs:   map[types.OciRef]clonedDir{},
	}
	rp.repos[repoKey] = e

	return e, nil
}

func (e *OciCacheEntry) GetExtractedDir(ref *types.OciRef) (string, git.CheckoutInfo, error) {
	e.updateMutex.Lock()
	defer e.updateMutex.Unlock()

	if ref == nil {
		ref = &types.OciRef{}
	}

	ed, ok := e.pulledDirs[*ref]
	if ok {
		return ed.dir, ed.info, nil
	}

	tmpDir := filepath.Join(utils.GetTmpBaseDir(e.rp.ctx), "oci-pulled")
	err := os.MkdirAll(tmpDir, 0700)
	if err != nil {
		return "", git.CheckoutInfo{}, err
	}

	pullDir, err := os.MkdirTemp(tmpDir, strings.ReplaceAll(e.url.Host, ":", "-")+"-")
	if err != nil {
		return "", git.CheckoutInfo{}, err
	}

	e.rp.cleanupDirsMutex.Lock()
	e.rp.cleanupDirs = append(e.rp.cleanupDirs, pullDir)
	e.rp.cleanupDirsMutex.Unlock()

	if e.overridePath != "" { // local override exist
		err = cp.Copy(e.overridePath, pullDir)
		if err != nil {
			return "", git.CheckoutInfo{}, err
		}
		return pullDir, git.CheckoutInfo{}, err
	}

	image := strings.TrimPrefix(e.url.String(), "oci://") + ref.ImageSuffix()

	md, err := oci.PullCached(e.rp.ctx, e.craneOptions, e.rp.ociCache, image, pullDir)
	if err != nil {
		return "", git.CheckoutInfo{}, err
	}

	var cd clonedDir
	cd.dir = pullDir

	if a, ok := md.Annotations["io.kluctl.image.git_info"]; ok {
		var gitInfo gittypes.GitInfo
		err = json.Unmarshal([]byte(a), &gitInfo)
		if err != nil {
			return pullDir, git.CheckoutInfo{}, err
		}
		if gitInfo.Ref != nil {
			cd.info.CheckedOutRef = *gitInfo.Ref
		}
		cd.info.CheckedOutCommit = gitInfo.Commit
	}

	e.pulledDirs[*ref] = cd
	return cd.dir, cd.info, nil
}
