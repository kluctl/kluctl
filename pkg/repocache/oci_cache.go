package repocache

import (
	"context"
	"fmt"
	"github.com/kluctl/kluctl/v2/pkg/git"
	"github.com/kluctl/kluctl/v2/pkg/status"
	"github.com/kluctl/kluctl/v2/pkg/tar"
	"github.com/kluctl/kluctl/v2/pkg/types"
	"github.com/kluctl/kluctl/v2/pkg/utils"
	cp "github.com/otiai10/copy"
	"net/url"
	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/content/file"
	"oras.land/oras-go/v2/registry/remote"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type OciRepoCache struct {
	ctx            context.Context
	updateInterval time.Duration

	repos      map[types.GitRepoKey]*OciCacheEntry
	reposMutex sync.Mutex

	repoOverrides []RepoOverride

	cleanupDirs      []string
	cleanupDirsMutex sync.Mutex
}

type OciCacheEntry struct {
	rp   *OciRepoCache
	url  url.URL
	repo *remote.Repository

	extractedDirs map[types.OciRef]string
	updateMutex   sync.Mutex
	overridePath  string
}

func NewOciRepoCache(ctx context.Context, repoOverrides []RepoOverride, updateInterval time.Duration) *OciRepoCache {
	return &OciRepoCache{
		ctx:            ctx,
		updateInterval: updateInterval,
		repos:          map[types.GitRepoKey]*OciCacheEntry{},
		repoOverrides:  repoOverrides,
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

	repoKey := types.NewRepoKey(urlN.Host, urlN.Path)

	overridePath, err := findRepoOverride(rp.repoOverrides, repoKey)
	if err != nil {
		return nil, err
	}

	if overridePath != "" {
		status.WarningOncef(rp.ctx, fmt.Sprintf("git-override-%s", repoKey), "Overriding oci repo %s with local directory %s", urlIn, overridePath)

		e := &OciCacheEntry{
			rp:            rp,
			url:           *urlN,
			repo:          nil, // mark as overridden
			extractedDirs: map[types.OciRef]string{},
			overridePath:  overridePath,
		}
		rp.repos[repoKey] = e
		return e, nil
	}

	e, ok := rp.repos[repoKey]
	if !ok {
		repo, err := remote.NewRepository(strings.TrimPrefix(urlN.String(), "oci://"))
		if err != nil {
			return nil, err
		}

		repo.PlainHTTP = true

		e = &OciCacheEntry{
			rp:            rp,
			url:           *urlN,
			repo:          repo,
			extractedDirs: map[types.OciRef]string{},
		}
		rp.repos[repoKey] = e
	}
	return e, nil
}

func (e *OciCacheEntry) GetExtractedDir(ref *types.OciRef) (string, git.CheckoutInfo, error) {
	e.updateMutex.Lock()
	defer e.updateMutex.Unlock()

	if ref == nil {
		ref = &types.OciRef{}
	}

	repoName := path.Base(e.url.Path) + "-"
	repoName += ref.String() + "-"
	repoName = strings.ReplaceAll(repoName, "/", "-")
	repoName = strings.ReplaceAll(repoName, ":", "-")

	ociDir := filepath.Join(utils.GetTmpBaseDir(e.rp.ctx), "oci")
	err := os.MkdirAll(ociDir, 0700)
	if err != nil {
		return "", git.CheckoutInfo{}, err
	}

	ociDir, err = os.MkdirTemp(ociDir, repoName)
	if err != nil {
		return "", git.CheckoutInfo{}, err
	}

	pulledDir := filepath.Join(ociDir, "pulled")
	extractedDir := filepath.Join(ociDir, "extracted")
	err = os.Mkdir(pulledDir, 0700)
	if err != nil {
		return "", git.CheckoutInfo{}, err
	}
	err = os.Mkdir(extractedDir, 0700)
	if err != nil {
		return "", git.CheckoutInfo{}, err
	}

	e.rp.cleanupDirsMutex.Lock()
	e.rp.cleanupDirs = append(e.rp.cleanupDirs, ociDir)
	e.rp.cleanupDirsMutex.Unlock()

	if e.repo == nil { // local override exist
		err = cp.Copy(e.overridePath, extractedDir)
		if err != nil {
			return "", git.CheckoutInfo{}, err
		}
		return extractedDir, git.CheckoutInfo{}, err
	}

	fs, err := file.New(pulledDir)
	if err != nil {
		return "", git.CheckoutInfo{}, err
	}

	manifestDescriptor, err := oras.Copy(e.rp.ctx, e.repo, ref.String(), fs, "", oras.CopyOptions{})
	if err != nil {
		return "", git.CheckoutInfo{}, err
	}
	_ = manifestDescriptor

	tgz, err := os.Open(filepath.Join(pulledDir, "artifact.tgz"))
	if err != nil {
		return "", git.CheckoutInfo{}, err
	}
	defer tgz.Close()

	err = tar.Untar(tgz, extractedDir, tar.WithSkipSymlinks())
	if err != nil {
		return "", git.CheckoutInfo{}, err
	}

	e.extractedDirs[*ref] = extractedDir

	return extractedDir, git.CheckoutInfo{}, nil
}
