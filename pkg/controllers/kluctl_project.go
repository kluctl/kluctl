package controllers

import (
	"context"
	"fmt"
	"github.com/getsops/sops/v3/keyservice"
	"github.com/getsops/sops/v3/kms"
	"github.com/google/uuid"
	"github.com/kluctl/go-jinja2"
	"github.com/kluctl/kluctl/v2/lib/git"
	"github.com/kluctl/kluctl/v2/pkg/clouds/aws"
	internal_metrics "github.com/kluctl/kluctl/v2/pkg/controllers/metrics"
	helm_auth "github.com/kluctl/kluctl/v2/pkg/helm/auth"
	"github.com/kluctl/kluctl/v2/pkg/kluctl_jinja2"
	"github.com/kluctl/kluctl/v2/pkg/kluctl_project/target-context"
	"github.com/kluctl/kluctl/v2/pkg/oci/auth_provider"
	"github.com/kluctl/kluctl/v2/pkg/repocache"
	"github.com/kluctl/kluctl/v2/pkg/sops"
	"github.com/kluctl/kluctl/v2/pkg/sops/decryptor"
	intkeyservice "github.com/kluctl/kluctl/v2/pkg/sops/keyservice"
	"github.com/kluctl/kluctl/v2/pkg/sourceoverride"
	types2 "github.com/kluctl/kluctl/v2/pkg/types"
	"github.com/kluctl/kluctl/v2/pkg/types/result"
	"github.com/kluctl/kluctl/v2/pkg/utils/uo"
	"github.com/prometheus/client_golang/prometheus"
	"helm.sh/helm/v3/pkg/repo"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"os"
	"path/filepath"
	"strings"
	"time"

	securejoin "github.com/cyphar/filepath-securejoin"
	kluctlv1 "github.com/kluctl/kluctl/v2/api/v1beta1"
	"github.com/kluctl/kluctl/v2/pkg/deployment"
	"github.com/kluctl/kluctl/v2/pkg/deployment/commands"
	k8s2 "github.com/kluctl/kluctl/v2/pkg/k8s"
	"github.com/kluctl/kluctl/v2/pkg/kluctl_project"
	"github.com/kluctl/kluctl/v2/pkg/types/k8s"
	"github.com/kluctl/kluctl/v2/pkg/utils"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"
	ctrl "sigs.k8s.io/controller-runtime"
)

type preparedProject struct {
	r   *KluctlDeploymentReconciler
	obj *kluctlv1.KluctlDeployment

	startTime time.Time

	j2    *jinja2.Jinja2
	gitRP *repocache.GitRepoCache
	ociRP *repocache.OciRepoCache

	helmAuthProvider helm_auth.HelmAuthProvider
	ociAuthProvider  auth_provider.OciAuthProvider

	co         git.CheckoutInfo
	tmpDir     string
	repoDir    string
	projectDir string

	soClients []*sourceoverride.ProxyClientController
}

type preparedTarget struct {
	pp *preparedProject
}

func prepareProject(ctx context.Context,
	r *KluctlDeploymentReconciler,
	obj *kluctlv1.KluctlDeployment,
	doCloneSource bool) (*preparedProject, error) {

	cleanup := true

	pp := &preparedProject{
		r:         r,
		obj:       obj,
		startTime: time.Now(),
	}
	defer func() {
		if !cleanup {
			return
		}
		pp.cleanup()
	}()

	var err error
	pp.j2, err = kluctl_jinja2.NewKluctlJinja2(ctx, true, r.UseSystemPython)
	if err != nil {
		return nil, err
	}

	// create tmp dir
	pp.tmpDir, err = os.MkdirTemp("", obj.GetName()+"-")
	if err != nil {
		return pp, fmt.Errorf("failed to create temp dir for kluctl project: %w", err)
	}

	pp.soClients, err = r.buildSourceOverridesClients(ctx, obj)
	if err != nil {
		return nil, err
	}

	var resolvers []sourceoverride.Resolver
	for _, x := range pp.soClients {
		resolvers = append(resolvers, x)
	}
	resolver := sourceoverride.NewChainedResolver(resolvers)

	gitSecrets, err := r.getGitSecrets(ctx, pp.obj.Spec.Source, pp.obj.Spec.Credentials, obj.GetNamespace())
	if err != nil {
		return nil, err
	}

	ociSecrets, err := r.getOciSecrets(ctx, pp.obj.Spec.Credentials, obj.GetNamespace())
	if err != nil {
		return nil, err
	}

	helmSecrets, err := r.getHelmSecrets(ctx, pp.obj, obj.GetNamespace())
	if err != nil {
		return nil, err
	}

	pp.gitRP, err = r.buildGitRepoCache(ctx, gitSecrets, resolver)
	if err != nil {
		return nil, err
	}

	pp.ociRP, pp.ociAuthProvider, err = r.buildOciRepoCache(ctx, ociSecrets, resolver)
	if err != nil {
		return nil, err
	}

	pp.helmAuthProvider, err = r.buildHelmAuth(ctx, helmSecrets)
	if err != nil {
		return nil, err
	}

	if doCloneSource {
		pth := ""
		if pp.obj.Spec.Source.Git != nil {
			pth = pp.obj.Spec.Source.Git.Path
			rpEntry, err := pp.gitRP.GetEntry(pp.obj.Spec.Source.Git.URL)
			if err != nil {
				return nil, fmt.Errorf("failed to clone git source: %w", err)
			}

			pp.repoDir, pp.co, err = rpEntry.GetClonedDir(pp.obj.Spec.Source.Git.Ref)
			if err != nil {
				return nil, err
			}
		} else if pp.obj.Spec.Source.Oci != nil {
			pth = pp.obj.Spec.Source.Oci.Path
			rpEntry, err := pp.ociRP.GetEntry(pp.obj.Spec.Source.Oci.URL)
			if err != nil {
				return nil, fmt.Errorf("failed to pull OCI source: %w", err)
			}

			pp.repoDir, pp.co, err = rpEntry.GetExtractedDir(pp.obj.Spec.Source.Oci.Ref)
			if err != nil {
				return nil, err
			}
		} else if pp.obj.Spec.Source.URL != nil {
			pth = pp.obj.Spec.Source.Path
			rpEntry, err := pp.gitRP.GetEntry(*pp.obj.Spec.Source.URL)
			if err != nil {
				return nil, fmt.Errorf("failed to clone git source: %w", err)
			}

			pp.repoDir, pp.co, err = rpEntry.GetClonedDir(pp.obj.Spec.Source.Ref)
			if err != nil {
				return nil, err
			}
		} else {
			return nil, fmt.Errorf("missing source spec")
		}

		// check kluctl project path exists
		pp.projectDir, err = securejoin.SecureJoin(pp.repoDir, pth)
		if err != nil {
			return pp, err
		}
		if _, err := os.Stat(pp.projectDir); err != nil {
			return pp, fmt.Errorf("kluctlDeployment path not found: %w", err)
		}
	}

	cleanup = false
	return pp, nil
}

func (pp *preparedProject) cleanup() {
	if pp.tmpDir != "" {
		_ = os.RemoveAll(pp.tmpDir)
	}
	if pp.gitRP != nil {
		pp.gitRP.Clear()
		pp.gitRP = nil
	}
	for _, c := range pp.soClients {
		_ = c.Close()
		c.Cleanup()
	}
	if pp.j2 != nil {
		pp.j2.Close()
	}
}

func (pp *preparedProject) newTarget() *preparedTarget {
	pt := preparedTarget{
		pp: pp,
	}

	return &pt
}

func (pt *preparedTarget) restConfigToKubeconfig(restConfig *rest.Config) *api.Config {
	kubeConfig := api.NewConfig()
	cluster := api.NewCluster()
	cluster.Server = restConfig.Host
	cluster.CertificateAuthority = restConfig.TLSClientConfig.CAFile
	cluster.CertificateAuthorityData = restConfig.TLSClientConfig.CAData
	cluster.InsecureSkipTLSVerify = restConfig.TLSClientConfig.Insecure
	kubeConfig.Clusters["default"] = cluster

	user := api.NewAuthInfo()
	user.ClientKey = restConfig.KeyFile
	user.ClientKeyData = restConfig.KeyData
	user.ClientCertificate = restConfig.CertFile
	user.ClientCertificateData = restConfig.CertData
	user.TokenFile = restConfig.BearerTokenFile
	user.Token = restConfig.BearerToken
	user.Impersonate = restConfig.Impersonate.UserName
	user.ImpersonateUID = restConfig.Impersonate.UID
	user.ImpersonateUserExtra = restConfig.Impersonate.Extra
	user.ImpersonateGroups = restConfig.Impersonate.Groups
	user.Username = restConfig.Username
	user.Password = restConfig.Password
	user.AuthProvider = restConfig.AuthProvider
	user.Exec = restConfig.ExecProvider
	kubeConfig.AuthInfos["default"] = user

	kctx := api.NewContext()
	kctx.Cluster = "default"
	kctx.AuthInfo = "default"
	kubeConfig.Contexts["default"] = kctx
	kubeConfig.CurrentContext = "default"

	return kubeConfig
}

func (pt *preparedTarget) getKubeconfigFromSecret(ctx context.Context) ([]byte, error) {
	secretName := types.NamespacedName{
		Namespace: pt.pp.obj.GetNamespace(),
		Name:      pt.pp.obj.Spec.KubeConfig.SecretRef.Name,
	}

	var secret corev1.Secret
	if err := pt.pp.r.Client.Get(ctx, secretName, &secret); err != nil {
		return nil, fmt.Errorf("unable to read KubeConfig secret '%s' error: %w", secretName.String(), err)
	}

	var kubeConfig []byte
	switch {
	case pt.pp.obj.Spec.KubeConfig.SecretRef.Key != "":
		key := pt.pp.obj.Spec.KubeConfig.SecretRef.Key
		kubeConfig = secret.Data[key]
		if kubeConfig == nil {
			return nil, fmt.Errorf("KubeConfig secret '%s' does not contain a '%s' key with a kubeconfig", secretName, key)
		}
	case secret.Data["value"] != nil:
		kubeConfig = secret.Data["value"]
	case secret.Data["value.yaml"] != nil:
		kubeConfig = secret.Data["value.yaml"]
	default:
		// User did not specify a key, and the 'value' key was not defined.
		return nil, fmt.Errorf("KubeConfig secret '%s' does not contain a 'value' key with a kubeconfig", secretName)
	}

	return kubeConfig, nil
}

func (pt *preparedTarget) setImpersonationConfig(restConfig *rest.Config) {
	name := pt.pp.r.DefaultServiceAccount
	if sa := pt.pp.obj.Spec.ServiceAccountName; sa != "" {
		name = sa
	}
	if name != "" {
		username := fmt.Sprintf("system:serviceaccount:%s:%s", pt.pp.obj.GetNamespace(), name)
		restConfig.Impersonate = rest.ImpersonationConfig{UserName: username}
	}
}

func (pt *preparedTarget) buildRestConfig(ctx context.Context) (*rest.Config, error) {
	var restConfig *rest.Config

	if pt.pp.obj.Spec.KubeConfig != nil {
		kubeConfig, err := pt.getKubeconfigFromSecret(ctx)
		if err != nil {
			return nil, err
		}
		restConfig, err = clientcmd.RESTConfigFromKubeConfig(kubeConfig)
		if err != nil {
			return nil, err
		}
	} else {
		restConfig = rest.CopyConfig(pt.pp.r.RestConfig)
	}

	pt.setImpersonationConfig(restConfig)

	return restConfig, nil
}

func (pt *preparedTarget) buildKubeconfig(ctx context.Context) (*api.Config, error) {
	restConfig, err := pt.buildRestConfig(ctx)
	if err != nil {
		return nil, err
	}

	kubeConfig := pt.restConfigToKubeconfig(restConfig)
	return kubeConfig, nil
}

/*
func (pt *preparedTarget) getRegistrySecrets(ctx context.Context) ([]*corev1.Secret, error) {
	var ret []*corev1.Secret
	for _, ref := range pt.pp.obj.Spec.RegistrySecrets {
		name := types.NamespacedName{
			Namespace: pt.pp.obj.GetNamespace(),
			Name:      ref.Name,
		}
		var secret corev1.Secret
		if err := pt.pp.r.Get(ctx, name, &secret); err != nil {
			return nil, fmt.Errorf("failed to get secret '%s': %w", name.String(), err)
		}
		ret = append(ret, &secret)
	}
	return ret, nil
}

func (pt *preparedTarget) buildRegistryHelper(ctx context.Context) (*registries.RegistryHelper, error) {
	secrets, err := pt.getRegistrySecrets(ctx)
	if err != nil {
		return nil, err
	}

	rh := registries.NewRegistryHelper(ctx)
	err = rh.ParseAuthEntriesFromEnv()
	if err != nil {
		return nil, err
	}

	for _, s := range secrets {
		caFile := s.Data["caFile"]
		insecure := false
		if x, ok := s.Data["insecure"]; ok {
			insecure, err = strconv.ParseBool(string(x))
			if err != nil {
				return nil, fmt.Errorf("failed parsing insecure flag from secret %s: %w", s.Name, err)
			}
		}

		if dockerConfig, ok := s.Data[".dockerconfigjson"]; ok {
			maxFields := 1
			if _, ok := s.Data["caFile"]; ok {
				maxFields++
			}
			if _, ok := s.Data["insecure"]; ok {
				maxFields++
			}
			if len(s.Data) != maxFields {
				return nil, fmt.Errorf("when using .dockerconfigjson in registry secret, only caFile and insecure fields are allowed additionally")
			}

			c := configfile.New(".dockerconfigjson")
			err = c.LoadFromReader(bytes.NewReader(dockerConfig))
			if err != nil {
				return nil, fmt.Errorf("failed to parse .dockerconfigjson from secret %s: %w", s.Name, err)
			}
			for registry, ac := range c.GetAuthConfigs() {
				var e registries.AuthEntry
				e.Registry = registry
				e.Username = ac.Username
				e.Password = ac.Password
				e.Auth = ac.Auth
				e.CABundle = caFile
				e.Insecure = insecure

				rh.AddAuthEntry(e)
			}
		} else {
			var e registries.AuthEntry
			e.Registry = string(s.Data["registry"])
			e.Username = string(s.Data["username"])
			e.Password = string(s.Data["password"])
			e.Auth = string(s.Data["auth"])
			e.CABundle = caFile
			e.Insecure = insecure

			if e.Registry == "" || (e.Username == "" && e.Auth == "") {
				return nil, fmt.Errorf("registry secret is incomplete")
			}
			rh.AddAuthEntry(e)
		}
	}
	return rh, nil
}
*/

func (pt *preparedTarget) buildImages(ctx context.Context) (*deployment.Images, error) {
	/*rh, err := pt.buildRegistryHelper(ctx)
	if err != nil {
		return nil, err
	}
	offline := !pt.pp.obj.Spec.UpdateImages
	images, err := deployment.NewImages(rh, pt.pp.obj.Spec.UpdateImages, offline)*/

	images, err := deployment.NewImages()
	if err != nil {
		return nil, err
	}
	for _, fi := range pt.pp.obj.Spec.Images {
		images.AddFixedImage(fi)
	}
	return images, nil
}

type helmCredentialsProvider []repo.Entry

func (p helmCredentialsProvider) FindCredentials(repoUrl string, credentialsId *string) *repo.Entry {
	if credentialsId != nil {
		for _, e := range p {
			if e.Name != "" && e.Name == *credentialsId {
				return &e
			}
		}
	}
	if repoUrl == "" {
		return nil
	}
	for _, e := range p {
		if e.URL == repoUrl {
			return &e
		}
	}
	return nil
}

func (pt *preparedTarget) buildInclusion() *utils.Inclusion {
	inc := utils.NewInclusion()
	for _, x := range pt.pp.obj.Spec.IncludeTags {
		inc.AddInclude("tag", x)
	}
	for _, x := range pt.pp.obj.Spec.ExcludeTags {
		inc.AddExclude("tag", x)
	}
	for _, x := range pt.pp.obj.Spec.IncludeDeploymentDirs {
		inc.AddInclude("deploymentItemDir", x)
	}
	for _, x := range pt.pp.obj.Spec.ExcludeDeploymentDirs {
		inc.AddExclude("deploymentItemDir", x)
	}
	return inc
}

func (pt *preparedTarget) clientConfigGetter(ctx context.Context) func(context *string) (*rest.Config, *api.Config, error) {
	return func(context *string) (*rest.Config, *api.Config, error) {
		kubeConfig, err := pt.buildKubeconfig(ctx)
		if err != nil {
			return nil, nil, err
		}

		configOverrides := &clientcmd.ConfigOverrides{}
		if context != nil {
			configOverrides.CurrentContext = *context
		}
		clientConfig := clientcmd.NewDefaultClientConfig(*kubeConfig, configOverrides)
		rawConfig, err := clientConfig.RawConfig()
		if err != nil {
			return nil, nil, err
		}
		if context != nil {
			rawConfig.CurrentContext = *context
		}
		restConfig, err := clientConfig.ClientConfig()
		if err != nil {
			return nil, nil, err
		}
		return restConfig, &rawConfig, nil
	}
}

func (pp *preparedProject) addKeyServers(ctx context.Context, d *decryptor.Decryptor) error {
	if pp.obj.Spec.Decryption == nil {
		return nil
	}
	if pp.obj.Spec.Decryption.Provider != "sops" {
		return fmt.Errorf("not supported decryption provider %s", pp.obj.Spec.Decryption.Provider)
	}

	err := pp.addSecretBasedKeyServers(ctx, d)
	if err != nil {
		return err
	}
	err = pp.addServiceAccountBasedKeyServers(ctx, d)
	if err != nil {
		return err
	}
	return nil
}

func (pp *preparedProject) addSecretBasedKeyServers(ctx context.Context, d *decryptor.Decryptor) error {
	if pp.obj.Spec.Decryption.SecretRef == nil {
		return nil
	}

	secretName := types.NamespacedName{
		Namespace: pp.obj.GetNamespace(),
		Name:      pp.obj.Spec.Decryption.SecretRef.Name,
	}

	var secret corev1.Secret
	if err := pp.r.Client.Get(ctx, secretName, &secret); err != nil {
		if apierrors.IsNotFound(err) {
			return err
		}
		return fmt.Errorf("cannot get decryption Secret '%s': %w", secretName, err)
	}

	gnuPGHome := filepath.Join(pp.tmpDir, "sops-gnupghome")
	err := os.MkdirAll(gnuPGHome, 0o700)
	if err != nil {
		return err
	}

	ks, err := sops.BuildSopsKeyServerFromSecret(&secret, gnuPGHome)
	if err != nil {
		return err
	}
	if ks != nil {
		d.AddKeyServiceClient(ks)
	}
	return nil
}

func (pp *preparedProject) addServiceAccountBasedKeyServers(ctx context.Context, d *decryptor.Decryptor) error {
	name := pp.r.DefaultServiceAccount
	if pp.obj.Spec.Decryption != nil && pp.obj.Spec.Decryption.ServiceAccount != "" {
		name = pp.obj.Spec.Decryption.ServiceAccount
	} else if sa := pp.obj.Spec.ServiceAccountName; sa != "" {
		name = sa
	}
	if name == "" {
		return nil
	}

	creds, err := aws.BuildCredentialsFromServiceAccount(ctx, pp.r.Client, name, pp.obj.Namespace, "kluctl-controller")
	if err != nil || creds == nil {
		return err
	}

	server := intkeyservice.NewServer(intkeyservice.WithAWSKeys{CredsProvider: kms.NewCredentialsProvider(creds)})
	ks := keyservice.NewCustomLocalClient(server)
	d.AddKeyServiceClient(ks)
	return nil
}

func (pp *preparedProject) loadKluctlProject(ctx context.Context, pt *preparedTarget) (*kluctl_project.LoadedKluctlProject, error) {
	var err error

	var externalArgs *uo.UnstructuredObject
	if pt.pp.obj.Spec.Args != nil {
		externalArgs, err = uo.FromString(string(pt.pp.obj.Spec.Args.Raw))
		if err != nil {
			return nil, err
		}
	}

	loadArgs := kluctl_project.LoadKluctlProjectArgs{
		RepoRoot:     pp.repoDir,
		ExternalArgs: externalArgs,
		ProjectDir:   pp.projectDir,
		GitRP:        pp.gitRP,
		OciRP:        pp.ociRP,
		AddKeyServersFunc: func(ctx context.Context, d *decryptor.Decryptor) error {
			return pp.addKeyServers(ctx, d)
		},
	}
	if pt != nil {
		loadArgs.ClientConfigGetter = pt.clientConfigGetter(ctx)
	}

	p, err := kluctl_project.LoadKluctlProject(ctx, loadArgs, pp.j2)
	if err != nil {
		return nil, err
	}

	return p, nil
}

func (pt *preparedTarget) loadTarget(ctx context.Context, p *kluctl_project.LoadedKluctlProject) (*target_context.TargetContext, error) {
	renderOutputDir, err := os.MkdirTemp(pt.pp.tmpDir, "render-")
	if err != nil {
		return nil, err
	}
	images, err := pt.buildImages(ctx)
	if err != nil {
		return nil, err
	}

	inclusion := pt.buildInclusion()

	props := target_context.TargetContextParams{
		DryRun:           pt.pp.r.DryRun || pt.pp.obj.Spec.DryRun,
		Images:           images,
		Inclusion:        inclusion,
		HelmAuthProvider: pt.pp.helmAuthProvider,
		OciAuthProvider:  pt.pp.ociAuthProvider,
		RenderOutputDir:  renderOutputDir,
	}
	if pt.pp.obj.Spec.Target != nil {
		props.TargetName = *pt.pp.obj.Spec.Target
	}
	if pt.pp.obj.Spec.TargetNameOverride != nil {
		props.TargetNameOverride = *pt.pp.obj.Spec.TargetNameOverride
	}
	if pt.pp.obj.Spec.Context != nil {
		props.ContextOverride = *pt.pp.obj.Spec.Context
	}

	restConfig, contextName, err := p.LoadK8sConfig(ctx, props.TargetName, props.ContextOverride, props.OfflineK8s)
	if err != nil {
		return nil, err
	}

	discovery, mapper, err := k8s2.CreateDiscoveryAndMapper(ctx, restConfig)
	if err != nil {
		return nil, err
	}

	k, err := k8s2.NewK8sCluster(ctx, restConfig, discovery, mapper, props.DryRun)
	if err != nil {
		return nil, err
	}

	targetContext, err := target_context.NewTargetContext(ctx, p, contextName, k, props)
	if err != nil {
		return targetContext, err
	}
	err = targetContext.DeploymentCollection.Prepare()
	if err != nil {
		return targetContext, err
	}
	return targetContext, nil
}

func (pt *preparedTarget) addCommandResultInfo(ctx context.Context, cmdResult *result.CommandResult, rr *kluctlv1.ManualRequestResult, reconcileId string, objectsHash string) error {
	if cmdResult == nil {
		// this might happen if something has gone so wrong that nothing succeeded
		return nil
	}

	cmdResult.Id = uuid.NewString()
	cmdResult.ReconcileId = reconcileId
	cmdResult.Command.Initiator = result.CommandInititiator_KluctlDeployment
	cmdResult.KluctlDeployment = &result.KluctlDeploymentInfo{
		Name:      pt.pp.obj.Name,
		Namespace: pt.pp.obj.Namespace,
	}
	cmdResult.RenderedObjectsHash = objectsHash

	if rr != nil && rr.Request.OverridesPatch != nil {
		cmdResult.OverridesPatch = uo.FromStringMust(string(rr.Request.OverridesPatch.Raw))
	}

	var err error
	cmdResult.KluctlDeployment.ClusterId, err = k8s2.GetClusterId(ctx, pt.pp.r.Client)
	if err != nil {
		return err
	}

	// the ref is not properly set by addGitInfo due to the way the repo cache checks out by commit
	if pt.pp.co.CheckedOutRef != (types2.GitRef{}) {
		cmdResult.GitInfo.Ref = &pt.pp.co.CheckedOutRef
	}

	return nil
}

func (pt *preparedTarget) writeCommandResult(ctx context.Context, cmdResult *result.CommandResult, rr *kluctlv1.ManualRequestResult, commandName string, reconcileId string, objectsHash string, forceStore bool) error {
	log := ctrl.LoggerFrom(ctx)

	if cmdResult == nil {
		return fmt.Errorf("command result is nil, which should not happen")
	}

	err := pt.addCommandResultInfo(ctx, cmdResult, rr, reconcileId, objectsHash)
	if err != nil {
		return err
	}

	summary := cmdResult.BuildSummary()
	needStore := summary.NewObjects != 0 ||
		summary.ChangedObjects != 0 ||
		summary.OrphanObjects != 0 ||
		summary.DeletedObjects != 0 ||
		len(summary.Errors) != 0 ||
		len(summary.Warnings) != 0
	if !needStore && forceStore {
		needStore = true
		log.Info("forcing storing of empty command result because the command was requested")
	}

	if !needStore {
		log.Info("skipping storing of empty command result")
	} else if pt.pp.r.ResultStore != nil {
		log.Info(fmt.Sprintf("Writing command result %s", cmdResult.Id))
		err = pt.pp.r.ResultStore.WriteCommandResult(cmdResult)
		if err != nil {
			log.Error(err, "Writing command result failed")
		}
	}

	log.Info(fmt.Sprintf("command finished with %d errors and %d warnings", len(cmdResult.Errors), len(cmdResult.Warnings)))
	defer pt.exportCommandResultMetricsToProm(summary)

	msg := pt.pp.r.buildResultMessage(summary, commandName)
	pt.pp.r.event(ctx, pt.pp.obj, len(cmdResult.Errors) != 0, msg, nil)

	return nil
}

func (pt *preparedTarget) writeValidateResult(ctx context.Context, validateResult *result.ValidateResult, rr *kluctlv1.ManualRequestResult, reconcileId string, objectsHash string) error {
	log := ctrl.LoggerFrom(ctx)

	if validateResult == nil {
		return fmt.Errorf("validate result is nil, which should not happen")
	}

	validateResult.Id = uuid.NewString()
	validateResult.ReconcileId = reconcileId
	validateResult.KluctlDeployment = &result.KluctlDeploymentInfo{
		Name:      pt.pp.obj.Name,
		Namespace: pt.pp.obj.Namespace,
	}
	validateResult.RenderedObjectsHash = objectsHash

	if rr != nil && rr.Request.OverridesPatch != nil {
		validateResult.OverridesPatch = uo.FromStringMust(string(rr.Request.OverridesPatch.Raw))
	}

	var err error
	validateResult.KluctlDeployment.ClusterId, err = k8s2.GetClusterId(ctx, pt.pp.r.Client)
	if err != nil {
		return err
	}

	if pt.pp.r.ResultStore != nil {
		log.Info(fmt.Sprintf("Writing validate result %s", validateResult.Id))
		err = pt.pp.r.ResultStore.WriteValidateResult(validateResult)
		if err != nil {
			log.Error(err, "Writing validate result failed")
		}
	}

	return nil
}

func (pt *preparedTarget) kluctlDeployOrPokeImages(deployMode string, targetContext *target_context.TargetContext) (*result.CommandResult, error) {
	if deployMode == kluctlv1.KluctlDeployModeFull {
		return pt.kluctlDeploy(targetContext), nil
	} else if deployMode == kluctlv1.KluctlDeployPokeImages {
		return pt.kluctlPokeImages(targetContext), nil
	} else {
		return nil, fmt.Errorf("deployMode '%s' not supported", deployMode)
	}
}

func (pt *preparedTarget) kluctlDeploy(targetContext *target_context.TargetContext) *result.CommandResult {
	timer := prometheus.NewTimer(internal_metrics.NewKluctlDeploymentDuration(pt.pp.obj.ObjectMeta.Namespace, pt.pp.obj.ObjectMeta.Name, pt.pp.obj.Spec.DeployMode))
	defer timer.ObserveDuration()
	cmd := commands.NewDeployCommand(targetContext)
	cmd.ForceApply = pt.pp.obj.Spec.ForceApply
	cmd.ReplaceOnError = pt.pp.obj.Spec.ReplaceOnError
	cmd.ForceReplaceOnError = pt.pp.obj.Spec.ForceReplaceOnError
	cmd.AbortOnError = pt.pp.obj.Spec.AbortOnError
	cmd.ReadinessTimeout = time.Minute * 10
	cmd.NoWait = pt.pp.obj.Spec.NoWait
	cmd.Prune = pt.pp.obj.Spec.Prune
	cmd.WaitPrune = false

	cmdResult := cmd.Run(nil)
	return cmdResult
}

func (pt *preparedTarget) kluctlPokeImages(targetContext *target_context.TargetContext) *result.CommandResult {
	timer := prometheus.NewTimer(internal_metrics.NewKluctlDeploymentDuration(pt.pp.obj.ObjectMeta.Namespace, pt.pp.obj.ObjectMeta.Name, pt.pp.obj.Spec.DeployMode))
	defer timer.ObserveDuration()
	cmd := commands.NewPokeImagesCommand(targetContext)

	cmdResult := cmd.Run()
	return cmdResult
}

func (pt *preparedTarget) kluctlPrune(targetContext *target_context.TargetContext) *result.CommandResult {
	timer := prometheus.NewTimer(internal_metrics.NewKluctlDeploymentDuration(pt.pp.obj.ObjectMeta.Namespace, pt.pp.obj.ObjectMeta.Name, pt.pp.obj.Spec.DeployMode))
	defer timer.ObserveDuration()
	cmd := commands.NewPruneCommand("", targetContext, false)

	cmdResult := cmd.Run(func(refs []k8s.ObjectRef) error {
		pt.printDeletedRefs(targetContext.SharedContext.Ctx, refs)
		return nil
	})
	return cmdResult
}

func (pt *preparedTarget) kluctlDiff(targetContext *target_context.TargetContext, resourceVersions map[k8s.ObjectRef]string) *result.CommandResult {
	cmd := commands.NewDiffCommand(targetContext)
	cmd.ForceApply = pt.pp.obj.Spec.ForceApply
	cmd.ReplaceOnError = pt.pp.obj.Spec.ReplaceOnError
	cmd.ForceReplaceOnError = pt.pp.obj.Spec.ForceReplaceOnError
	cmd.SkipResourceVersions = resourceVersions

	cmdResult := cmd.Run()
	return cmdResult
}

func (pt *preparedTarget) kluctlValidate(targetContext *target_context.TargetContext) *result.ValidateResult {
	timer := prometheus.NewTimer(internal_metrics.NewKluctlValidateDuration(pt.pp.obj.ObjectMeta.Namespace, pt.pp.obj.ObjectMeta.Name))
	defer timer.ObserveDuration()

	cmd := commands.NewValidateCommand(targetContext.Target.Discriminator, targetContext)

	validateResult := cmd.Run(targetContext.SharedContext.Ctx)
	return validateResult
}

func (pt *preparedTarget) kluctlDelete(ctx context.Context, discriminator string) (*result.CommandResult, error) {
	if !pt.pp.obj.Spec.Delete {
		return nil, nil
	}

	timer := prometheus.NewTimer(internal_metrics.NewKluctlDeleteDuration(pt.pp.obj.ObjectMeta.Namespace, pt.pp.obj.ObjectMeta.Name))
	defer timer.ObserveDuration()

	inclusion := pt.buildInclusion()

	cmd := commands.NewDeleteCommand(discriminator, nil, inclusion, false)

	restConfig, err := pt.buildRestConfig(ctx)
	if err != nil {
		return nil, err
	}

	discovery, mapper, err := k8s2.CreateDiscoveryAndMapper(ctx, restConfig)
	if err != nil {
		return nil, err
	}

	k, err := k8s2.NewK8sCluster(ctx, restConfig, discovery, mapper, pt.pp.r.DryRun || pt.pp.obj.Spec.DryRun)
	if err != nil {
		return nil, err
	}

	cmdResult := cmd.Run(ctx, k, func(refs []k8s.ObjectRef) error {
		pt.printDeletedRefs(ctx, refs)
		return nil
	})

	if pt.pp.obj.Spec.Target != nil {
		cmdResult.TargetKey.TargetName = *pt.pp.obj.Spec.Target
	}
	if pt.pp.obj.Spec.TargetNameOverride != nil {
		cmdResult.TargetKey.TargetName = *pt.pp.obj.Spec.TargetNameOverride
	}

	cmdResult.TargetKey.Discriminator = discriminator
	cmdResult.TargetKey.ClusterId = cmdResult.ClusterInfo.ClusterId

	return cmdResult, err
}

func (pt *preparedTarget) printDeletedRefs(ctx context.Context, refs []k8s.ObjectRef) {
	log := ctrl.LoggerFrom(ctx)

	var refStrs []string
	for _, ref := range refs {
		refStrs = append(refStrs, ref.String())
	}
	if len(refStrs) != 0 {
		log.Info(fmt.Sprintf("deleting (without waiting): %s", strings.Join(refStrs, ", ")))
	}
}

func (pt *preparedTarget) exportCommandResultMetricsToProm(summary *result.CommandResultSummary) {
	internal_metrics.NewKluctlNumberOfDeletedObjects(pt.pp.obj.Namespace, pt.pp.obj.Name).Set(float64(summary.DeletedObjects))
	internal_metrics.NewKluctlNumberOfChangedObjects(pt.pp.obj.Namespace, pt.pp.obj.Name).Set(float64(summary.ChangedObjects))
	internal_metrics.NewKluctlNumberOfOrphanObjects(pt.pp.obj.Namespace, pt.pp.obj.Name).Set(float64(summary.OrphanObjects))
	internal_metrics.NewKluctlNumberOfWarnings(pt.pp.obj.Namespace, pt.pp.obj.Name, summary.Command.Command).Set(float64(len(summary.Warnings)))
	internal_metrics.NewKluctlNumberOfErrors(pt.pp.obj.Namespace, pt.pp.obj.Name, summary.Command.Command).Set(float64(len(summary.Errors)))
}
