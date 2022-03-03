package registries

import (
	"bufio"
	"bytes"
	"fmt"
	"github.com/codablock/kluctl/pkg/utils"
	"github.com/docker/distribution/registry/client/auth/challenge"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"net/http"
	"net/http/httputil"
	"os"
	"strconv"
	"sync"
)

func ListImageTags(image string) ([]string, error) {
	var nameOpts []name.Option
	remoteOpts := []remote.Option{
		remote.WithAuthFromKeychain(&globalMyKeychain),
		remote.WithTransport(&globalMyKeychain),
	}

	repo, err := name.NewRepository(image, nameOpts...)
	if err != nil {
		return nil, err
	}

	if isRegistryInsecure(repo.RegistryStr()) {
		nameOpts = append(nameOpts, name.Insecure)
		repo, err = name.NewRepository(image, nameOpts...)
	}

	return remote.List(repo, remoteOpts...)
}

var globalMyKeychain myKeychain

type myKeychain struct {
	cachedResponses utils.ThreadSafeMultiCache
	authRealms      map[string]bool
	authErrors      map[string]bool
	init            sync.Once
	mutex           sync.Mutex
}

func (kc *myKeychain) Resolve(resource authn.Resource) (authn.Authenticator, error) {
	registry := resource.RegistryStr()

	for _, m := range utils.ParseEnvConfigSets("KLUCTL_REGISTRY") {
		host, _ := m["HOST"]
		if host == registry {
			username, _ := m["USERNAME"]
			password, _ := m["PASSWORD"]
			return authn.FromConfig(authn.AuthConfig{
				Username: username,
				Password: password,
			}), nil
		}
	}

	return authn.DefaultKeychain.Resolve(resource)
}

func (kc *myKeychain) realmFromRequest(req *http.Request) string {
	return fmt.Sprintf("%s://%s%s", req.URL.Scheme, req.URL.Host, req.URL.Path)
}

func (kc *myKeychain) RoundTripCached(req *http.Request, extraKey string, onNew func(res *http.Response) error) (*http.Response, error) {
	resI, err := kc.cachedResponses.Get(req.Host, req.URL.Path+"#"+extraKey, func() (interface{}, error) {
		res, err := remote.DefaultTransport.RoundTrip(req)
		if err != nil {
			return nil, err
		}

		if onNew != nil {
			err = onNew(res)
			if err != nil {
				return nil, err
			}
		}

		return httputil.DumpResponse(res, true)
	})

	if err != nil {
		return nil, err
	}
	resBytes, _ := resI.([]byte)
	r := bufio.NewReader(bytes.NewReader(resBytes))

	res, err := http.ReadResponse(r, req)
	if err != nil {
		return res, err
	}

	return res, err
}

func (kc *myKeychain) RoundTripInfoReq(req *http.Request) (*http.Response, error) {
	return kc.RoundTripCached(req, "info", func(res *http.Response) error {
		kc.mutex.Lock()
		defer kc.mutex.Unlock()

		chgs := challenge.ResponseChallenges(res)
		for _, chg := range chgs {
			if realm, ok := chg.Parameters["realm"]; ok {
				kc.authRealms[realm] = true
			}
		}
		return nil
	})
}

func (kc *myKeychain) RoundTripAuth(req *http.Request) (*http.Response, error) {
	b := bytes.NewBuffer(nil)
	err := req.Header.Write(b)
	if err != nil {
		return nil, err
	}

	b.WriteString("\n" + req.URL.RawQuery)

	hash := utils.Sha256String(b.String())

	return kc.RoundTripCached(req, hash, func(res *http.Response) error {
		kc.mutex.Lock()
		defer kc.mutex.Unlock()

		if res.StatusCode == http.StatusUnauthorized || res.StatusCode == http.StatusForbidden {
			// if auth fails once for a registry, we must not retry any auth on that registry as we could easily run
			// into an IP block
			kc.authErrors[kc.realmFromRequest(req)] = true
		}

		return nil
	})
}

func (kc *myKeychain) RoundTrip(req *http.Request) (*http.Response, error) {
	kc.init.Do(func() {
		kc.authRealms = make(map[string]bool)
		kc.authErrors = make(map[string]bool)
	})

	if req.URL.Path == "/v2/" {
		return kc.RoundTripInfoReq(req)
	}

	kc.mutex.Lock()
	realm := kc.realmFromRequest(req)
	_, isAuthRealm := kc.authRealms[realm]
	_, isAuthError := kc.authErrors[realm]
	kc.mutex.Unlock()

	if isAuthError {
		return nil, fmt.Errorf("previous auth request for %s gave an error, we won't retry", realm)
	}

	if isAuthRealm {
		return kc.RoundTripAuth(req)
	}

	resp, err := remote.DefaultTransport.RoundTrip(req)
	return resp, err
}

func isRegistryInsecure(registry string) bool {
	for _, m := range utils.ParseEnvConfigSets("KLUCTL_REGISTRY") {
		host, _ := m["HOST"]
		if host == registry {
			tlsverifyStr, ok := m["TLSVERIFY"]
			if ok {
				tlsverify, err := strconv.ParseBool(tlsverifyStr)
				if err == nil {
					return !tlsverify
				}
			}
		}
	}

	if x, ok := os.LookupEnv("KLUCTL_REGISTRY_DEFAULT_TLSVERIFY"); ok {
		if b, err := strconv.ParseBool(x); err == nil {
			return !b
		}
	}
	return false
}
