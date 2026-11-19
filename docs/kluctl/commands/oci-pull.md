<!-- This comment is uncommented when auto-synced to www-kluctl.io

---
title: "oci pull"
linkTitle: "oci pull"
weight: 11
description: >
    oci pull command
---
-->

## Command
<!-- BEGIN SECTION "oci pull" "Usage" false -->
Usage: kluctl oci pull [flags]

Pull from an OCI repository
The pull command downloads a project artifact from an OCI repository and
extracts it into the specified directory.

<!-- END SECTION -->

For example, to extract an artifact created by [`kluctl oci push`](./oci-push.md):

```sh
kluctl oci pull --url oci://registry.example.com/team/project:v1 --output-dir ./project
```

The URL accepts a tag or a digest (`@sha256:…`). If neither is specified, the
`latest` tag is used. The destination is created if needed, and an existing
non-empty directory is rejected. If extraction fails, partial files may remain.

## Arguments
The following sets of arguments are available:
1. [registry arguments](./common-arguments.md#registry-arguments)

<!-- BEGIN SECTION "oci pull" "Misc arguments" true -->
```
Misc arguments:
  Command specific arguments.

      --output-dir string   Specifies the directory to extract the artifact into. It must be empty or not exist.
      --timeout duration    Specifies the timeout for pulling the artifact. (default 10m0s)
      --url string          Specifies the artifact URL.

```
<!-- END SECTION -->
