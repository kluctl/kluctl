package git

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/protocol/packp/capability"
	"github.com/go-git/go-git/v5/plumbing/transport"
	auth2 "github.com/kluctl/kluctl/lib/git/auth"
	_ "github.com/kluctl/kluctl/lib/git/ssh-pool"
	ssh_pool "github.com/kluctl/kluctl/lib/git/ssh-pool"
	"github.com/kluctl/kluctl/lib/git/types"
	"github.com/kluctl/kluctl/lib/status"
	"github.com/rogpeppe/go-internal/lockedfile"
)

func init() {
	// see https://github.com/go-git/go-git/pull/613
	old := transport.UnsupportedCapabilities
	transport.UnsupportedCapabilities = nil
	for _, c := range old {
		if c == capability.MultiACK || c == capability.MultiACKDetailed {
			continue
		}
		transport.UnsupportedCapabilities = append(transport.UnsupportedCapabilities, c)
	}
}

var defaultFetch = []config.RefSpec{
	"+refs/heads/*:refs/heads/*",
	"+refs/tags/*:refs/tags/*",
}

type MirroredGitRepo struct {
	ctx context.Context

	baseDir       string
	sshPool       *ssh_pool.SshPool
	authProviders *auth2.GitAuthProviders

	url       types.GitUrl
	mirrorDir string

	hasUpdated bool

	fileLock     *lockedfile.File
	fileLockPath string

	mutex sync.Mutex
}

func NewMirroredGitRepo(ctx context.Context, u types.GitUrl, baseDir string, sshPool *ssh_pool.SshPool, authProviders *auth2.GitAuthProviders) (*MirroredGitRepo, error) {
	mirrorRepoName := buildMirrorRepoName(u)
	o := &MirroredGitRepo{
		ctx:           ctx,
		baseDir:       baseDir,
		sshPool:       sshPool,
		authProviders: authProviders,
		url:           u,
		mirrorDir:     filepath.Join(baseDir, mirrorRepoName),
	}

	st, err := os.Stat(o.mirrorDir)
	if err != nil {
		err = os.MkdirAll(o.mirrorDir, 0o700)
		if err != nil {
			return nil, fmt.Errorf("failed to create mirror repo for %v: %w", u.String(), err)
		}
	} else if !st.IsDir() {
		return nil, fmt.Errorf("%s is not a directory", o.mirrorDir)
	}

	o.fileLockPath = filepath.Join(o.mirrorDir, ".cache.lock")
	return o, nil
}

func (g *MirroredGitRepo) Url() types.GitUrl {
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
	s, err := os.ReadFile(filepath.Join(g.mirrorDir, ".update-time"))
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

func (g *MirroredGitRepo) DefaultRef() (*types.GitRef, error) {
	r, err := git.PlainOpen(g.mirrorDir)
	if err != nil {
		return nil, err
	}

	ref, err := r.Reference("HEAD", false)
	if err != nil {
		return nil, err
	}

	s := ref.Target().String()
	ret, err := types.ParseGitRef(s)
	if err != nil {
		return nil, err
	}
	return &ret, nil
}

func (g *MirroredGitRepo) Delete() error {
	if !g.IsLocked() {
		panic("tried to delete a project that is not locked")
	}
	err := os.RemoveAll(g.mirrorDir)
	if err != nil {
		return err
	}
	return nil
}

func (g *MirroredGitRepo) buildRepositoryObject() (*git.Repository, error) {
	return git.PlainOpen(g.mirrorDir)
}

func (g *MirroredGitRepo) cleanupMirrorDir() error {
	st, err := os.Stat(g.mirrorDir)
	if err == nil && st.IsDir() {
		files, err := os.ReadDir(g.mirrorDir)
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

func (g *MirroredGitRepo) update(repoDir string) error {
	r, err := git.PlainOpen(repoDir)
	if err != nil {
		return err
	}

	auth, err := g.authProviders.BuildAuth(g.ctx, g.url)
	if err != nil {
		return err
	}

	remoteRefs, err := ListRemoteRefs(g.ctx, g.url, g.sshPool, auth)
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
		remote, err := r.Remote("origin")
		if err != nil {
			return err
		}

		// go-git does not respect the context deadline in some situations, especially after errors occur internally.
		// This leads to hanging fetches, which can easily deadlock the whole kluctl process. The only way to handle
		// this currently is to panic when the deadline is exceeded too much.
		err = RunWithDeadlineAndPanic(g.ctx, 5*time.Second, func() error {
			return remote.FetchContext(g.ctx, &git.FetchOptions{
				Auth:     auth.AuthMethod,
				CABundle: auth.CABundle,
				Tags:     git.AllTags,
				Force:    true,
			})
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

	_ = os.WriteFile(filepath.Join(g.mirrorDir, ".update-time"), []byte(time.Now().Format(time.RFC3339Nano)), 0644)

	return nil
}

func (g *MirroredGitRepo) cloneOrUpdate() error {
	initMarker := filepath.Join(g.mirrorDir, ".cache2.init")
	st, err := os.Stat(initMarker)
	if err == nil && st.Mode().IsRegular() {
		err = g.update(g.mirrorDir)
		if err == nil {
			return nil
		} else if strings.Contains(err.Error(), "multi_ack") {
			// looks like the server tried to do multi_ack/multi_ack_detailed, which is not supported.
			// in that case, retry a full clone which does hopefully not rely on multi_ack.
			// See https://github.com/go-git/go-git/pull/613
			// TODO remove this when https://github.com/go-git/go-git/issues/64 gets fully fixed
			status.Tracef(g.ctx, "Got multi_ack related error from remote. Retrying full clone: %v", err)
		} else {
			return err
		}
	}
	err = g.cleanupMirrorDir()
	if err != nil {
		return err
	}

	tmpMirrorDir, err := os.MkdirTemp(g.baseDir, "tmp-mirror-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpMirrorDir)

	repo, err := git.PlainInit(tmpMirrorDir, true)
	if err != nil {
		return err
	}

	_, err = repo.CreateRemote(&config.RemoteConfig{
		Name:  "origin",
		URLs:  []string{g.url.String()},
		Fetch: defaultFetch,
	})
	if err != nil {
		return err
	}

	err = g.update(tmpMirrorDir)
	if err != nil {
		return err
	}

	files, err := os.ReadDir(tmpMirrorDir)
	if err != nil {
		return err
	}
	for _, fi := range files {
		err = os.Rename(filepath.Join(tmpMirrorDir, fi.Name()), filepath.Join(g.mirrorDir, fi.Name()))
		if err != nil {
			return err
		}
	}
	f, err := os.Create(initMarker)
	if err != nil {
		return err
	}
	defer f.Close()
	return nil
}

func (g *MirroredGitRepo) Update() error {
	if !g.IsLocked() {
		panic("tried to update a project that is not locked")
	}

	err := g.cloneOrUpdate()
	if err != nil {
		return err
	}
	g.hasUpdated = true
	return nil
}

func (g *MirroredGitRepo) CloneProjectByCommit(commit string, targetDir string) error {
	if !g.IsLocked() || !g.hasUpdated {
		panic("tried to clone from a project that is not locked/updated")
	}

	err := PoorMansClone(g.mirrorDir, targetDir, &git.CheckoutOptions{Hash: plumbing.NewHash(commit)})
	if err != nil {
		return fmt.Errorf("failed to clone %s from %s: %w", commit, g.url.String(), err)
	}
	return nil
}

func (g *MirroredGitRepo) GetGitTreeByCommit(commitHash string) (*object.Tree, error) {
	if !g.IsLocked() || !g.hasUpdated {
		panic("tried to read a file from a project that is not locked/updated")
	}

	doError := func(err error) (*object.Tree, error) {
		return nil, fmt.Errorf("failed to read file from git repostory: %w", err)
	}

	r, err := git.PlainOpen(g.mirrorDir)
	if err != nil {
		return nil, err
	}

	h := plumbing.NewHash(commitHash)

	commit, err := object.GetCommit(r.Storer, h)
	if err != nil {
		return doError(err)
	}
	tree, err := commit.Tree()
	if err != nil {
		return doError(err)
	}
	return tree, nil
}

func (g *MirroredGitRepo) GetObjectByHash(hash string) (object.Object, error) {
	if !g.IsLocked() || !g.hasUpdated {
		panic("tried to read a file from a project that is not locked/updated")
	}

	r, err := git.PlainOpen(g.mirrorDir)
	if err != nil {
		return nil, err
	}

	h := plumbing.NewHash(hash)
	return object.GetObject(r.Storer, h)
}

func buildMirrorRepoName(u types.GitUrl) string {
	h := sha256.New()
	h.Write([]byte(u.String()))
	h2 := hex.EncodeToString(h.Sum(nil))

	r := filepath.Base(u.Path)
	r = strings.ReplaceAll(r, "/", "-")
	r = strings.ReplaceAll(r, "\\", "-")
	if r != ".git" && strings.HasSuffix(r, ".git") {
		r = r[:len(r)-len(".git")]
	}
	r += "-" + h2[:6]
	return r
}
