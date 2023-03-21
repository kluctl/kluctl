package git

import (
	"bytes"
	"context"
	"fmt"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/protocol/packp"
	"github.com/go-git/go-git/v5/storage/memory"
	auth2 "github.com/kluctl/kluctl/v2/pkg/git/auth"
	git_url "github.com/kluctl/kluctl/v2/pkg/git/git-url"
	ssh_pool "github.com/kluctl/kluctl/v2/pkg/git/ssh-pool"
	"strconv"
)

// ListRemoteRefsFastSsh will reuse existing ssh connections from a pool
func ListRemoteRefsFastSsh(ctx context.Context, url git_url.GitUrl, sshPool *ssh_pool.SshPool, auth auth2.AuthMethodAndCA) ([]*plumbing.Reference, error) {
	var portInt int64 = 22
	if url.Port() != "" {
		var err error
		portInt, err = strconv.ParseInt(url.Port(), 10, 32)
		if err != nil {
			return nil, err
		}
	}

	s, err := sshPool.GetSession(ctx, url.Hostname(), int(portInt), auth)
	if err != nil {
		return nil, err
	}
	defer s.Close()

	cmd := fmt.Sprintf("git-upload-pack %s", url.Path)

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

func ListRemoteRefsSlow(ctx context.Context, url git_url.GitUrl, auth auth2.AuthMethodAndCA) ([]*plumbing.Reference, error) {
	storage := memory.NewStorage()
	remote := git.NewRemote(storage, &config.RemoteConfig{
		Name:  "origin",
		URLs:  []string{url.String()},
		Fetch: defaultFetch,
	})

	remoteRefs, err := remote.ListContext(ctx, &git.ListOptions{
		Auth:     auth.AuthMethod,
		CABundle: auth.CABundle,
	})
	if err != nil {
		return nil, err
	}
	return remoteRefs, nil
}

func ListRemoteRefs(ctx context.Context, url git_url.GitUrl, sshPool *ssh_pool.SshPool, auth auth2.AuthMethodAndCA) ([]*plumbing.Reference, error) {
	if url.IsSsh() {
		refs, err := ListRemoteRefsFastSsh(ctx, url, sshPool, auth)
		if err == nil {
			return refs, nil
		}
	}
	return ListRemoteRefsSlow(ctx, url, auth)
}
