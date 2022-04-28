package git

import (
	"context"
	"fmt"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	auth2 "github.com/kluctl/kluctl/v2/pkg/git/auth"
	git_url "github.com/kluctl/kluctl/v2/pkg/git/git-url"
	"github.com/kluctl/kluctl/v2/pkg/utils"
	log "github.com/sirupsen/logrus"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"
)
import "github.com/gofrs/flock"

var cacheBaseDir = filepath.Join(utils.GetTmpBaseDir(), "git-cache")

type MirroredGitRepo struct {
	url       git_url.GitUrl
	mirrorDir string

	hasUpdated bool
	fileLock   *flock.Flock

	remoteRefs []*plumbing.Reference
}

func NewMirroredGitRepo(u git_url.GitUrl) (*MirroredGitRepo, error) {
	mirrorRepoName := buildMirrorRepoName(u)
	o := &MirroredGitRepo{
		url:       u,
		mirrorDir: filepath.Join(cacheBaseDir, mirrorRepoName),
	}

	if !utils.IsDirectory(o.mirrorDir) {
		err := os.MkdirAll(o.mirrorDir, 0o700)
		if err != nil {
			return nil, fmt.Errorf("failed to create mirror repo for %v: %w", u.String(), err)
		}
	}
	o.fileLock = flock.New(filepath.Join(o.mirrorDir, ".cache.lock"))
	return o, nil
}

func (g *MirroredGitRepo) HasUpdated() bool {
	return g.hasUpdated
}

func (g *MirroredGitRepo) Lock(ctx context.Context) error {
	ok, err := g.fileLock.TryLockContext(ctx, time.Millisecond*100)
	if err != nil {
		return fmt.Errorf("locking of %s failed: %w", g.fileLock.Path(), err)
	}
	if !ok {
		return fmt.Errorf("locking of %s failed: unkown reason", g.fileLock.Path())
	}
	return nil
}

func (g *MirroredGitRepo) Unlock() error {
	err := g.fileLock.Unlock()
	if err != nil {
		log.Warningf("Unlock of %s failed: %v", g.fileLock.Path(), err)
		return err
	}
	return nil
}

func (g *MirroredGitRepo) WithLock(ctx context.Context, cb func() error) error {
	err := g.Lock(ctx)
	if err != nil {
		return err
	}
	defer g.Unlock()
	return cb()
}

func (g *MirroredGitRepo) MaybeWithLock(ctx context.Context, lock bool, cb func() error) error {
	if lock {
		return g.WithLock(ctx, cb)
	}
	return cb()
}

func (g *MirroredGitRepo) RemoteRefHashesMap() map[string]string {
	refs := make(map[string]string)
	for _, r := range g.remoteRefs {
		refs[r.Name().String()] = r.Hash().String()
	}
	return refs
}

func (g *MirroredGitRepo) DefaultRef() *string {
	for _, ref := range g.remoteRefs {
		if ref.Name() == "HEAD" {
			if ref.Type() == plumbing.SymbolicReference {
				s := string(ref.Target())
				return &s
			}
		}
	}
	return nil
}

func (g *MirroredGitRepo) buildRepositoryObject() (*git.Repository, error) {
	return git.PlainOpen(g.mirrorDir)
}

func (g *MirroredGitRepo) cleanupMirrorDir() error {
	if utils.IsDirectory(g.mirrorDir) {
		files, err := ioutil.ReadDir(g.mirrorDir)
		if err != nil {
			return err
		}
		for _, fi := range files {
			if fi.Name() == ".cache.lock" {
				continue
			}
			_ = os.RemoveAll(filepath.Join(g.mirrorDir, fi.Name()))
		}
	}
	return nil
}

func (g *MirroredGitRepo) update(ctx context.Context, repoDir string, authProviders *auth2.GitAuthProviders) error {
	log.Infof("Updating mirror repo: url='%v'", g.url.String())
	r, err := git.PlainOpen(repoDir)
	if err != nil {
		return err
	}

	auth := authProviders.BuildAuth(g.url)

	remote, err := r.Remote("origin")
	if err != nil {
		return err
	}

	g.remoteRefs, err = remote.ListContext(ctx, &git.ListOptions{
		Auth:     auth.AuthMethod,
		CABundle: auth.CABundle,
	})
	if err != nil {
		return err
	}
	remoteRefsMap := make(map[plumbing.ReferenceName]bool)
	for _, reference := range g.remoteRefs {
		remoteRefsMap[reference.Name()] = true
	}

	err = remote.FetchContext(ctx, &git.FetchOptions{
		Auth:     auth.AuthMethod,
		CABundle: auth.CABundle,
		Progress: os.Stdout,
		Tags:     git.AllTags,
		Force:    true,
	})
	if err != nil && err != git.NoErrAlreadyUpToDate {
		return err
	}

	localRemoteRefs, err := r.References()
	if err != nil {
		return err
	}
	var toDelete []plumbing.Reference
	err = localRemoteRefs.ForEach(func(reference *plumbing.Reference) error {
		if _, ok := remoteRefsMap[reference.Name()]; !ok {
			toDelete = append(toDelete, *reference)
		}
		return nil
	})
	if err != nil {
		return err
	}
	for _, ref := range toDelete {
		err = r.Storer.RemoveReference(ref.Name())
		if err != nil {
			return err
		}
	}

	// update default branch, referenced via HEAD
	// we assume that HEAD is a symbolic ref and don't care about old git versions
	for _, ref := range g.remoteRefs {
		if ref.Name() == "HEAD" {
			err = r.Storer.SetReference(ref)
			if err != nil {
				return err
			}
			break
		}
	}

	return nil
}

func (g *MirroredGitRepo) cloneOrUpdate(ctx context.Context, authProviders *auth2.GitAuthProviders) error {
	initMarker := filepath.Join(g.mirrorDir, ".cache2.init")
	if utils.IsFile(initMarker) {
		return g.update(ctx, g.mirrorDir, authProviders)
	}
	err := g.cleanupMirrorDir()
	if err != nil {
		return err
	}

	log.Infof("Cloning mirror repo at %v", g.mirrorDir)

	tmpMirrorDir, err := ioutil.TempDir(utils.GetTmpBaseDir(), "mirror-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpMirrorDir)

	repo, err := git.PlainInit(tmpMirrorDir, true)
	if err != nil {
		return err
	}

	_, err = repo.CreateRemote(&config.RemoteConfig{
		Name: "origin",
		URLs: []string{g.url.String()},
		Fetch: []config.RefSpec{
			"+refs/heads/*:refs/heads/*",
			"+refs/tags/*:refs/tags/*",
		},
	})
	if err != nil {
		return err
	}

	err = g.update(ctx, tmpMirrorDir, authProviders)
	if err != nil {
		return err
	}

	files, err := ioutil.ReadDir(tmpMirrorDir)
	if err != nil {
		return err
	}
	for _, fi := range files {
		err = os.Rename(filepath.Join(tmpMirrorDir, fi.Name()), filepath.Join(g.mirrorDir, fi.Name()))
		if err != nil {
			return err
		}
	}
	err = utils.Touch(initMarker)
	if err != nil {
		return err
	}
	return nil
}

func (g *MirroredGitRepo) Update(ctx context.Context, authProviders *auth2.GitAuthProviders) error {
	err := g.cloneOrUpdate(ctx, authProviders)
	if err != nil {
		return err
	}
	g.hasUpdated = true
	return nil
}

func (g *MirroredGitRepo) CloneProject(ctx context.Context, ref string, targetDir string) error {
	if !g.fileLock.Locked() || !g.hasUpdated {
		log.Fatalf("tried to clone from a project that is not locked/updated")
	}

	log.Debugf("Cloning git project: url='%s', ref='%s', target='%s'", g.url.String(), ref, targetDir)

	err := PoorMansClone(g.mirrorDir, targetDir, ref)
	if err != nil {
		return fmt.Errorf("failed to clone %s from %s: %w", ref, g.url.String(), err)
	}
	return nil
}

func buildMirrorRepoName(u git_url.GitUrl) string {
	r := filepath.Base(u.Path)
	if strings.HasSuffix(r, ".git") {
		r = r[:len(r)-len(".git")]
	}
	r += "-" + utils.Sha256String(u.String())[:6]
	return r
}
