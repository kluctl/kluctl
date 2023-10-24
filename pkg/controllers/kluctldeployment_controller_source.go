package controllers

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/gobwas/glob"
	kluctlv1 "github.com/kluctl/kluctl/v2/api/v1beta1"
	"github.com/kluctl/kluctl/v2/pkg/git/auth"
	"github.com/kluctl/kluctl/v2/pkg/git/messages"
	helm_auth "github.com/kluctl/kluctl/v2/pkg/helm/auth"
	"github.com/kluctl/kluctl/v2/pkg/oci/auth_provider"
	"github.com/kluctl/kluctl/v2/pkg/repocache"
	"github.com/kluctl/kluctl/v2/pkg/sourceoverride"
	"github.com/kluctl/kluctl/v2/pkg/status"
	"github.com/kluctl/kluctl/v2/pkg/utils"
	corev1 "k8s.io/api/core/v1"
	"net/url"
	"os"
	"path/filepath"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"strings"
)

type gitRepoSecrets struct {
	deprecatedSecret bool

	host   string
	path   string
	secret corev1.Secret
}

type ociRepoSecrets struct {
	registry string
	repo     string
	secret   corev1.Secret
}

type helmRepoSecrets struct {
	credentialsId string
	host          string
	path          string
	secret        corev1.Secret
}

func (r *KluctlDeploymentReconciler) buildSourceOverridesClients(ctx context.Context, obj *kluctlv1.KluctlDeployment) ([]*sourceoverride.ProxyClientController, error) {
	cleanup := true
	m := map[string]*sourceoverride.ProxyClientController{}

	defer func() {
		if cleanup {
			for _, c := range m {
				_ = c.Close()
				c.Cleanup()
			}
		}
	}()

	for _, x := range obj.Spec.SourceOverrides {
		if _, ok := m[x.Url]; ok {
			continue
		}

		u, err := url.Parse(x.Url)
		if err != nil {
			return nil, fmt.Errorf("failed to parse override url: %w", err)
		}

		if u.Scheme != kluctlv1.SourceOverrideScheme {
			return nil, fmt.Errorf("unsupported source override url scheme: %s", u.Scheme)
		}

		serverId := strings.TrimPrefix(u.Path, "/")

		c, err := sourceoverride.NewClientController(serverId, x.RepoKey, x.IsGroup)
		if err != nil {
			return nil, fmt.Errorf("failed to create source override client: %w", err)
		}

		err = c.Connect(ctx, u.Host)
		if err != nil {
			return nil, fmt.Errorf("failed to create source override client: %w", err)
		}

		m[x.Url] = c
	}
	var l []*sourceoverride.ProxyClientController
	for _, c := range m {
		l = append(l, c)
	}
	cleanup = false
	return l, nil
}

func (r *KluctlDeploymentReconciler) getGitSecrets(ctx context.Context, source kluctlv1.ProjectSource, credentials kluctlv1.ProjectCredentials, objNs string) ([]gitRepoSecrets, error) {
	var ret []gitRepoSecrets

	loadCredentials := func(deprecatedSecret bool, host string, path string, secretName string) error {
		if host == "" {
			host = "*"
		}
		var secret corev1.Secret
		err := r.Client.Get(ctx, client.ObjectKey{Namespace: objNs, Name: secretName}, &secret)
		if err != nil {
			return fmt.Errorf("failed to get secret '%s': %w", secretName, err)
		}
		ret = append(ret, gitRepoSecrets{
			deprecatedSecret: deprecatedSecret,
			host:             host,
			path:             path,
			secret:           secret,
		})
		return nil
	}

	for _, c := range credentials.Git {
		err := loadCredentials(false, c.Host, c.Path, c.SecretRef.Name)
		if err != nil {
			return nil, err
		}
	}
	if source.SecretRef != nil {
		status.Deprecation(ctx, "spec.source.secretRef", "the 'spec.source.secretRef' is deprecated and will be removed in the next API version bump.")
		err := loadCredentials(true, "", "", source.SecretRef.Name)
		if err != nil {
			return nil, err
		}
	}
	if len(source.Credentials) != 0 {
		status.Deprecation(ctx, "spec.source.credentials", "'spec.source.credentials' is deprecated and will be removed in the next API version bump. Use 'spec.credentials.git' instead")
		for _, c := range source.Credentials {
			err := loadCredentials(false, c.Host, c.PathPrefix+"**", c.SecretRef.Name)
			if err != nil {
				return nil, err
			}
		}
	}

	return ret, nil
}

func (r *KluctlDeploymentReconciler) getOciSecrets(ctx context.Context, credentials kluctlv1.ProjectCredentials, objNs string) ([]ociRepoSecrets, error) {
	var ret []ociRepoSecrets

	loadCredentials := func(deprecatedSecret bool, registry string, repo string, secretName string) error {
		var secret corev1.Secret
		err := r.Client.Get(ctx, client.ObjectKey{Namespace: objNs, Name: secretName}, &secret)
		if err != nil {
			return fmt.Errorf("failed to get secret '%s': %w", secretName, err)
		}
		ret = append(ret, ociRepoSecrets{
			registry: registry,
			repo:     repo,
			secret:   secret,
		})
		return nil
	}

	for _, c := range credentials.Oci {
		err := loadCredentials(false, c.Registry, c.Repository, c.SecretRef.Name)
		if err != nil {
			return nil, err
		}
	}

	return ret, nil
}

func (r *KluctlDeploymentReconciler) getHelmSecrets(ctx context.Context, obj *kluctlv1.KluctlDeployment, objNs string) ([]helmRepoSecrets, error) {
	var ret []helmRepoSecrets

	loadCredentials := func(host string, path string, secretName string) error {
		var secret corev1.Secret
		err := r.Client.Get(ctx, client.ObjectKey{Namespace: objNs, Name: secretName}, &secret)
		if err != nil {
			return fmt.Errorf("failed to get secret '%s': %w", secretName, err)
		}
		ret = append(ret, helmRepoSecrets{
			host:   host,
			path:   path,
			secret: secret,
		})
		return nil
	}

	for _, c := range obj.Spec.Credentials.Helm {
		err := loadCredentials(c.Host, c.Path, c.SecretRef.Name)
		if err != nil {
			return nil, err
		}
	}

	for _, c := range obj.Spec.HelmCredentials {
		err := loadCredentials("", "", c.SecretRef.Name)
		if err != nil {
			return nil, err
		}
		e := &ret[len(ret)-1]
		cid := e.secret.Data["credentialsId"]
		u := e.secret.Data["url"]
		if cid != nil {
			e.credentialsId = string(cid)
		}
		if u != nil {
			u2, err := url.Parse(string(u))
			if err != nil {
				return nil, err
			}
			e.host = u2.Host
			e.path = strings.TrimPrefix(u2.Path, "/")
			e.path = strings.TrimSuffix(e.path, "/")
		}
	}

	return ret, nil
}

func (r *KluctlDeploymentReconciler) buildGitAuth(ctx context.Context, gitSecrets []gitRepoSecrets) (*auth.GitAuthProviders, error) {
	log := ctrl.LoggerFrom(ctx)
	ga := auth.NewDefaultAuthProviders("KLUCTL_GIT", &messages.MessageCallbacks{
		WarningFn: func(s string) {
			log.Info(s)
		},
		TraceFn: func(s string) {
			log.V(1).Info(s)
		},
	})

	if len(gitSecrets) == 0 {
		return ga, nil
	}

	var la auth.ListAuthProvider
	ga.RegisterAuthProvider(&la, false)

	for _, secret := range gitSecrets {
		e := auth.AuthEntry{
			AllowWildcardHostForHttp: secret.deprecatedSecret,

			Host:     secret.host,
			Username: "*",
		}
		if secret.path != "" {
			g, err := glob.Compile(secret.path, '/')
			if err != nil {
				return nil, err
			}
			e.PathStr = secret.path
			e.PathGlob = g
		}

		if x, ok := secret.secret.Data["username"]; ok {
			e.Username = string(x)
		}
		if x, ok := secret.secret.Data["password"]; ok {
			e.Password = string(x)
		}
		if x, ok := secret.secret.Data["caFile"]; ok {
			e.CABundle = x
		}
		if x, ok := secret.secret.Data["known_hosts"]; ok {
			e.KnownHosts = x
		}
		if x, ok := secret.secret.Data["identity"]; ok {
			e.SshKey = x
		}

		la.AddEntry(e)
	}
	return ga, nil
}

func (r *KluctlDeploymentReconciler) buildOciAuth(ctx context.Context, ociSecrets []ociRepoSecrets) (*auth_provider.OciAuthProviders, error) {
	ociAuthProvider := auth_provider.NewDefaultAuthProviders("KLUCTL_REGISTRY")

	if len(ociSecrets) == 0 {
		return ociAuthProvider, nil
	}

	var la auth_provider.ListAuthProvider
	ociAuthProvider.RegisterAuthProvider(&la, false)

	for _, secret := range ociSecrets {
		e := auth_provider.AuthEntry{
			Registry: secret.registry,
		}

		if secret.repo != "" {
			g, err := glob.Compile(secret.repo, '/')
			if err != nil {
				return nil, err
			}
			e.RepoStr = secret.repo
			e.RepoGlob = g
		}

		if x, ok := secret.secret.Data["username"]; ok {
			e.AuthConfig.Username = string(x)
		}
		if x, ok := secret.secret.Data["password"]; ok {
			e.AuthConfig.Password = string(x)
		}
		if x, ok := secret.secret.Data["identityToken"]; ok {
			e.AuthConfig.IdentityToken = string(x)
		}
		if x, ok := secret.secret.Data["token"]; ok {
			e.AuthConfig.RegistryToken = string(x)
		}
		if x, ok := secret.secret.Data["plainHttp"]; ok {
			e.PlainHTTP = utils.ParseBoolOrFalse(string(x))
		}
		if x, ok := secret.secret.Data["cert"]; ok {
			e.Cert = x
		} else if x, ok := secret.secret.Data["certFile"]; ok {
			e.Cert = x
		}
		if x, ok := secret.secret.Data["key"]; ok {
			e.Key = x
		} else if x, ok := secret.secret.Data["keyFile"]; ok {
			e.Key = x
		}
		if x, ok := secret.secret.Data["ca"]; ok {
			e.CA = x
		} else if x, ok := secret.secret.Data["caFile"]; ok {
			e.CA = x
		}
		if x, ok := secret.secret.Data["insecureSkipTlsVerify"]; ok {
			e.InsecureSkipTlsVerify = utils.ParseBoolOrFalse(string(x))
		}

		la.AddEntry(e)
	}
	return ociAuthProvider, nil
}

func (r *KluctlDeploymentReconciler) buildHelmAuth(ctx context.Context, helmSecrets []helmRepoSecrets) (helm_auth.HelmAuthProvider, error) {
	authProvider := helm_auth.NewDefaultAuthProviders("KLUCTL_HELM")

	var la helm_auth.ListAuthProvider
	authProvider.RegisterAuthProvider(&la, false)

	for _, secret := range helmSecrets {
		e := helm_auth.AuthEntry{
			CredentialsId: secret.credentialsId,
			Host:          secret.host,
		}

		if secret.path != "" {
			g, err := glob.Compile(secret.path, '/')
			if err != nil {
				return nil, err
			}
			e.PathStr = secret.path
			e.PathGlob = g
		}

		if x, ok := secret.secret.Data["username"]; ok {
			e.Username = string(x)
		}
		if x, ok := secret.secret.Data["password"]; ok {
			e.Password = string(x)
		}
		if x, ok := secret.secret.Data["cert"]; ok {
			e.Cert = x
		} else if x, ok := secret.secret.Data["certFile"]; ok {
			e.Cert = x
		}
		if x, ok := secret.secret.Data["key"]; ok {
			e.Key = x
		} else if x, ok := secret.secret.Data["keyFile"]; ok {
			e.Key = x
		}
		if x, ok := secret.secret.Data["ca"]; ok {
			e.CA = x
		} else if x, ok := secret.secret.Data["caFile"]; ok {
			e.CA = x
		}
		if x, ok := secret.secret.Data["insecure"]; ok {
			e.InsecureSkipTLSverify = utils.ParseBoolOrFalse(string(x))
		} else if x, ok := secret.secret.Data["insecureSkipTlsVerify"]; ok {
			e.InsecureSkipTLSverify = utils.ParseBoolOrFalse(string(x))
		}
		if x, ok := secret.secret.Data["passCredentialsAll"]; ok {
			e.PassCredentialsAll = utils.ParseBoolOrFalse(string(x))
		}

		la.AddEntry(e)
	}
	return authProvider, nil
}

func (r *KluctlDeploymentReconciler) buildGitRepoCacheKey(secrets []gitRepoSecrets) (string, error) {
	// make sure we use a unique repo cache per set of credentials
	h := sha256.New()
	if len(secrets) == 0 {
		h.Write([]byte("no-secret"))
	} else {
		m := json.NewEncoder(h)
		for _, secret := range secrets {
			err := m.Encode(map[string]any{
				"host": secret.host,
				"path": secret.path,
				"data": secret.secret.Data,
			})
			if err != nil {
				return "", err
			}
		}
	}
	h2 := hex.EncodeToString(h.Sum(nil))
	return h2, nil
}

func (r *KluctlDeploymentReconciler) buildOciRepoCacheKey(secrets []ociRepoSecrets) (string, error) {
	// make sure we use a unique repo cache per set of credentials
	h := sha256.New()
	if len(secrets) == 0 {
		h.Write([]byte("no-secret"))
	} else {
		m := json.NewEncoder(h)
		for _, secret := range secrets {
			err := m.Encode(map[string]any{
				"registry": secret.registry,
				"repo":     secret.repo,
				"data":     secret.secret.Data,
			})
			if err != nil {
				return "", err
			}
		}
	}
	h2 := hex.EncodeToString(h.Sum(nil))
	return h2, nil
}

func (r *KluctlDeploymentReconciler) buildGitRepoCache(ctx context.Context, secrets []gitRepoSecrets, soClient sourceoverride.Resolver) (*repocache.GitRepoCache, error) {
	key, err := r.buildGitRepoCacheKey(secrets)
	if err != nil {
		return nil, err
	}

	tmpBaseDir := filepath.Join(os.TempDir(), "kluctl-controller-git-cache", key)
	err = os.MkdirAll(tmpBaseDir, 0o700)
	if err != nil {
		return nil, err
	}

	ctx = utils.WithTmpBaseDir(ctx, tmpBaseDir)

	ga, err := r.buildGitAuth(ctx, secrets)
	if err != nil {
		return nil, err
	}

	rc := repocache.NewGitRepoCache(ctx, r.SshPool, ga, soClient, 0)
	return rc, nil
}

func (r *KluctlDeploymentReconciler) buildOciRepoCache(ctx context.Context, secrets []ociRepoSecrets, soClient sourceoverride.Resolver) (*repocache.OciRepoCache, *auth_provider.OciAuthProviders, error) {
	key, err := r.buildOciRepoCacheKey(secrets)
	if err != nil {
		return nil, nil, err
	}

	tmpBaseDir := filepath.Join(os.TempDir(), "kluctl-controller-oci-cache", key)
	err = os.MkdirAll(tmpBaseDir, 0o700)
	if err != nil {
		return nil, nil, err
	}

	ctx = utils.WithTmpBaseDir(ctx, tmpBaseDir)

	ociAuthProvider, err := r.buildOciAuth(ctx, secrets)
	if err != nil {
		return nil, nil, err
	}

	rc := repocache.NewOciRepoCache(ctx, ociAuthProvider, soClient, 0)
	return rc, ociAuthProvider, nil
}
