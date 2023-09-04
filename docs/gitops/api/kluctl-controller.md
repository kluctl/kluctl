<h1>Kluctl Controller API reference</h1>
<p>Packages:</p>
<ul class="simple">
<li>
<a href="#gitops.kluctl.io%2fv1beta1">gitops.kluctl.io/v1beta1</a>
</li>
</ul>
<h2 id="gitops.kluctl.io/v1beta1">gitops.kluctl.io/v1beta1</h2>
<p>Package v1beta1 contains API Schema definitions for the gitops.kluctl.io v1beta1 API group.</p>
Resource Types:
<ul class="simple"></ul>
<h3 id="gitops.kluctl.io/v1beta1.Decryption">Decryption
</h3>
<p>
(<em>Appears on:</em>
<a href="#gitops.kluctl.io/v1beta1.KluctlDeploymentSpec">KluctlDeploymentSpec</a>)
</p>
<p>Decryption defines how decryption is handled for Kubernetes manifests.</p>
<div class="md-typeset__scrollwrap">
<div class="md-typeset__table">
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>provider</code><br>
<em>
string
</em>
</td>
<td>
<p>Provider is the name of the decryption engine.</p>
</td>
</tr>
<tr>
<td>
<code>secretRef</code><br>
<em>
<a href="#gitops.kluctl.io/v1beta1.LocalObjectReference">
LocalObjectReference
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>The secret name containing the private OpenPGP keys used for decryption.</p>
</td>
</tr>
<tr>
<td>
<code>serviceAccount</code><br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>ServiceAccount specifies the service account used to authenticate against cloud providers.
This is currently only usable for AWS KMS keys. The specified service account will be used to authenticate to AWS
by signing a token in an IRSA compliant way.</p>
</td>
</tr>
</tbody>
</table>
</div>
</div>
<h3 id="gitops.kluctl.io/v1beta1.GitCredentials">GitCredentials
</h3>
<p>
(<em>Appears on:</em>
<a href="#gitops.kluctl.io/v1beta1.ProjectSource">ProjectSource</a>)
</p>
<div class="md-typeset__scrollwrap">
<div class="md-typeset__table">
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>host</code><br>
<em>
string
</em>
</td>
<td>
<p>Host specifies the hostname that this git secret applies to. If set to &lsquo;<em>&rsquo;, this set of credentials
applies to all hosts.
Using &lsquo;</em>&rsquo; for http(s) based repositories is not supported, meaning that such credentials sets will be ignored.
You must always set a proper hostname in that case.</p>
</td>
</tr>
<tr>
<td>
<code>pathPrefix</code><br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>PathPrefix specified the path prefix to be used to filter git urls. Only urls that have this prefix will use
this set of credentials.</p>
</td>
</tr>
<tr>
<td>
<code>secretRef</code><br>
<em>
<a href="#gitops.kluctl.io/v1beta1.LocalObjectReference">
LocalObjectReference
</a>
</em>
</td>
<td>
<p>SecretRef specifies the Secret containing authentication credentials for
the git repository.
For HTTPS repositories the Secret must contain &lsquo;username&rsquo; and &lsquo;password&rsquo;
fields.
For SSH repositories the Secret must contain &lsquo;identity&rsquo;
and &lsquo;known_hosts&rsquo; fields.</p>
</td>
</tr>
</tbody>
</table>
</div>
</div>
<h3 id="gitops.kluctl.io/v1beta1.HelmCredentials">HelmCredentials
</h3>
<p>
(<em>Appears on:</em>
<a href="#gitops.kluctl.io/v1beta1.KluctlDeploymentSpec">KluctlDeploymentSpec</a>)
</p>
<div class="md-typeset__scrollwrap">
<div class="md-typeset__table">
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>secretRef</code><br>
<em>
<a href="#gitops.kluctl.io/v1beta1.LocalObjectReference">
LocalObjectReference
</a>
</em>
</td>
<td>
<p>SecretRef holds the name of a secret that contains the Helm credentials.
The secret must either contain the fields <code>credentialsId</code> which refers to the credentialsId
found in <a href="https://kluctl.io/docs/kluctl/reference/deployments/helm/#private-chart-repositories">https://kluctl.io/docs/kluctl/reference/deployments/helm/#private-chart-repositories</a> or an <code>url</code> used
to match the credentials found in Kluctl projects helm-chart.yaml files.
The secret can either container basic authentication credentials via <code>username</code> and <code>password</code> or
TLS authentication via <code>certFile</code> and <code>keyFile</code>. <code>caFile</code> can be specified to override the CA to use while
contacting the repository.
The secret can also contain <code>insecureSkipTlsVerify: &quot;true&quot;</code>, which will disable TLS verification.
<code>passCredentialsAll: &quot;true&quot;</code> can be specified to make the controller pass credentials to all requests, even if
the hostname changes in-between.</p>
</td>
</tr>
</tbody>
</table>
</div>
</div>
<h3 id="gitops.kluctl.io/v1beta1.KluctlDeployment">KluctlDeployment
</h3>
<p>KluctlDeployment is the Schema for the kluctldeployments API</p>
<div class="md-typeset__scrollwrap">
<div class="md-typeset__table">
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>metadata</code><br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.18/#objectmeta-v1-meta">
Kubernetes meta/v1.ObjectMeta
</a>
</em>
</td>
<td>
Refer to the Kubernetes API documentation for the fields of the
<code>metadata</code> field.
</td>
</tr>
<tr>
<td>
<code>spec</code><br>
<em>
<a href="#gitops.kluctl.io/v1beta1.KluctlDeploymentSpec">
KluctlDeploymentSpec
</a>
</em>
</td>
<td>
<br/>
<br/>
<table>
<tr>
<td>
<code>source</code><br>
<em>
<a href="#gitops.kluctl.io/v1beta1.ProjectSource">
ProjectSource
</a>
</em>
</td>
<td>
<p>Specifies the project source location</p>
</td>
</tr>
<tr>
<td>
<code>decryption</code><br>
<em>
<a href="#gitops.kluctl.io/v1beta1.Decryption">
Decryption
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Decrypt Kubernetes secrets before applying them on the cluster.</p>
</td>
</tr>
<tr>
<td>
<code>interval</code><br>
<em>
<a href="https://godoc.org/k8s.io/apimachinery/pkg/apis/meta/v1#Duration">
Kubernetes meta/v1.Duration
</a>
</em>
</td>
<td>
<p>The interval at which to reconcile the KluctlDeployment.
Reconciliation means that the deployment is fully rendered and only deployed when the result changes compared
to the last deployment.
To override this behavior, set the DeployInterval value.</p>
</td>
</tr>
<tr>
<td>
<code>retryInterval</code><br>
<em>
<a href="https://godoc.org/k8s.io/apimachinery/pkg/apis/meta/v1#Duration">
Kubernetes meta/v1.Duration
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>The interval at which to retry a previously failed reconciliation.
When not specified, the controller uses the Interval
value to retry failures.</p>
</td>
</tr>
<tr>
<td>
<code>deployInterval</code><br>
<em>
<a href="#gitops.kluctl.io/v1beta1.SafeDuration">
SafeDuration
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>DeployInterval specifies the interval at which to deploy the KluctlDeployment, even in cases the rendered
result does not change.</p>
</td>
</tr>
<tr>
<td>
<code>validateInterval</code><br>
<em>
<a href="#gitops.kluctl.io/v1beta1.SafeDuration">
SafeDuration
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>ValidateInterval specifies the interval at which to validate the KluctlDeployment.
Validation is performed the same way as with &lsquo;kluctl validate -t <target>&rsquo;.
Defaults to the same value as specified in Interval.
Validate is also performed whenever a deployment is performed, independent of the value of ValidateInterval</p>
</td>
</tr>
<tr>
<td>
<code>timeout</code><br>
<em>
<a href="https://godoc.org/k8s.io/apimachinery/pkg/apis/meta/v1#Duration">
Kubernetes meta/v1.Duration
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Timeout for all operations.
Defaults to &lsquo;Interval&rsquo; duration.</p>
</td>
</tr>
<tr>
<td>
<code>suspend</code><br>
<em>
bool
</em>
</td>
<td>
<em>(Optional)</em>
<p>This flag tells the controller to suspend subsequent kluctl executions,
it does not apply to already started executions. Defaults to false.</p>
</td>
</tr>
<tr>
<td>
<code>helmCredentials</code><br>
<em>
<a href="#gitops.kluctl.io/v1beta1.HelmCredentials">
[]HelmCredentials
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>HelmCredentials is a list of Helm credentials used when non pre-pulled Helm Charts are used inside a
Kluctl deployment.</p>
</td>
</tr>
<tr>
<td>
<code>serviceAccountName</code><br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>The name of the Kubernetes service account to use while deploying.
If not specified, the default service account is used.</p>
</td>
</tr>
<tr>
<td>
<code>kubeConfig</code><br>
<em>
<a href="#gitops.kluctl.io/v1beta1.KubeConfig">
KubeConfig
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>The KubeConfig for deploying to the target cluster.
Specifies the kubeconfig to be used when invoking kluctl. Contexts in this kubeconfig must match
the context found in the kluctl target. As an alternative, specify the context to be used via &lsquo;context&rsquo;</p>
</td>
</tr>
<tr>
<td>
<code>target</code><br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Target specifies the kluctl target to deploy. If not specified, an empty target is used that has no name and no
context. Use &lsquo;TargetName&rsquo; and &lsquo;Context&rsquo; to specify the name and context in that case.</p>
</td>
</tr>
<tr>
<td>
<code>targetNameOverride</code><br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>TargetNameOverride sets or overrides the target name. This is especially useful when deployment without a target.</p>
</td>
</tr>
<tr>
<td>
<code>context</code><br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>If specified, overrides the context to be used. This will effectively make kluctl ignore the context specified
in the target.</p>
</td>
</tr>
<tr>
<td>
<code>args</code><br>
<em>
k8s.io/apimachinery/pkg/runtime.RawExtension
</em>
</td>
<td>
<em>(Optional)</em>
<p>Args specifies dynamic target args.</p>
</td>
</tr>
<tr>
<td>
<code>images</code><br>
<em>
[]github.com/kluctl/kluctl/v2/pkg/types.FixedImage
</em>
</td>
<td>
<em>(Optional)</em>
<p>Images contains a list of fixed image overrides.
Equivalent to using &lsquo;&ndash;fixed-images-file&rsquo; when calling kluctl.</p>
</td>
</tr>
<tr>
<td>
<code>dryRun</code><br>
<em>
bool
</em>
</td>
<td>
<em>(Optional)</em>
<p>DryRun instructs kluctl to run everything in dry-run mode.
Equivalent to using &lsquo;&ndash;dry-run&rsquo; when calling kluctl.</p>
</td>
</tr>
<tr>
<td>
<code>noWait</code><br>
<em>
bool
</em>
</td>
<td>
<em>(Optional)</em>
<p>NoWait instructs kluctl to not wait for any resources to become ready, including hooks.
Equivalent to using &lsquo;&ndash;no-wait&rsquo; when calling kluctl.</p>
</td>
</tr>
<tr>
<td>
<code>forceApply</code><br>
<em>
bool
</em>
</td>
<td>
<em>(Optional)</em>
<p>ForceApply instructs kluctl to force-apply in case of SSA conflicts.
Equivalent to using &lsquo;&ndash;force-apply&rsquo; when calling kluctl.</p>
</td>
</tr>
<tr>
<td>
<code>replaceOnError</code><br>
<em>
bool
</em>
</td>
<td>
<em>(Optional)</em>
<p>ReplaceOnError instructs kluctl to replace resources on error.
Equivalent to using &lsquo;&ndash;replace-on-error&rsquo; when calling kluctl.</p>
</td>
</tr>
<tr>
<td>
<code>forceReplaceOnError</code><br>
<em>
bool
</em>
</td>
<td>
<em>(Optional)</em>
<p>ForceReplaceOnError instructs kluctl to force-replace resources in case a normal replace fails.
Equivalent to using &lsquo;&ndash;force-replace-on-error&rsquo; when calling kluctl.</p>
</td>
</tr>
<tr>
<td>
<code>abortOnError</code><br>
<em>
bool
</em>
</td>
<td>
<em>(Optional)</em>
<p>ForceReplaceOnError instructs kluctl to abort deployments immediately when something fails.
Equivalent to using &lsquo;&ndash;abort-on-error&rsquo; when calling kluctl.</p>
</td>
</tr>
<tr>
<td>
<code>includeTags</code><br>
<em>
[]string
</em>
</td>
<td>
<em>(Optional)</em>
<p>IncludeTags instructs kluctl to only include deployments with given tags.
Equivalent to using &lsquo;&ndash;include-tag&rsquo; when calling kluctl.</p>
</td>
</tr>
<tr>
<td>
<code>excludeTags</code><br>
<em>
[]string
</em>
</td>
<td>
<em>(Optional)</em>
<p>ExcludeTags instructs kluctl to exclude deployments with given tags.
Equivalent to using &lsquo;&ndash;exclude-tag&rsquo; when calling kluctl.</p>
</td>
</tr>
<tr>
<td>
<code>includeDeploymentDirs</code><br>
<em>
[]string
</em>
</td>
<td>
<em>(Optional)</em>
<p>IncludeDeploymentDirs instructs kluctl to only include deployments with the given dir.
Equivalent to using &lsquo;&ndash;include-deployment-dir&rsquo; when calling kluctl.</p>
</td>
</tr>
<tr>
<td>
<code>excludeDeploymentDirs</code><br>
<em>
[]string
</em>
</td>
<td>
<em>(Optional)</em>
<p>ExcludeDeploymentDirs instructs kluctl to exclude deployments with the given dir.
Equivalent to using &lsquo;&ndash;exclude-deployment-dir&rsquo; when calling kluctl.</p>
</td>
</tr>
<tr>
<td>
<code>deployMode</code><br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>DeployMode specifies what deploy mode should be used.
The options &lsquo;full-deploy&rsquo; and &lsquo;poke-images&rsquo; are supported.
With the &lsquo;poke-images&rsquo; option, only images are patched into the target without performing a full deployment.</p>
</td>
</tr>
<tr>
<td>
<code>validate</code><br>
<em>
bool
</em>
</td>
<td>
<em>(Optional)</em>
<p>Validate enables validation after deploying</p>
</td>
</tr>
<tr>
<td>
<code>prune</code><br>
<em>
bool
</em>
</td>
<td>
<em>(Optional)</em>
<p>Prune enables pruning after deploying.</p>
</td>
</tr>
<tr>
<td>
<code>delete</code><br>
<em>
bool
</em>
</td>
<td>
<em>(Optional)</em>
<p>Delete enables deletion of the specified target when the KluctlDeployment object gets deleted.</p>
</td>
</tr>
<tr>
<td>
<code>manual</code><br>
<em>
bool
</em>
</td>
<td>
<em>(Optional)</em>
<p>Manual enables manual deployments, meaning that the deployment will initially start as a dry run deployment
and only after manual approval cause a real deployment</p>
</td>
</tr>
<tr>
<td>
<code>manualObjectsHash</code><br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>ManualObjectsHash specifies the rendered objects hash that is approved for manual deployment.
If Manual is set to true, the controller will skip deployments when the current reconciliation loops calculated
objects hash does not match this value.
There are two ways to use this value properly.
1. Set it manually to the value found in status.lastObjectsHash.
2. Use the Kluctl Webui to manually approve a deployment, which will set this field appropriately.</p>
</td>
</tr>
</table>
</td>
</tr>
<tr>
<td>
<code>status</code><br>
<em>
<a href="#gitops.kluctl.io/v1beta1.KluctlDeploymentStatus">
KluctlDeploymentStatus
</a>
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
</div>
</div>
<h3 id="gitops.kluctl.io/v1beta1.KluctlDeploymentSpec">KluctlDeploymentSpec
</h3>
<p>
(<em>Appears on:</em>
<a href="#gitops.kluctl.io/v1beta1.KluctlDeployment">KluctlDeployment</a>)
</p>
<div class="md-typeset__scrollwrap">
<div class="md-typeset__table">
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>source</code><br>
<em>
<a href="#gitops.kluctl.io/v1beta1.ProjectSource">
ProjectSource
</a>
</em>
</td>
<td>
<p>Specifies the project source location</p>
</td>
</tr>
<tr>
<td>
<code>decryption</code><br>
<em>
<a href="#gitops.kluctl.io/v1beta1.Decryption">
Decryption
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Decrypt Kubernetes secrets before applying them on the cluster.</p>
</td>
</tr>
<tr>
<td>
<code>interval</code><br>
<em>
<a href="https://godoc.org/k8s.io/apimachinery/pkg/apis/meta/v1#Duration">
Kubernetes meta/v1.Duration
</a>
</em>
</td>
<td>
<p>The interval at which to reconcile the KluctlDeployment.
Reconciliation means that the deployment is fully rendered and only deployed when the result changes compared
to the last deployment.
To override this behavior, set the DeployInterval value.</p>
</td>
</tr>
<tr>
<td>
<code>retryInterval</code><br>
<em>
<a href="https://godoc.org/k8s.io/apimachinery/pkg/apis/meta/v1#Duration">
Kubernetes meta/v1.Duration
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>The interval at which to retry a previously failed reconciliation.
When not specified, the controller uses the Interval
value to retry failures.</p>
</td>
</tr>
<tr>
<td>
<code>deployInterval</code><br>
<em>
<a href="#gitops.kluctl.io/v1beta1.SafeDuration">
SafeDuration
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>DeployInterval specifies the interval at which to deploy the KluctlDeployment, even in cases the rendered
result does not change.</p>
</td>
</tr>
<tr>
<td>
<code>validateInterval</code><br>
<em>
<a href="#gitops.kluctl.io/v1beta1.SafeDuration">
SafeDuration
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>ValidateInterval specifies the interval at which to validate the KluctlDeployment.
Validation is performed the same way as with &lsquo;kluctl validate -t <target>&rsquo;.
Defaults to the same value as specified in Interval.
Validate is also performed whenever a deployment is performed, independent of the value of ValidateInterval</p>
</td>
</tr>
<tr>
<td>
<code>timeout</code><br>
<em>
<a href="https://godoc.org/k8s.io/apimachinery/pkg/apis/meta/v1#Duration">
Kubernetes meta/v1.Duration
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Timeout for all operations.
Defaults to &lsquo;Interval&rsquo; duration.</p>
</td>
</tr>
<tr>
<td>
<code>suspend</code><br>
<em>
bool
</em>
</td>
<td>
<em>(Optional)</em>
<p>This flag tells the controller to suspend subsequent kluctl executions,
it does not apply to already started executions. Defaults to false.</p>
</td>
</tr>
<tr>
<td>
<code>helmCredentials</code><br>
<em>
<a href="#gitops.kluctl.io/v1beta1.HelmCredentials">
[]HelmCredentials
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>HelmCredentials is a list of Helm credentials used when non pre-pulled Helm Charts are used inside a
Kluctl deployment.</p>
</td>
</tr>
<tr>
<td>
<code>serviceAccountName</code><br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>The name of the Kubernetes service account to use while deploying.
If not specified, the default service account is used.</p>
</td>
</tr>
<tr>
<td>
<code>kubeConfig</code><br>
<em>
<a href="#gitops.kluctl.io/v1beta1.KubeConfig">
KubeConfig
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>The KubeConfig for deploying to the target cluster.
Specifies the kubeconfig to be used when invoking kluctl. Contexts in this kubeconfig must match
the context found in the kluctl target. As an alternative, specify the context to be used via &lsquo;context&rsquo;</p>
</td>
</tr>
<tr>
<td>
<code>target</code><br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Target specifies the kluctl target to deploy. If not specified, an empty target is used that has no name and no
context. Use &lsquo;TargetName&rsquo; and &lsquo;Context&rsquo; to specify the name and context in that case.</p>
</td>
</tr>
<tr>
<td>
<code>targetNameOverride</code><br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>TargetNameOverride sets or overrides the target name. This is especially useful when deployment without a target.</p>
</td>
</tr>
<tr>
<td>
<code>context</code><br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>If specified, overrides the context to be used. This will effectively make kluctl ignore the context specified
in the target.</p>
</td>
</tr>
<tr>
<td>
<code>args</code><br>
<em>
k8s.io/apimachinery/pkg/runtime.RawExtension
</em>
</td>
<td>
<em>(Optional)</em>
<p>Args specifies dynamic target args.</p>
</td>
</tr>
<tr>
<td>
<code>images</code><br>
<em>
[]github.com/kluctl/kluctl/v2/pkg/types.FixedImage
</em>
</td>
<td>
<em>(Optional)</em>
<p>Images contains a list of fixed image overrides.
Equivalent to using &lsquo;&ndash;fixed-images-file&rsquo; when calling kluctl.</p>
</td>
</tr>
<tr>
<td>
<code>dryRun</code><br>
<em>
bool
</em>
</td>
<td>
<em>(Optional)</em>
<p>DryRun instructs kluctl to run everything in dry-run mode.
Equivalent to using &lsquo;&ndash;dry-run&rsquo; when calling kluctl.</p>
</td>
</tr>
<tr>
<td>
<code>noWait</code><br>
<em>
bool
</em>
</td>
<td>
<em>(Optional)</em>
<p>NoWait instructs kluctl to not wait for any resources to become ready, including hooks.
Equivalent to using &lsquo;&ndash;no-wait&rsquo; when calling kluctl.</p>
</td>
</tr>
<tr>
<td>
<code>forceApply</code><br>
<em>
bool
</em>
</td>
<td>
<em>(Optional)</em>
<p>ForceApply instructs kluctl to force-apply in case of SSA conflicts.
Equivalent to using &lsquo;&ndash;force-apply&rsquo; when calling kluctl.</p>
</td>
</tr>
<tr>
<td>
<code>replaceOnError</code><br>
<em>
bool
</em>
</td>
<td>
<em>(Optional)</em>
<p>ReplaceOnError instructs kluctl to replace resources on error.
Equivalent to using &lsquo;&ndash;replace-on-error&rsquo; when calling kluctl.</p>
</td>
</tr>
<tr>
<td>
<code>forceReplaceOnError</code><br>
<em>
bool
</em>
</td>
<td>
<em>(Optional)</em>
<p>ForceReplaceOnError instructs kluctl to force-replace resources in case a normal replace fails.
Equivalent to using &lsquo;&ndash;force-replace-on-error&rsquo; when calling kluctl.</p>
</td>
</tr>
<tr>
<td>
<code>abortOnError</code><br>
<em>
bool
</em>
</td>
<td>
<em>(Optional)</em>
<p>ForceReplaceOnError instructs kluctl to abort deployments immediately when something fails.
Equivalent to using &lsquo;&ndash;abort-on-error&rsquo; when calling kluctl.</p>
</td>
</tr>
<tr>
<td>
<code>includeTags</code><br>
<em>
[]string
</em>
</td>
<td>
<em>(Optional)</em>
<p>IncludeTags instructs kluctl to only include deployments with given tags.
Equivalent to using &lsquo;&ndash;include-tag&rsquo; when calling kluctl.</p>
</td>
</tr>
<tr>
<td>
<code>excludeTags</code><br>
<em>
[]string
</em>
</td>
<td>
<em>(Optional)</em>
<p>ExcludeTags instructs kluctl to exclude deployments with given tags.
Equivalent to using &lsquo;&ndash;exclude-tag&rsquo; when calling kluctl.</p>
</td>
</tr>
<tr>
<td>
<code>includeDeploymentDirs</code><br>
<em>
[]string
</em>
</td>
<td>
<em>(Optional)</em>
<p>IncludeDeploymentDirs instructs kluctl to only include deployments with the given dir.
Equivalent to using &lsquo;&ndash;include-deployment-dir&rsquo; when calling kluctl.</p>
</td>
</tr>
<tr>
<td>
<code>excludeDeploymentDirs</code><br>
<em>
[]string
</em>
</td>
<td>
<em>(Optional)</em>
<p>ExcludeDeploymentDirs instructs kluctl to exclude deployments with the given dir.
Equivalent to using &lsquo;&ndash;exclude-deployment-dir&rsquo; when calling kluctl.</p>
</td>
</tr>
<tr>
<td>
<code>deployMode</code><br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>DeployMode specifies what deploy mode should be used.
The options &lsquo;full-deploy&rsquo; and &lsquo;poke-images&rsquo; are supported.
With the &lsquo;poke-images&rsquo; option, only images are patched into the target without performing a full deployment.</p>
</td>
</tr>
<tr>
<td>
<code>validate</code><br>
<em>
bool
</em>
</td>
<td>
<em>(Optional)</em>
<p>Validate enables validation after deploying</p>
</td>
</tr>
<tr>
<td>
<code>prune</code><br>
<em>
bool
</em>
</td>
<td>
<em>(Optional)</em>
<p>Prune enables pruning after deploying.</p>
</td>
</tr>
<tr>
<td>
<code>delete</code><br>
<em>
bool
</em>
</td>
<td>
<em>(Optional)</em>
<p>Delete enables deletion of the specified target when the KluctlDeployment object gets deleted.</p>
</td>
</tr>
<tr>
<td>
<code>manual</code><br>
<em>
bool
</em>
</td>
<td>
<em>(Optional)</em>
<p>Manual enables manual deployments, meaning that the deployment will initially start as a dry run deployment
and only after manual approval cause a real deployment</p>
</td>
</tr>
<tr>
<td>
<code>manualObjectsHash</code><br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>ManualObjectsHash specifies the rendered objects hash that is approved for manual deployment.
If Manual is set to true, the controller will skip deployments when the current reconciliation loops calculated
objects hash does not match this value.
There are two ways to use this value properly.
1. Set it manually to the value found in status.lastObjectsHash.
2. Use the Kluctl Webui to manually approve a deployment, which will set this field appropriately.</p>
</td>
</tr>
</tbody>
</table>
</div>
</div>
<h3 id="gitops.kluctl.io/v1beta1.KluctlDeploymentStatus">KluctlDeploymentStatus
</h3>
<p>
(<em>Appears on:</em>
<a href="#gitops.kluctl.io/v1beta1.KluctlDeployment">KluctlDeployment</a>)
</p>
<p>KluctlDeploymentStatus defines the observed state of KluctlDeployment</p>
<div class="md-typeset__scrollwrap">
<div class="md-typeset__table">
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>lastHandledReconcileAt</code><br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>LastHandledReconcileAt holds the value of the most recent
reconcile request value, so a change of the annotation value
can be detected.</p>
</td>
</tr>
<tr>
<td>
<code>lastHandledDeployAt</code><br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
</td>
</tr>
<tr>
<td>
<code>lastHandledValidateAt</code><br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
</td>
</tr>
<tr>
<td>
<code>observedGeneration</code><br>
<em>
int64
</em>
</td>
<td>
<em>(Optional)</em>
<p>ObservedGeneration is the last reconciled generation.</p>
</td>
</tr>
<tr>
<td>
<code>observedCommit</code><br>
<em>
string
</em>
</td>
<td>
<p>ObservedCommit is the last commit observed</p>
</td>
</tr>
<tr>
<td>
<code>conditions</code><br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.18/#condition-v1-meta">
[]Kubernetes meta/v1.Condition
</a>
</em>
</td>
<td>
<em>(Optional)</em>
</td>
</tr>
<tr>
<td>
<code>projectKey</code><br>
<em>
github.com/kluctl/kluctl/v2/pkg/types/result.ProjectKey
</em>
</td>
<td>
<em>(Optional)</em>
</td>
</tr>
<tr>
<td>
<code>targetKey</code><br>
<em>
github.com/kluctl/kluctl/v2/pkg/types/result.TargetKey
</em>
</td>
<td>
<em>(Optional)</em>
</td>
</tr>
<tr>
<td>
<code>lastObjectsHash</code><br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
</td>
</tr>
<tr>
<td>
<code>lastManualObjectsHash</code><br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
</td>
</tr>
<tr>
<td>
<code>lastPrepareError</code><br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
</td>
</tr>
<tr>
<td>
<code>lastDeployError</code><br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
</td>
</tr>
<tr>
<td>
<code>lastValidateError</code><br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
</td>
</tr>
<tr>
<td>
<code>lastDriftDetectionError</code><br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
</td>
</tr>
<tr>
<td>
<code>lastDeployResult</code><br>
<em>
k8s.io/apimachinery/pkg/runtime.RawExtension
</em>
</td>
<td>
<em>(Optional)</em>
<p>LastDeployResult is the result summary of the last deploy command</p>
</td>
</tr>
<tr>
<td>
<code>lastValidateResult</code><br>
<em>
k8s.io/apimachinery/pkg/runtime.RawExtension
</em>
</td>
<td>
<em>(Optional)</em>
<p>LastValidateResult is the result summary of the last validate command</p>
</td>
</tr>
<tr>
<td>
<code>lastDriftDetectionResult</code><br>
<em>
k8s.io/apimachinery/pkg/runtime.RawExtension
</em>
</td>
<td>
<p>LastDriftDetectionResult is the result of the last drift detection command
optional</p>
</td>
</tr>
<tr>
<td>
<code>lastDriftDetectionResultMessage</code><br>
<em>
string
</em>
</td>
<td>
<p>LastDriftDetectionResultMessage contains a short message that describes the drift
optional</p>
</td>
</tr>
</tbody>
</table>
</div>
</div>
<h3 id="gitops.kluctl.io/v1beta1.KubeConfig">KubeConfig
</h3>
<p>
(<em>Appears on:</em>
<a href="#gitops.kluctl.io/v1beta1.KluctlDeploymentSpec">KluctlDeploymentSpec</a>)
</p>
<p>KubeConfig references a Kubernetes secret that contains a kubeconfig file.</p>
<div class="md-typeset__scrollwrap">
<div class="md-typeset__table">
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>secretRef</code><br>
<em>
<a href="#gitops.kluctl.io/v1beta1.SecretKeyReference">
SecretKeyReference
</a>
</em>
</td>
<td>
<p>SecretRef holds the name of a secret that contains a key with
the kubeconfig file as the value. If no key is set, the key will default
to &lsquo;value&rsquo;. The secret must be in the same namespace as
the Kustomization.
It is recommended that the kubeconfig is self-contained, and the secret
is regularly updated if credentials such as a cloud-access-token expire.
Cloud specific <code>cmd-path</code> auth helpers will not function without adding
binaries and credentials to the Pod that is responsible for reconciling
the KluctlDeployment.</p>
</td>
</tr>
</tbody>
</table>
</div>
</div>
<h3 id="gitops.kluctl.io/v1beta1.LocalObjectReference">LocalObjectReference
</h3>
<p>
(<em>Appears on:</em>
<a href="#gitops.kluctl.io/v1beta1.Decryption">Decryption</a>, 
<a href="#gitops.kluctl.io/v1beta1.GitCredentials">GitCredentials</a>, 
<a href="#gitops.kluctl.io/v1beta1.HelmCredentials">HelmCredentials</a>, 
<a href="#gitops.kluctl.io/v1beta1.ProjectSource">ProjectSource</a>)
</p>
<div class="md-typeset__scrollwrap">
<div class="md-typeset__table">
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>name</code><br>
<em>
string
</em>
</td>
<td>
<p>Name of the referent.</p>
</td>
</tr>
</tbody>
</table>
</div>
</div>
<h3 id="gitops.kluctl.io/v1beta1.ProjectSource">ProjectSource
</h3>
<p>
(<em>Appears on:</em>
<a href="#gitops.kluctl.io/v1beta1.KluctlDeploymentSpec">KluctlDeploymentSpec</a>)
</p>
<div class="md-typeset__scrollwrap">
<div class="md-typeset__table">
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>url</code><br>
<em>
github.com/kluctl/kluctl/v2/pkg/types.GitUrl
</em>
</td>
<td>
<p>Url specifies the Git url where the project source is located</p>
</td>
</tr>
<tr>
<td>
<code>ref</code><br>
<em>
github.com/kluctl/kluctl/v2/pkg/types.GitRef
</em>
</td>
<td>
<em>(Optional)</em>
<p>Ref specifies the branch, tag or commit that should be used. If omitted, the default branch of the repo is used.</p>
</td>
</tr>
<tr>
<td>
<code>path</code><br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Path specifies the sub-directory to be used as project directory</p>
</td>
</tr>
<tr>
<td>
<code>secretRef</code><br>
<em>
<a href="#gitops.kluctl.io/v1beta1.LocalObjectReference">
LocalObjectReference
</a>
</em>
</td>
<td>
<p>SecretRef specifies the Secret containing authentication credentials for
See GitCredentials.SecretRef for details
DEPRECATED this field is deprecated and will be removed in a future version of the controller. Use Credentials
instead.
WARNING using this field causes the controller to pass http basic auth credentials to ALL repositories involved.
Use Credentials with a proper Host field instead.</p>
</td>
</tr>
<tr>
<td>
<code>credentials</code><br>
<em>
<a href="#gitops.kluctl.io/v1beta1.GitCredentials">
[]GitCredentials
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Credentials specifies a list of secrets with credentials</p>
</td>
</tr>
</tbody>
</table>
</div>
</div>
<h3 id="gitops.kluctl.io/v1beta1.SafeDuration">SafeDuration
</h3>
<p>
(<em>Appears on:</em>
<a href="#gitops.kluctl.io/v1beta1.KluctlDeploymentSpec">KluctlDeploymentSpec</a>)
</p>
<div class="md-typeset__scrollwrap">
<div class="md-typeset__table">
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>Duration</code><br>
<em>
<a href="https://godoc.org/k8s.io/apimachinery/pkg/apis/meta/v1#Duration">
Kubernetes meta/v1.Duration
</a>
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
</div>
</div>
<h3 id="gitops.kluctl.io/v1beta1.SecretKeyReference">SecretKeyReference
</h3>
<p>
(<em>Appears on:</em>
<a href="#gitops.kluctl.io/v1beta1.KubeConfig">KubeConfig</a>)
</p>
<p>SecretKeyReference contains enough information to locate the referenced Kubernetes Secret object in the same
namespace. Optionally a key can be specified.
Use this type instead of core/v1 SecretKeySelector when the Key is optional and the Optional field is not
applicable.</p>
<div class="md-typeset__scrollwrap">
<div class="md-typeset__table">
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>name</code><br>
<em>
string
</em>
</td>
<td>
<p>Name of the Secret.</p>
</td>
</tr>
<tr>
<td>
<code>key</code><br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Key in the Secret, when not specified an implementation-specific default key is used.</p>
</td>
</tr>
</tbody>
</table>
</div>
</div>
<div class="admonition note">
<p class="last">This page was automatically generated with <code>gen-crd-api-reference-docs</code></p>
</div>
