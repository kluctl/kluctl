package registries

import (
	"bufio"
	"bytes"
	"context"
	"crypto/x509"
	"fmt"
	"github.com/docker/distribution/registry/client/auth/challenge"
	"github.com/golang-jwt/jwt/v4"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/remote/transport"
	"github.com/kluctl/kluctl/v2/pkg/status"
	"github.com/kluctl/kluctl/v2/pkg/utils"
	"github.com/kluctl/kluctl/v2/pkg/utils/uo"
	"github.com/rogpeppe/go-internal/lockedfile"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

type noAuthRetryError struct {
	msg string
}

func (e *noAuthRetryError) Error() string {
	return e.msg
}

type transportKeyType int

var transportKey transportKeyType

type RegistryHelper struct {
	ctx context.Context

	authEntries []AuthEntry

	cachedTransports utils.ThreadSafeCache
	cachedAuth       utils.ThreadSafeCache
	cachedResponses  utils.ThreadSafeMultiCache
	authRealms       map[string]bool
	authErrors       map[string]bool
	init             sync.Once
	mutex            sync.Mutex
}

type AuthEntry struct {
	Registry      string
	Username      string
	Password      string
	Auth          string
	CABundle      []byte
	Insecure      bool
	SkipTlsVerify bool
}

func NewRegistryHelper(ctx context.Context) *RegistryHelper {
	return &RegistryHelper{
		ctx: ctx,
	}
}

func (rh *RegistryHelper) ListImageTags(image string) ([]string, error) {
	var nameOpts []name.Option
	repo, err := name.NewRepository(image, nameOpts...)
	if err != nil {
		return nil, err
	}

	t, err := rh.buildTransport(repo.RegistryStr())
	if err != nil {
		return nil, err
	}

	remoteOpts := []remote.Option{
		remote.WithAuthFromKeychain(rh),
		remote.WithTransport(rh),
		remote.WithContext(context.WithValue(rh.ctx, transportKey, t)),
	}

	if rh.isInsecureRegistry(repo.RegistryStr()) {
		nameOpts = append(nameOpts, name.Insecure)
		repo, err = name.NewRepository(image, nameOpts...)
	}

	ret, err := remote.List(repo, remoteOpts...)
	if e, ok := err.(*transport.Error); ok && (e.StatusCode == http.StatusUnauthorized || e.StatusCode == http.StatusForbidden) {
		return nil, fmt.Errorf("failed to authenticate against image registry %s, "+
			"please make sure that you provided credentials, e.g. via 'docker login' or via environment variables: %w", repo.Registry, err)
	}
	if e, ok := err.(*url.Error); ok {
		if _, ok := e.Err.(*noAuthRetryError); ok {
			// we explicitly ignore these errors as we assume that the original auth error is handled by another request
			return nil, nil
		}
	}
	return ret, err
}

func (rh *RegistryHelper) AddAuthEntry(e AuthEntry) {
	rh.authEntries = append(rh.authEntries, e)
}

func (rh *RegistryHelper) isDefaultInsecure() bool {
	defaultInsecure, err := utils.ParseEnvBool("KLUCTL_REGISTRY_DEFAULT_INSECURE", false)
	if err != nil {
		status.Warningf(rh.ctx, "Failed to parse KLUCTL_REGISTRY_DEFAULT_INSECURE: %s", err)
		return false
	}
	return defaultInsecure
}

func (rh *RegistryHelper) isDefaultSkipTlsVerify() bool {
	defaultTlsVerify, err := utils.ParseEnvBool("KLUCTL_REGISTRY_DEFAULT_TLSVERIFY", true)
	if err != nil {
		status.Warningf(rh.ctx, "Failed to parse KLUCTL_REGISTRY_DEFAULT_TLSVERIFY: %s", err)
		return false
	}
	return !defaultTlsVerify
}

func (rh *RegistryHelper) isInsecureRegistry(registry string) bool {
	e := rh.findAuthEntry(registry)
	if e == nil {
		return rh.isDefaultInsecure()
	}
	return e.Insecure
}

func (rh *RegistryHelper) isSkipTlsVerify(registry string) bool {
	e := rh.findAuthEntry(registry)
	if e == nil {
		return rh.isDefaultSkipTlsVerify()
	}
	return e.SkipTlsVerify
}

func (rh *RegistryHelper) ParseAuthEntriesFromEnv() error {
	defaultInsecure := rh.isDefaultInsecure()
	defaultSkipTlsVerify := rh.isDefaultSkipTlsVerify()

	for _, m := range utils.ParseEnvConfigSets("KLUCTL_REGISTRY") {
		e := AuthEntry{
			Registry:      m["HOST"],
			Username:      m["USERNAME"],
			Password:      m["PASSWORD"],
			Auth:          m["AUTH"],
			Insecure:      defaultInsecure,
			SkipTlsVerify: defaultSkipTlsVerify,
		}
		if e.Registry == "" {
			continue
		}
		insecureStr, ok := m["INSECURE"]
		if ok {
			insecure, err := strconv.ParseBool(insecureStr)
			if err != nil {
				return fmt.Errorf("failed to parse INSECURE from env: %w", err)
			}
			e.Insecure = insecure
		}
		tlsverifyStr, ok := m["TLSVERIFY"]
		if ok {
			tlsverify, err := strconv.ParseBool(tlsverifyStr)
			if err != nil {
				return fmt.Errorf("failed to parse TLSVERIFY from env: %w", err)
			}
			e.SkipTlsVerify = !tlsverify
		}

		ca_bundle_path := m["CA_BUNDLE"]
		if ca_bundle_path != "" {
			ca_bundle_path = utils.ExpandPath(ca_bundle_path)
			b, err := os.ReadFile(ca_bundle_path)
			if err != nil {
				return fmt.Errorf("failed to read ca bundle %s: %w", ca_bundle_path, err)
			} else {
				e.CABundle = b
			}
		}

		rh.AddAuthEntry(e)
	}
	return nil
}

func (rh *RegistryHelper) findAuthEntry(registry string) *AuthEntry {
	for _, e := range rh.authEntries {
		if e.Registry == "*" || e.Registry == registry {
			return &e
		}
	}
	return nil
}

func (rh *RegistryHelper) loadCA(registry string) (*x509.CertPool, error) {
	e := rh.findAuthEntry(registry)
	if e == nil || e.CABundle == nil {
		return nil, nil
	}

	p := x509.NewCertPool()
	if !p.AppendCertsFromPEM(e.CABundle) {
		return nil, fmt.Errorf("failed to load CA for %s", registry)
	}

	return p, nil
}

func (rh *RegistryHelper) buildTransport(registry string) (http.RoundTripper, error) {
	ret, err := rh.cachedTransports.Get(registry, func() (interface{}, error) {
		skipTls := rh.isSkipTlsVerify(registry)

		ca, err := rh.loadCA(registry)
		if err != nil {
			return nil, err
		}
		if ca == nil && !skipTls {
			return remote.DefaultTransport, nil
		}

		httpTransport, ok := remote.DefaultTransport.(*http.Transport)
		if !ok {
			return nil, fmt.Errorf("remote.DefaultTransport is not a http.Transport anymore. Please report this to https://github.com/kluctl/kluctl")
		}
		t := httpTransport.Clone()

		t.TLSClientConfig.RootCAs = ca
		t.TLSClientConfig.InsecureSkipVerify = skipTls
		return t, nil
	})
	if err != nil {
		return nil, err
	}
	return ret.(http.RoundTripper), nil
}

func (rh *RegistryHelper) getTransport(req *http.Request) http.RoundTripper {
	t := req.Context().Value(transportKey).(http.RoundTripper)
	return t
}

func (rh *RegistryHelper) doResolve(resource authn.Resource) (authn.Authenticator, error) {
	e := rh.findAuthEntry(resource.RegistryStr())
	if e != nil {
		// only use credentials from entry if present, otherwise fallback to default keychain
		// (which will also check ~/.docker/config.json)
		if e.Username != "" || e.Auth != "" {
			return authn.FromConfig(authn.AuthConfig{
				Username: e.Username,
				Password: e.Password,
				Auth:     e.Auth,
			}), nil
		}
	}

	return authn.DefaultKeychain.Resolve(resource)
}

func (rh *RegistryHelper) Resolve(resource authn.Resource) (authn.Authenticator, error) {
	registry := resource.RegistryStr()

	ret, err := rh.cachedAuth.Get(registry, func() (interface{}, error) {
		return rh.doResolve(resource)
	})
	if err != nil {
		return nil, err
	}
	return ret.(authn.Authenticator), nil
}

func (rh *RegistryHelper) realmFromRequest(req *http.Request) string {
	return fmt.Sprintf("%s://%s%s", req.URL.Scheme, req.URL.Host, req.URL.Path)
}

func (rh *RegistryHelper) getCachePath(key string) string {
	return filepath.Join(utils.GetTmpBaseDir(rh.ctx), "registries-cache", key[0:2], key[2:4], key)
}

func (rh *RegistryHelper) checkInvalidToken(resBody []byte) bool {
	j, err := uo.FromString(string(resBody))
	if err != nil {
		return false
	}

	tokenStr, ok, _ := j.GetNestedString("token")
	if !ok {
		return false
	}

	_, err = jwt.Parse(tokenStr, func(token *jwt.Token) (interface{}, error) {
		return nil, nil
	})
	if err != nil {
		if vErr, ok := err.(*jwt.ValidationError); ok {
			if vErr.Errors & ^jwt.ValidationErrorSignatureInvalid == 0 {
				// invalid signature errors are expected as we did not provide a key
				return false
			}
		}
		return true
	}
	return false
}

func (rh *RegistryHelper) readCachedResponse(key string) []byte {
	cachePath := rh.getCachePath(key)
	st, err := os.Stat(cachePath)

	if err != nil {
		return nil
	}

	if time.Now().Sub(st.ModTime()) > 55*time.Minute {
		return nil
	}

	f, err := lockedfile.OpenFile(cachePath, os.O_RDONLY, 0)
	if err != nil {
		status.Warningf(rh.ctx, "readCachedResponse failed: %v", err)
		return nil
	}
	b, err := io.ReadAll(f)
	_ = f.Close()
	if err != nil {
		return nil
	}

	res, err := http.ReadResponse(bufio.NewReader(bytes.NewReader(b)), nil)
	if err != nil {
		status.Warningf(rh.ctx, "readCachedResponse failed: %v", err)
		return nil
	}

	if strings.HasPrefix(res.Header.Get("Content-Type"), "application/json") {
		jb, err := io.ReadAll(res.Body)
		if err != nil {
			return nil
		}
		if rh.checkInvalidToken(jb) {
			return nil
		}
	}

	return b
}

func (rh *RegistryHelper) writeCachedResponse(key string, data []byte) {
	cachePath := rh.getCachePath(key)
	cacheDir := filepath.Dir(cachePath)
	if !utils.Exists(cacheDir) {
		err := os.MkdirAll(cacheDir, 0o700)
		if err != nil {
			status.Warningf(rh.ctx, "writeCachedResponse failed: %v", err)
			return
		}
	}

	f, err := lockedfile.OpenFile(cachePath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		status.Warningf(rh.ctx, "writeCachedResponse failed: %v", err)
		return
	}
	defer f.Close()

	_, err = f.Write(data)
	if err != nil {
		status.Warningf(rh.ctx, "writeCachedResponse failed: %v", err)
		return
	}
}

func (rh *RegistryHelper) RoundTripCached(req *http.Request, extraKey string, onNew func(res *http.Response) error) (*http.Response, error) {
	key := fmt.Sprintf("%s\n%s\n%s\n%s\n%s\n", req.URL.Scheme, req.URL.Host, req.Host, req.URL.Path, extraKey)
	key = utils.Sha256String(key)

	isNew := false
	resI, err := rh.cachedResponses.Get(req.Host, key, func() (interface{}, error) {
		isNew = true

		b := rh.readCachedResponse(key)
		if b == nil {
			res, err := rh.getTransport(req).RoundTrip(req)
			if err != nil {
				return nil, err
			}
			b, err = httputil.DumpResponse(res, true)
			if err != nil {
				return nil, err
			}

			if res.StatusCode < 500 {
				rh.writeCachedResponse(key, b)
			}
		}

		return b, nil
	})
	if err != nil {
		return nil, err
	}
	resBytes, _ := resI.([]byte)

	res, err := http.ReadResponse(bufio.NewReader(bytes.NewReader(resBytes)), req)
	if err != nil {
		return res, err
	}

	if isNew && onNew != nil {
		err = onNew(res)
		if err != nil {
			return nil, err
		}
		res, _ = http.ReadResponse(bufio.NewReader(bytes.NewReader(resBytes)), req)
	}

	return res, err
}

func (rh *RegistryHelper) RoundTripInfoReq(req *http.Request) (*http.Response, error) {
	return rh.RoundTripCached(req, "info", func(res *http.Response) error {
		rh.mutex.Lock()
		defer rh.mutex.Unlock()

		chgs := challenge.ResponseChallenges(res)
		for _, chg := range chgs {
			if realm, ok := chg.Parameters["realm"]; ok {
				rh.authRealms[realm] = true
			}
		}
		return nil
	})
}

func (rh *RegistryHelper) RoundTripAuth(req *http.Request) (*http.Response, error) {
	b := bytes.NewBuffer(nil)
	err := req.Header.Write(b)
	if err != nil {
		return nil, err
	}

	b.WriteString("\n" + req.URL.RawQuery)

	hash := utils.Sha256String(b.String())

	return rh.RoundTripCached(req, hash, func(res *http.Response) error {
		rh.mutex.Lock()
		defer rh.mutex.Unlock()

		if res.StatusCode == http.StatusUnauthorized || res.StatusCode == http.StatusForbidden {
			// if auth fails once for a registry, we must not retry any auth on that registry as we could easily run
			// into an IP block
			rh.authErrors[rh.realmFromRequest(req)] = true
		}

		return nil
	})
}

func (rh *RegistryHelper) RoundTrip(req *http.Request) (*http.Response, error) {
	rh.init.Do(func() {
		rh.authRealms = make(map[string]bool)
		rh.authErrors = make(map[string]bool)
	})

	if req.URL.Path == "/v2/" {
		return rh.RoundTripInfoReq(req)
	}

	rh.mutex.Lock()
	realm := rh.realmFromRequest(req)
	_, isAuthRealm := rh.authRealms[realm]
	_, isAuthError := rh.authErrors[realm]
	rh.mutex.Unlock()

	if isAuthError {
		return nil, &noAuthRetryError{fmt.Sprintf("previous auth request for %s gave an error, we won't retry", realm)}
	}

	if isAuthRealm {
		return rh.RoundTripAuth(req)
	}

	return rh.getTransport(req).RoundTrip(req)
}
