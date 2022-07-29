package git

import (
	"bytes"
	"fmt"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/protocol/packp"
	auth2 "github.com/kluctl/kluctl/v2/pkg/git/auth"
	"github.com/kluctl/kluctl/v2/pkg/status"
	"strconv"
)

// listRemoteRefsFastSsh will reuse existing ssh connections from a pool
func (g *MirroredGitRepo) listRemoteRefsFastSsh(r *git.Repository, auth auth2.AuthMethodAndCA) ([]*plumbing.Reference, error) {
	var portInt int64 = -1
	if g.url.Port() != "" {
		var err error
		portInt, err = strconv.ParseInt(g.url.Port(), 10, 32)
		if err != nil {
			return nil, err
		}
	}

	s, err := g.sshPool.GetSession(g.ctx, g.url.Hostname(), int(portInt), auth)
	if err != nil {
		return nil, err
	}
	defer s.Close()

	cmd := fmt.Sprintf("git-upload-pack %s", g.url.Path)

	stdout := bytes.NewBuffer(nil)
	stderr := bytes.NewBuffer(nil)
	stdin := bytes.NewBuffer([]byte("0000\n"))

	s.Session.Stdout = stdout
	s.Session.Stderr = stderr
	s.Session.Stdin = stdin

	err = s.Session.Run(cmd)
	if err != nil {
		return nil, fmt.Errorf("git-upload-pack failed: %w\nstderr=%s", err, stderr.String())
	}

	ar := packp.NewAdvRefs()
	err = ar.Decode(stdout)
	if err != nil {
		return nil, err
	}

	allRefs, err := ar.AllReferences()
	if err != nil {
		return nil, err
	}

	refs, err := allRefs.IterReferences()
	if err != nil {
		return nil, err
	}

	var resultRefs []*plumbing.Reference
	err = refs.ForEach(func(ref *plumbing.Reference) error {
		resultRefs = append(resultRefs, ref)
		return nil
	})
	if err != nil {
		return nil, err
	}

	return resultRefs, nil
}

func (g *MirroredGitRepo) listRemoteRefsSlow(r *git.Repository, auth auth2.AuthMethodAndCA) ([]*plumbing.Reference, error) {
	remote, err := r.Remote("origin")
	if err != nil {
		return nil, err
	}

	remoteRefs, err := remote.ListContext(g.ctx, &git.ListOptions{
		Auth:     auth.AuthMethod,
		CABundle: auth.CABundle,
	})
	if err != nil {
		return nil, err
	}
	return remoteRefs, nil
}

func (g *MirroredGitRepo) listRemoteRefs(r *git.Repository, auth auth2.AuthMethodAndCA) ([]*plumbing.Reference, error) {
	if g.url.IsSsh() {
		refs, err := g.listRemoteRefsFastSsh(r, auth)
		if err == nil {
			return refs, nil
		}
		status.Warning(g.ctx, "Fast listing of remote refs failed: %s", err.Error())
	}
	return g.listRemoteRefsSlow(r, auth)
}
