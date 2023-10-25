<!-- This comment is uncommented when auto-synced to www-kluctl.io

---
title: "gitops deploy"
linkTitle: "gitops deploy"
weight: 10
description: >
    webui command
---
-->

## Command
<!-- BEGIN SECTION "gitops deploy" "Usage" false -->
Usage: kluctl gitops deploy [flags]

Trigger a GitOps deployment
This command will trigger an existing KluctlDeployment to perform a reconciliation loop with a forced deployment.
It does this by setting the annotation 'kluctl.io/request-deploy' to the current time.

You can override many deployment relevant fields, see the list of command flags for details.

<!-- END SECTION -->

## Arguments

The following arguments are available:
<!-- BEGIN SECTION "gitops deploy" "GitOps arguments" true -->
```
GitOps arguments:
  Specify gitops flags.

      --context string                   Override the context to use.
  -l, --label-selector string            If specified, KluctlDeployments are searched and filtered by this label
                                         selector.
      --local-source-override-port int   Specifies the local port to which the source-override client should
                                         connect to when running the controller locally.
      --name string                      Specifies the name of the KluctlDeployment.
  -n, --namespace string                 Specifies the namespace of the KluctlDeployment. If omitted, the current
                                         namespace from your kubeconfig is used.

```
<!-- END SECTION -->
<!-- BEGIN SECTION "gitops deploy" "Misc arguments" true -->
```
Misc arguments:
  Command specific arguments.

      --no-obfuscate                Disable obfuscation of sensitive/secret data
  -o, --output-format stringArray   Specify output format and target file, in the format 'format=path'. Format can
                                    either be 'text' or 'yaml'. Can be specified multiple times. The actual format
                                    for yaml is currently not documented and subject to change.
      --short-output                When using the 'text' output format (which is the default), only names of
                                    changes objects are shown instead of showing all changes.

```
<!-- END SECTION -->
<!-- BEGIN SECTION "gitops deploy" "Command Results" true -->
```
Command Results:
  Configure how command results are stored.

      --command-result-namespace string   Override the namespace to be used when writing command results. (default
                                          "kluctl-results")

```
<!-- END SECTION -->
<!-- BEGIN SECTION "gitops deploy" "Log arguments" true -->
```
Log arguments:
  Configure logging.

      --log-grouping-time duration   Logs are by default grouped by time passed, meaning that they are printed in
                                     batches to make reading them easier. This argument allows to modify the
                                     grouping time. (default 1s)
      --log-since duration           Show logs since this time. (default 1m0s)
      --log-time                     If enabled, adds timestamps to log lines

```
<!-- END SECTION -->
<!-- BEGIN SECTION "gitops deploy" "GitOps overrides" true -->
```
GitOps overrides:
  Override settings for GitOps deployments.

      --abort-on-error                         Abort deploying when an error occurs instead of trying the
                                               remaining deployments
  -a, --arg stringArray                        Passes a template argument in the form of name=value. Nested args
                                               can be set with the '-a my.nested.arg=value' syntax. Values are
                                               interpreted as yaml values, meaning that 'true' and 'false' will
                                               lead to boolean values and numbers will be treated as numbers. Use
                                               quotes if you want these to be treated as strings. If the value
                                               starts with @, it is treated as a file, meaning that the contents
                                               of the file will be loaded and treated as yaml.
      --args-from-file stringArray             Loads a yaml file and makes it available as arguments, meaning that
                                               they will be available thought the global 'args' variable.
      --dry-run                                Performs all kubernetes API calls in dry-run mode.
      --exclude-deployment-dir stringArray     Exclude deployment dir. The path must be relative to the root
                                               deployment project. Exclusion has precedence over inclusion, same
                                               as in --exclude-tag
  -E, --exclude-tag stringArray                Exclude deployments with given tag. Exclusion has precedence over
                                               inclusion, meaning that explicitly excluded deployments will always
                                               be excluded even if an inclusion rule would match the same deployment.
  -F, --fixed-image stringArray                Pin an image to a given version. Expects
                                               '--fixed-image=image<:namespace:deployment:container>=result'
      --fixed-images-file existingfile         Use .yaml file to pin image versions. See output of list-images
                                               sub-command or read the documentation for details about the output
                                               format
      --force-apply                            Force conflict resolution when applying. See documentation for details
      --force-replace-on-error                 Same as --replace-on-error, but also try to delete and re-create
                                               objects. See documentation for more details.
      --include-deployment-dir stringArray     Include deployment dir. The path must be relative to the root
                                               deployment project.
  -I, --include-tag stringArray                Include deployments with given tag.
      --local-git-group-override stringArray   Same as --local-git-override, but for a whole group prefix instead
                                               of a single repository. All repositories that have the given prefix
                                               will be overridden with the given local path and the repository
                                               suffix appended. For example,
                                               'gitlab.com/some-org/sub-org=/local/path/to/my-forks' will override
                                               all repositories below 'gitlab.com/some-org/sub-org/' with the
                                               repositories found in '/local/path/to/my-forks'. It will however
                                               only perform an override if the given repository actually exists
                                               locally and otherwise revert to the actual (non-overridden) repository.
      --local-git-override stringArray         Specify a single repository local git override in the form of
                                               'github.com/my-org/my-repo=/local/path/to/override'. This will
                                               cause kluctl to not use git to clone for the specified repository
                                               but instead use the local directory. This is useful in case you
                                               need to test out changes in external git repositories without
                                               pushing them.
      --local-oci-group-override stringArray   Same as --local-git-group-override, but for OCI repositories.
      --local-oci-override stringArray         Same as --local-git-override, but for OCI repositories.
      --no-wait                                Don't wait for objects readiness.
      --prune                                  Prune orphaned objects directly after deploying. See the help for
                                               the 'prune' sub-command for details.
      --replace-on-error                       When patching an object fails, try to replace it. See documentation
                                               for more details.
  -t, --target string                          Target name to run command for. Target must exist in .kluctl.yaml.
      --target-context string                  Overrides the context name specified in the target. If the selected
                                               target does not specify a context or the no-name target is used,
                                               --context will override the currently active context.
  -T, --target-name-override string            Overrides the target name. If -t is used at the same time, then the
                                               target will be looked up based on -t <name> and then renamed to the
                                               value of -T. If no target is specified via -t, then the no-name
                                               target is renamed to the value of -T.

```
<!-- END SECTION -->