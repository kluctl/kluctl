package git

import (
	"context"
	"fmt"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	auth2 "github.com/kluctl/kluctl/v2/pkg/git/auth"
	git_url "github.com/kluctl/kluctl/v2/pkg/git/git-url"
	"github.com/kluctl/kluctl/v2/pkg/status"
	"github.com/kluctl/kluctl/v2/pkg/utils"
	"github.com/rogpeppe/go-internal/lockedfile"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

var cacheBaseDir = filepath.Join(utils.GetTmpBaseDir(), "git-cache")

type MirroredGitRepo struct {
	ctx context.Context

	url       git_url.GitUrl
	mirrorDir string

	hasUpdated bool

	fileLock     *lockedfile.File
	fileLockPath string

	mutex sync.Mutex
}

func NewMirroredGitRepo(ctx context.Context, u git_url.GitUrl) (*MirroredGitRepo, error) {
	mirrorRepoName := buildMirrorRepoName(u)
	o := &MirroredGitRepo{
		ctx:       ctx,
		url:       u,
		mirrorDir: filepath.Join(cacheBaseDir, mirrorRepoName),
	}

	if !utils.IsDirectory(o.mirrorDir) {
		err := os.MkdirAll(o.mirrorDir, 0o700)
		if err != nil {
			return nil, fmt.Errorf("failed to create mirror repo for %v: %w", u.String(), err)
		}
	}

	o.fileLockPath = filepath.Join(o.mirrorDir, ".cache.lock")
	return o, nil
}

func (g *MirroredGitRepo) Url() git_url.GitUrl {
	return g.url
}

func (g *MirroredGitRepo) HasUpdated() bool {
	return g.hasUpdated
}

func (g *MirroredGitRepo) SetUpdated(u bool) {
	g.hasUpdated = u
}

func (g *MirroredGitRepo) Lock() error {
	g.mutex.Lock()
	defer g.mutex.Unlock()

	if g.fileLock != nil {
		return fmt.Errorf("file %s already locked", g.fileLockPath)
	}

	var err error
	g.fileLock, err = lockedfile.Create(g.fileLockPath)
	if err != nil {
		return fmt.Errorf("locking of %s failed: %w", g.fileLockPath, err)
	}

	return nil
}

func (g *MirroredGitRepo) Unlock() error {
	g.mutex.Lock()
	defer g.mutex.Unlock()

	if g.fileLock == nil {
		return fmt.Errorf("file %s is not locked", g.fileLockPath)
	}

	err := g.fileLock.Close()
	if err != nil {
		status.Warning(g.ctx, "Unlock of %s failed: %v", g.fileLockPath, err)
		return err
	}
	g.fileLock = nil
	return nil
}

func (g *MirroredGitRepo) IsLocked() bool {
	g.mutex.Lock()
	defer g.mutex.Unlock()
	return g.fileLock != nil
}

func (g *MirroredGitRepo) LastUpdateTime() time.Time {
	s, err := ioutil.ReadFile(filepath.Join(g.mirrorDir, ".update-time"))
	if err != nil {
		return time.Time{}
	}
	t, err := time.Parse(time.RFC3339Nano, string(s))
	if err != nil {
		return time.Time{}
	}
	return t
}

func (g *MirroredGitRepo) RemoteRefHashesMap() (map[string]string, error) {
	r, err := git.PlainOpen(g.mirrorDir)
	if err != nil {
		return nil, err
	}

	localRemoteRefs, err := r.References()
	if err != nil {
		return nil, err
	}

	refs := make(map[string]string)
	err = localRemoteRefs.ForEach(func(reference *plumbing.Reference) error {
		name := reference.Name().String()
		hash := reference.Hash().String()
		if reference.Hash().IsZero() {
			reference, err = r.Reference(reference.Name(), true)
			if err != nil {
				return err
			}
			hash = reference.Hash().String()
		}
		refs[name] = hash
		return nil
	})
	if err != nil {
		return nil, err
	}
	return refs, nil
}

func (g *MirroredGitRepo) DefaultRef() (string, error) {
	r, err := git.PlainOpen(g.mirrorDir)
	if err != nil {
		return "", err
	}

	ref, err := r.Reference("HEAD", false)
	if err != nil {
		return "", err
	}

	s := ref.Target().String()
	return s, nil
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

func (g *MirroredGitRepo) update(s *status.StatusContext, repoDir string, authProviders *auth2.GitAuthProviders) error {
	r, err := git.PlainOpen(repoDir)
	if err != nil {
		return err
	}

	auth := authProviders.BuildAuth(g.ctx, g.url)

	remote, err := r.Remote("origin")
	if err != nil {
		return err
	}

	remoteRefs, err := remote.ListContext(g.ctx, &git.ListOptions{
		Auth:     auth.AuthMethod,
		CABundle: auth.CABundle,
	})
	if err != nil {
		return err
	}
	remoteRefsMap := make(map[plumbing.ReferenceName]*plumbing.Reference)
	for _, reference := range remoteRefs {
		remoteRefsMap[reference.Name()] = reference
	}

	localRemoteRefs, err := r.References()
	if err != nil {
		return err
	}

	localRemoteRefsMap := make(map[plumbing.ReferenceName]*plumbing.Reference)
	_ = localRemoteRefs.ForEach(func(reference *plumbing.Reference) error {
		localRemoteRefsMap[reference.Name()] = reference
		return nil
	})

	var toDelete []*plumbing.Reference
	changed := false
	for name, ref := range remoteRefsMap {
		if name.String() != "HEAD" && !strings.HasPrefix(name.String(), "refs/heads/") && !strings.HasPrefix(name.String(), "refs/tags/") {
			// we only fetch branches and tags
			continue
		}
		if x, ok := localRemoteRefsMap[name]; !ok {
			changed = true
		} else if *x != *ref {
			changed = true
		}
	}
	for name, ref := range localRemoteRefsMap {
		if _, ok := remoteRefsMap[name]; !ok {
			toDelete = append(toDelete, ref)
		}
	}

	if changed {
		err = remote.FetchContext(g.ctx, &git.FetchOptions{
			Auth:     auth.AuthMethod,
			CABundle: auth.CABundle,
			Tags:     git.AllTags,
			Force:    true,
		})
		if err != nil && err != git.NoErrAlreadyUpToDate {
			return err
		}
	}

	for _, ref := range toDelete {
		err = r.Storer.RemoveReference(ref.Name())
		if err != nil {
			return err
		}
	}

	// update default branch, referenced via HEAD
	// we assume that HEAD is a symbolic ref and don't care about old git versions
	for _, ref := range remoteRefs {
		if ref.Name() == "HEAD" {
			err = r.Storer.SetReference(ref)
			if err != nil {
				return err
			}
			break
		}
	}

	_ = ioutil.WriteFile(filepath.Join(g.mirrorDir, ".update-time"), []byte(time.Now().Format(time.RFC3339Nano)), 0644)

	return nil
}

func (g *MirroredGitRepo) cloneOrUpdate(s *status.StatusContext, authProviders *auth2.GitAuthProviders) error {
	initMarker := filepath.Join(g.mirrorDir, ".cache2.init")
	if utils.IsFile(initMarker) {
		return g.update(s, g.mirrorDir, authProviders)
	}
	err := g.cleanupMirrorDir()
	if err != nil {
		return err
	}

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

	err = g.update(s, tmpMirrorDir, authProviders)
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

func (g *MirroredGitRepo) Update(authProviders *auth2.GitAuthProviders) error {
	s := status.Start(g.ctx, "Updating git cache for %s", g.url.String())
	err := g.cloneOrUpdate(s, authProviders)
	if err != nil {
		s.FailedWithMessage(err.Error())
		return err
	}
	g.hasUpdated = true
	s.Success()
	return nil
}

func (g *MirroredGitRepo) CloneProject(ref string, targetDir string) error {
	if !g.IsLocked() || !g.hasUpdated {
		panic("tried to clone from a project that is not locked/updated")
	}

	status.Trace(g.ctx, "Cloning git project: url='%s', ref='%s', target='%s'", g.url.String(), ref, targetDir)

	err := PoorMansClone(g.mirrorDir, targetDir, ref)
	if err != nil {
		return fmt.Errorf("failed to clone %s from %s: %w", ref, g.url.String(), err)
	}
	return nil
}

func (g *MirroredGitRepo) ReadFile(ref string, path string) ([]byte, error) {
	if !g.IsLocked() || !g.hasUpdated {
		panic("tried to read a file from a project that is not locked/updated")
	}

	doError := func(err error) ([]byte, error) {
		return nil, fmt.Errorf("failed to read file from git repostory: %w", err)
	}

	r, err := git.PlainOpen(g.mirrorDir)
	if err != nil {
		return nil, err
	}

	if ref == "" {
		ref = "HEAD"
	}

	h, err := r.ResolveRevision(plumbing.Revision(ref))
	if err != nil {
		return doError(err)
	}

	commit, err := object.GetCommit(r.Storer, *h)
	if err != nil {
		return doError(err)
	}
	tree, err := commit.Tree()
	if err != nil {
		return doError(err)
	}

	f, err := tree.File(path)
	if err != nil {
		return doError(err)
	}
	reader, err := f.Reader()
	if err != nil {
		return doError(err)
	}
	defer reader.Close()

	return ioutil.ReadAll(reader)
}

func buildMirrorRepoName(u git_url.GitUrl) string {
	r := filepath.Base(u.Path)
	r = strings.ReplaceAll(r, "/", "-")
	r = strings.ReplaceAll(r, "\\", "-")
	if strings.HasSuffix(r, ".git") {
		r = r[:len(r)-len(".git")]
	}
	r += "-" + utils.Sha256String(u.String())[:6]
	return r
}
