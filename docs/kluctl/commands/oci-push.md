<!-- This comment is uncommented when auto-synced to www-kluctl.io

---
title: "oci push"
linkTitle: "oci push"
weight: 10
description: >
    oci push command
---
-->

## Command
<!-- BEGIN SECTION "oci push" "Usage" false -->
Usage: kluctl oci push [flags]

Push to an oci repository
The push command creates a tarball from the current project and uploads the
artifact to an OCI repository.

<!-- END SECTION -->

## Arguments

The following arguments are available:
<!-- BEGIN SECTION "oci push" "Misc arguments" true -->
```
Misc arguments:
  Command specific arguments.

      --annotation stringArray                          Set custom OCI annotations in the format '<key>=<value>'
      --ignore-path stringArray                         set paths to ignore in .gitignore format.
      --output string                                   the format in which the artifact digest should be printed,
                                                        can be 'json' or 'yaml'
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
      --timeout duration                                Specify timeout for all operations, including loading of
                                                        the project, all external api calls and waiting for
                                                        readiness. (default 10m0s)
      --url string                                      Specifies the artifact URL. This argument is required.

```
<!-- END SECTION -->

