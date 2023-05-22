package controllers

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	kluctlv1 "github.com/kluctl/kluctl/v2/api/v1beta1"
	"github.com/kluctl/kluctl/v2/pkg/git/auth"
	"github.com/kluctl/kluctl/v2/pkg/git/messages"
	"github.com/kluctl/kluctl/v2/pkg/repocache"
	"github.com/kluctl/kluctl/v2/pkg/utils"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"os"
	"path/filepath"
	ctrl "sigs.k8s.io/controller-runtime"
)

func (r *KluctlDeploymentReconciler) getGitSecret(ctx context.Context, source *kluctlv1.ProjectSource, objNs string) (*corev1.Secret, error) {
	if source == nil || source.SecretRef == nil {
		return nil, nil
	}

	// Attempt to retrieve secret
	name := types.NamespacedName{
		Namespace: objNs,
		Name:      source.SecretRef.Name,
	}
	var secret corev1.Secret
	if err := r.Get(ctx, name, &secret); err != nil {
		return nil, fmt.Errorf("failed to get secret '%s': %w", name.String(), err)
	}
	return &secret, nil
}

func (r *KluctlDeploymentReconciler) buildGitAuth(ctx context.Context, gitSecret *corev1.Secret) (*auth.GitAuthProviders, error) {
	log := ctrl.LoggerFrom(ctx)
	ga := auth.NewDefaultAuthProviders("KLUCTL_GIT", &messages.MessageCallbacks{
		WarningFn: func(s string) {
			log.Info(s)
		},
		TraceFn: func(s string) {
			log.V(1).Info(s)
		},
	})

	if gitSecret == nil {
		return ga, nil
	}

	e := auth.AuthEntry{
		Host:     "*",
		Username: "*",
	}

	if x, ok := gitSecret.Data["username"]; ok {
		e.Username = string(x)
	}
	if x, ok := gitSecret.Data["password"]; ok {
		e.Password = string(x)
	}
	if x, ok := gitSecret.Data["caFile"]; ok {
		e.CABundle = x
	}
	if x, ok := gitSecret.Data["known_hosts"]; ok {
		e.KnownHosts = x
	}
	if x, ok := gitSecret.Data["identity"]; ok {
		e.SshKey = x
	}

	var la auth.ListAuthProvider
	la.AddEntry(e)
	ga.RegisterAuthProvider(&la, false)
	return ga, nil
}

func (r *KluctlDeploymentReconciler) buildRepoCache(ctx context.Context, secret *corev1.Secret) (*repocache.GitRepoCache, error) {
	// make sure we use a unique repo cache per set of credentials
	h := sha256.New()
	if secret == nil {
		h.Write([]byte("no-secret"))
	} else {
		m := json.NewEncoder(h)
		err := m.Encode(secret.Data)
		if err != nil {
			return nil, err
		}
	}
	h2 := hex.EncodeToString(h.Sum(nil))

	tmpBaseDir := filepath.Join(os.TempDir(), "kluctl-controller-repo-cache", h2)
	err := os.MkdirAll(tmpBaseDir, 0o700)
	if err != nil {
		return nil, err
	}

	ctx = utils.WithTmpBaseDir(ctx, tmpBaseDir)

	ga, err := r.buildGitAuth(ctx, secret)
	if err != nil {
		return nil, err
	}

	rc := repocache.NewGitRepoCache(ctx, r.SshPool, ga, nil, 0)
	return rc, nil
}
