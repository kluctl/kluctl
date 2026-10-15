package kluctl_project

import (
	"context"
	"fmt"
	"github.com/getsops/sops/v3/cmd/sops/formats"
	"github.com/kluctl/kluctl/lib/status"
	"github.com/kluctl/kluctl/v2/pkg/sops"
	"github.com/kluctl/kluctl/v2/pkg/sops/decryptor"
	"github.com/kluctl/kluctl/v2/pkg/utils"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"
	"os"
	"path/filepath"
)

func (p *LoadedKluctlProject) LoadK8sConfig(ctx context.Context, targetName string, contextOverride string, offlineK8s bool) (*rest.Config, string, error) {
	if offlineK8s {
		return nil, "", nil
	}

	var contextName *string
	var kubeconfigPath string
	if targetName != "" {
		t, err := p.FindTarget(targetName)
		if err != nil {
			return nil, "", err
		}
		contextName = t.Context
		if t.Kubeconfig != nil {
			kubeconfigPath = *t.Kubeconfig
		}
	}
	if contextOverride != "" {
		contextName = &contextOverride
	}

	if kubeconfigPath != "" {
		return p.loadKubeconfigFromTarget(ctx, kubeconfigPath, contextName)
	}

	var err error
	var clientConfig *rest.Config
	var restConfig *api.Config
	clientConfig, restConfig, err = p.LoadArgs.ClientConfigGetter(contextName)
	if err != nil {
		if contextName == nil && clientcmd.IsEmptyConfig(err) {
			status.Warning(ctx, "No valid KUBECONFIG provided, which means the Kubernetes client is not available. Depending on your deployment project, this might cause follow-up errors.")
			return nil, "", nil
		}
		return nil, "", err
	}
	contextName = &restConfig.CurrentContext
	return clientConfig, *contextName, nil
}

func (p *LoadedKluctlProject) loadKubeconfigFromTarget(ctx context.Context, kubeconfigPath string, contextName *string) (*rest.Config, string, error) {
	path := filepath.Join(p.LoadArgs.ProjectDir, kubeconfigPath)
	err := utils.CheckInDir(p.LoadArgs.ProjectDir, path)
	if err != nil {
		return nil, "", fmt.Errorf("kubeconfig %s is not inside project directory: %w", kubeconfigPath, err)
	}

	if !utils.IsFile(path) {
		return nil, "", fmt.Errorf("kubeconfig %s does not exist", kubeconfigPath)
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return nil, "", fmt.Errorf("failed to read kubeconfig %s: %w", path, err)
	}

	// Build a SOPS decryptor for the project root. Reuse any configured key servers
	// (e.g. for age/PGP) via AddKeyServersFunc, otherwise fall back to the local key service.
	d := decryptor.NewDecryptor(p.LoadArgs.RepoRoot, decryptor.MaxEncryptedFileSize)
	if p.LoadArgs.AddKeyServersFunc != nil {
		if err := p.LoadArgs.AddKeyServersFunc(ctx, d); err != nil {
			return nil, "", fmt.Errorf("failed to configure SOPS key services for kubeconfig %s: %w", path, err)
		}
	} else {
		d.AddLocalKeyService()
	}

	format := formats.FormatForPath(path)
	decrypted, _, err := sops.MaybeDecrypt(d, content, format, format)
	if err != nil {
		return nil, "", fmt.Errorf("failed to decrypt kubeconfig %s: %w", path, err)
	}

	rawConfig, err := clientcmd.Load(decrypted)
	if err != nil {
		return nil, "", fmt.Errorf("failed to load kubeconfig %s: %w", path, err)
	}

	overrides := &clientcmd.ConfigOverrides{}
	if contextName != nil {
		overrides.CurrentContext = *contextName
	}

	clientConfig := clientcmd.NewDefaultClientConfig(*rawConfig, overrides)
	restConfig, err := clientConfig.ClientConfig()
	if err != nil {
		return nil, "", fmt.Errorf("failed to create rest config from kubeconfig %s: %w", path, err)
	}

	raw, err := clientConfig.RawConfig()
	if err != nil {
		return nil, "", err
	}

	return restConfig, raw.CurrentContext, nil
}
