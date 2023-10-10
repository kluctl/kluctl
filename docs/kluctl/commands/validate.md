<!-- This comment is uncommented when auto-synced to www-kluctl.io

---
title: "validate"
linkTitle: "validate"
weight: 10
description: >
    validate command
---
-->

## Command
<!-- BEGIN SECTION "validate" "Usage" false -->
Usage: kluctl validate [flags]

Validates the already deployed deployment
This means that all objects are retrieved from the cluster and checked for readiness.

TODO: This needs to be better documented!

<!-- END SECTION -->

## Arguments
The following sets of arguments are available:
1. [project arguments](./common-arguments.md#project-arguments)
1. [image arguments](./common-arguments.md#image-arguments)

In addition, the following arguments are available:
<!-- BEGIN SECTION "validate" "Misc arguments" true -->
```
Misc arguments:
  Command specific arguments.

      --helm-ca-file stringArray                        Specify ca bundle certificate to use for Helm Repository
                                                        authentication. Must be in the form
                                                        --helm-ca-file=<host>/<path>=<filePath> or in the
                                                        deprecated form --helm-ca-file=<credentialsId>:<filePath>,
                                                        where <credentialsId> must match the id specified in the
                                                        helm-chart.yaml.
      --helm-cert-file stringArray                      Specify key to use for Helm Repository authentication.
                                                        Must be in the form
                                                        --helm-cert-file=<host>/<path>=<filePath> or in the
                                                        deprecated form
                                                        --helm-cert-file=<credentialsId>:<filePath>, where
                                                        <credentialsId> must match the id specified in the
                                                        helm-chart.yaml.
      --helm-insecure-skip-tls-verify stringArray       Controls skipping of TLS verification. Must be in the form
                                                        --helm-insecure-skip-tls-verify=<host>/<path> or in the
                                                        deprecated form
                                                        --helm-insecure-skip-tls-verify=<credentialsId>, where
                                                        <credentialsId> must match the id specified in the
                                                        helm-chart.yaml.
      --helm-key-file stringArray                       Specify client certificate to use for Helm Repository
                                                        authentication. Must be in the form
                                                        --helm-key-file=<host>/<path>=<filePath> or in the
                                                        deprecated form
                                                        --helm-key-file=<credentialsId>:<filePath>, where
                                                        <credentialsId> must match the id specified in the
                                                        helm-chart.yaml.
      --helm-password stringArray                       Specify password to use for Helm Repository
                                                        authentication. Must be in the form
                                                        --helm-password=<host>/<path>=<password> or in the
                                                        deprecated form
                                                        --helm-password=<credentialsId>:<password>, where
                                                        <credentialsId> must match the id specified in the
                                                        helm-chart.yaml.
      --helm-username stringArray                       Specify username to use for Helm Repository
                                                        authentication. Must be in the form
                                                        --helm-username=<host>/<path>=<username> or in the
                                                        deprecated form
                                                        --helm-username=<credentialsId>:<username>, where
                                                        <credentialsId> must match the id specified in the
                                                        helm-chart.yaml.
  -o, --output stringArray                              Specify output target file. Can be specified multiple times
      --registry-auth stringArray                       Specify auth string to use for OCI authentication. Must be
                                                        in the form --registry-auth=<registry>/<repo>=<auth>.
      --registry-ca-file stringArray                    Specify CA bundle to use for https verification. Must be
                                                        in the form --registry-ca-file=<registry>/<repo>=<filePath>.
      --registry-cert-file stringArray                  Specify certificate to use for OCI authentication. Must be
                                                        in the form --registry-cert-file=<registry>/<repo>=<filePath>.
      --registry-identity-token stringArray             Specify auth string to use for OCI authentication. Must be
                                                        in the form
                                                        --registry-identity-token=<registry>/<repo>=<identity-token>.
      --registry-insecure-skip-tls-verify stringArray   Controls skipping of TLS verification. Must be in the form
                                                        --registry-insecure-skip-tls-verify=<registry>/<repo>.
      --registry-key-file stringArray                   Specify key to use for OCI authentication. Must be in the
                                                        form --registry-key-file=<registry>/<repo>=<filePath>.
      --registry-password stringArray                   Specify password to use for OCI authentication. Must be in
                                                        the form --registry-password=<registry>/<repo>=<password>.
      --registry-plain-http stringArray                 Forces the use of http (no TLS). Must be in the form
                                                        --registry-plain-http=<registry>/<repo>.
      --registry-token stringArray                      Specify auth string to use for OCI authentication. Must be
                                                        in the form --registry-token=<registry>/<repo>=<token>.
      --registry-username stringArray                   Specify username to use for OCI authentication. Must be in
                                                        the form --registry-username=<registry>/<repo>=<username>.
      --render-output-dir string                        Specifies the target directory to render the project into.
                                                        If omitted, a temporary directory is used.
      --sleep duration                                  Sleep duration between validation attempts (default 5s)
      --wait duration                                   Wait for the given amount of time until the deployment
                                                        validates
      --warnings-as-errors                              Consider warnings as failures

```
<!-- END SECTION -->
