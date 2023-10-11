package repocache

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/kluctl/kluctl/v2/pkg/git"
	"github.com/kluctl/kluctl/v2/pkg/oci/auth_provider"
	"github.com/kluctl/kluctl/v2/pkg/oci/client"
	"github.com/kluctl/kluctl/v2/pkg/status"
	"github.com/kluctl/kluctl/v2/pkg/types"
	"github.com/kluctl/kluctl/v2/pkg/types/result"
	"github.com/kluctl/kluctl/v2/pkg/utils"
	cp "github.com/otiai10/copy"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type OciRepoCache struct {
	ctx            context.Context
	updateInterval time.Duration

	ociAuthProvider auth_provider.OciAuthProvider

	repos      map[types.RepoKey]*OciCacheEntry
	reposMutex sync.Mutex

	repoOverrides []RepoOverride

	cleanupDirs      []string
	cleanupDirsMutex sync.Mutex
}

type OciCacheEntry struct {
	rp          *OciRepoCache
	url         url.URL
	ociClient   *client.Client
	ociCacheDir string

	pulledDirs   map[types.OciRef]clonedDir
	updateMutex  sync.Mutex
	overridePath string
}

func NewOciRepoCache(ctx context.Context, ociAuthProvider auth_provider.OciAuthProvider, repoOverrides []RepoOverride, updateInterval time.Duration) *OciRepoCache {
	return &OciRepoCache{
		ctx:             ctx,
		updateInterval:  updateInterval,
		ociAuthProvider: ociAuthProvider,
		repos:           map[types.RepoKey]*OciCacheEntry{},
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

	repoKey := types.NewRepoKey("oci", urlN.Host, urlN.Path)

	overridePath, err := findRepoOverride(rp.repoOverrides, repoKey)
	if err != nil {
		return nil, err
	}

	if overridePath != "" {
		status.WarningOncef(rp.ctx, fmt.Sprintf("git-override-%s", repoKey), "Overriding oci repo %s with local directory %s", urlIn, overridePath)

		e := &OciCacheEntry{
			rp:           rp,
			url:          *urlN,
			ociClient:    nil, // mark as overridden
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

	hostOciCacheDir := filepath.Join(utils.GetTmpBaseDir(rp.ctx), "oci")
	hostOciCacheDir = filepath.Join(hostOciCacheDir, strings.ReplaceAll(urlN.Host, ":", "-"))

	ociCacheDir := filepath.Join(hostOciCacheDir, urlN.Path)
	err = utils.CheckSubInDir(hostOciCacheDir, ociCacheDir)
	if err != nil {
		return nil, err
	}

	err = os.MkdirAll(ociCacheDir, 0700)
	if err != nil {
		return nil, err
	}

	rp.cleanupDirsMutex.Lock()
	rp.cleanupDirs = append(rp.cleanupDirs, ociCacheDir)
	rp.cleanupDirsMutex.Unlock()

	var clientOpts []crane.Option
	if rp.ociAuthProvider != nil {
		auth, err := rp.ociAuthProvider.FindAuthEntry(rp.ctx, urlN.String())
		if err != nil {
			return nil, err
		}
		authOpts, err := auth.BuildCraneOptions()
		if err != nil {
			return nil, err
		}
		clientOpts = append(clientOpts, authOpts...)
	}

	ociClient := client.NewClient(clientOpts)

	e = &OciCacheEntry{
		rp:          rp,
		url:         *urlN,
		ociClient:   ociClient,
		ociCacheDir: ociCacheDir,
		pulledDirs:  map[types.OciRef]clonedDir{},
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

	ociDir, err := os.MkdirTemp(e.ociCacheDir, "")
	if err != nil {
		return "", git.CheckoutInfo{}, err
	}

	if e.ociClient == nil { // local override exist
		err = cp.Copy(e.overridePath, ociDir)
		if err != nil {
			return "", git.CheckoutInfo{}, err
		}
		return ociDir, git.CheckoutInfo{}, err
	}

	image := strings.TrimPrefix(e.url.String(), "oci://") + ":" + ref.String()

	md, err := e.ociClient.Pull(e.rp.ctx, image, ociDir)
	if err != nil {
		return "", git.CheckoutInfo{}, err
	}

	var cd clonedDir
	cd.dir = ociDir

	if a, ok := md.Annotations["io.kluctl.image.git_info"]; ok {
		var gitInfo result.GitInfo
		err = json.Unmarshal([]byte(a), &gitInfo)
		if err != nil {
			return ociDir, git.CheckoutInfo{}, err
		}
		if gitInfo.Ref != nil {
			cd.info.CheckedOutRef = *gitInfo.Ref
		}
		cd.info.CheckedOutCommit = gitInfo.Commit
	}

	e.pulledDirs[*ref] = cd
	return cd.dir, cd.info, nil
}
