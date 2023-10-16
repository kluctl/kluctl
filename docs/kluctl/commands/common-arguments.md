<!-- This comment is uncommented when auto-synced to www-kluctl.io

---
title: "Common Arguments"
linkTitle: "Common Arguments"
weight: 1
description: >
    Common arguments
---
-->

# Common Arguments

A few sets of arguments are common between multiple commands. These arguments are still part of the command itself and
must be placed *after* the command name.

## Global arguments

These arguments are available for all commands.

<!-- BEGIN SECTION "deploy" "Global arguments" true -->
```
Global arguments:
      --cpu-profile string       Enable CPU profiling and write the result to the given path
      --debug                    Enable debug logging
      --gops-agent               Start gops agent in the background
      --gops-agent-addr string   Specify the address:port to use for the gops agent (default "127.0.0.1:0")
      --no-color                 Disable colored output
      --no-update-check          Disable update check on startup

```
<!-- END SECTION -->

## Project arguments

These arguments are available for all commands that are based on a Kluctl project.
They control where and how to load the kluctl project and deployment project.

<!-- BEGIN SECTION "deploy" "Project arguments" true -->
```
Project arguments:
  Define where and how to load the kluctl project and its components from.

  -a, --arg stringArray                        Passes a template argument in the form of name=value. Nested args
                                               can be set with the '-a my.nested.arg=value' syntax. Values are
                                               interpreted as yaml values, meaning that 'true' and 'false' will
                                               lead to boolean values and numbers will be treated as numbers. Use
                                               quotes if you want these to be treated as strings. If the value
                                               starts with @, it is treated as a file, meaning that the contents
                                               of the file will be loaded and treated as yaml.
      --args-from-file stringArray             Loads a yaml file and makes it available as arguments, meaning that
                                               they will be available thought the global 'args' variable.
      --context string                         Overrides the context name specified in the target. If the selected
                                               target does not specify a context or the no-name target is used,
                                               --context will override the currently active context.
      --git-cache-update-interval duration     Specify the time to wait between git cache updates. Defaults to not
                                               wait at all and always updating caches.
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
  -c, --project-config existingfile            Location of the .kluctl.yaml config file. Defaults to
                                               $PROJECT/.kluctl.yaml
      --project-dir existingdir                Specify the project directory. Defaults to the current working
                                               directory.
  -t, --target string                          Target name to run command for. Target must exist in .kluctl.yaml.
  -T, --target-name-override string            Overrides the target name. If -t is used at the same time, then the
                                               target will be looked up based on -t <name> and then renamed to the
                                               value of -T. If no target is specified via -t, then the no-name
                                               target is renamed to the value of -T.
      --timeout duration                       Specify timeout for all operations, including loading of the
                                               project, all external api calls and waiting for readiness. (default
                                               10m0s)

```
<!-- END SECTION -->

## Image arguments

These arguments are available on some target based commands.
They control image versions requested by `images.get_image(...)` [calls](../deployments/images.md#imagesget_image).

<!-- BEGIN SECTION "deploy" "Image arguments" true -->
```
Image arguments:
  Control fixed images and update behaviour.

  -F, --fixed-image stringArray          Pin an image to a given version. Expects
                                         '--fixed-image=image<:namespace:deployment:container>=result'
      --fixed-images-file existingfile   Use .yaml file to pin image versions. See output of list-images
                                         sub-command or read the documentation for details about the output format

```
<!-- END SECTION -->

## Inclusion/Exclusion arguments

These arguments are available for some target based commands.
They control inclusion/exclusion based on tags and deployment item pathes.

<!-- BEGIN SECTION "deploy" "Inclusion/Exclusion arguments" true -->
```
Inclusion/Exclusion arguments:
  Control inclusion/exclusion.

      --exclude-deployment-dir stringArray   Exclude deployment dir. The path must be relative to the root
                                             deployment project. Exclusion has precedence over inclusion, same as
                                             in --exclude-tag
  -E, --exclude-tag stringArray              Exclude deployments with given tag. Exclusion has precedence over
                                             inclusion, meaning that explicitly excluded deployments will always
                                             be excluded even if an inclusion rule would match the same deployment.
      --include-deployment-dir stringArray   Include deployment dir. The path must be relative to the root
                                             deployment project.
  -I, --include-tag stringArray              Include deployments with given tag.

```
<!-- END SECTION -->

## Command Results arguments

These arguments control how command results are stored.

<!-- BEGIN SECTION "deploy" "Command Results" true -->
```
Command Results:
  Configure how command results are stored.

      --command-result-namespace string   Override the namespace to be used when writing command results. (default
                                          "kluctl-results")
      --force-write-command-result        Force writing of command results, even if the command is run in dry-run mode.
      --keep-command-results-count int    Configure how many old command results to keep. (default 5)
      --keep-validate-results-count int   Configure how many old validate results to keep. (default 2)
      --write-command-result              Enable writing of command results into the cluster. This is enabled by
                                          default. (default true)

```
<!-- END SECTION -->

## Helm arguments

These arguments mainly control authentication to Helm repositories.

<!-- BEGIN SECTION "deploy" "Helm arguments" true -->
```
Helm arguments:
  Configure Helm authentication.

      --helm-ca-file stringArray                    Specify ca bundle certificate to use for Helm Repository
                                                    authentication. Must be in the form
                                                    --helm-ca-file=<host>/<path>=<filePath> or in the deprecated
                                                    form --helm-ca-file=<credentialsId>:<filePath>, where
                                                    <credentialsId> must match the id specified in the helm-chart.yaml.
      --helm-cert-file stringArray                  Specify key to use for Helm Repository authentication. Must be
                                                    in the form --helm-cert-file=<host>/<path>=<filePath> or in
                                                    the deprecated form
                                                    --helm-cert-file=<credentialsId>:<filePath>, where
                                                    <credentialsId> must match the id specified in the helm-chart.yaml.
      --helm-creds stringArray                      This is a shortcut to --helm-username and --helm-password.
                                                    Must be in the form
                                                    --helm-creds=<host>/<path>=<username>:<password>, which
                                                    specifies the username and password for the same repository.
      --helm-insecure-skip-tls-verify stringArray   Controls skipping of TLS verification. Must be in the form
                                                    --helm-insecure-skip-tls-verify=<host>/<path> or in the
                                                    deprecated form
                                                    --helm-insecure-skip-tls-verify=<credentialsId>, where
                                                    <credentialsId> must match the id specified in the helm-chart.yaml.
      --helm-key-file stringArray                   Specify client certificate to use for Helm Repository
                                                    authentication. Must be in the form
                                                    --helm-key-file=<host>/<path>=<filePath> or in the deprecated
                                                    form --helm-key-file=<credentialsId>:<filePath>, where
                                                    <credentialsId> must match the id specified in the helm-chart.yaml.
      --helm-password stringArray                   Specify password to use for Helm Repository authentication.
                                                    Must be in the form --helm-password=<host>/<path>=<password>
                                                    or in the deprecated form
                                                    --helm-password=<credentialsId>:<password>, where
                                                    <credentialsId> must match the id specified in the helm-chart.yaml.
      --helm-username stringArray                   Specify username to use for Helm Repository authentication.
                                                    Must be in the form --helm-username=<host>/<path>=<username>
                                                    or in the deprecated form
                                                    --helm-username=<credentialsId>:<username>, where
                                                    <credentialsId> must match the id specified in the helm-chart.yaml.

```
<!-- END SECTION -->

## Registry arguments

These arguments mainly control authentication to OCI based registries. This is used by the Helm integration and
by the OCI includes integration.

<!-- BEGIN SECTION "deploy" "Registry arguments" true -->
```
Registry arguments:
  Configure OCI registry authentication.

      --registry-auth stringArray                       Specify auth string to use for OCI authentication. Must be
                                                        in the form --registry-auth=<registry>/<repo>=<auth>.
      --registry-ca-file stringArray                    Specify CA bundle to use for https verification. Must be
                                                        in the form --registry-ca-file=<registry>/<repo>=<filePath>.
      --registry-cert-file stringArray                  Specify certificate to use for OCI authentication. Must be
                                                        in the form --registry-cert-file=<registry>/<repo>=<filePath>.
      --registry-creds stringArray                      This is a shortcut to --registry-username,
                                                        --registry-password and --registry-token. It can be
                                                        specified in two different forms. The first one is
                                                        --registry-creds=<registry>/<repo>=<username>:<password>,
                                                        which specifies the username and password for the same
                                                        registry. The second form is
                                                        --registry-creds=<registry>/<repo>=<token>, which
                                                        specifies a JWT token for the specified registry.
      --registry-identity-token stringArray             Specify identity token to use for OCI authentication. Must
                                                        be in the form
                                                        --registry-identity-token=<registry>/<repo>=<identity-token>.
      --registry-insecure-skip-tls-verify stringArray   Controls skipping of TLS verification. Must be in the form
                                                        --registry-insecure-skip-tls-verify=<registry>/<repo>.
      --registry-key-file stringArray                   Specify key to use for OCI authentication. Must be in the
                                                        form --registry-key-file=<registry>/<repo>=<filePath>.
      --registry-password stringArray                   Specify password to use for OCI authentication. Must be in
                                                        the form --registry-password=<registry>/<repo>=<password>.
      --registry-plain-http stringArray                 Forces the use of http (no TLS). Must be in the form
                                                        --registry-plain-http=<registry>/<repo>.
      --registry-token stringArray                      Specify registry token to use for OCI authentication. Must
                                                        be in the form --registry-token=<registry>/<repo>=<token>.
      --registry-username stringArray                   Specify username to use for OCI authentication. Must be in
                                                        the form --registry-username=<registry>/<repo>=<username>.

```
<!-- END SECTION -->