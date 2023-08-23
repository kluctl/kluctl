<!-- This comment is uncommented when auto-synced to www-kluctl.io

---
title: Running locally
linkTitle: Running locally
description: Running the Kluctl Webui locally
weight: 10
---
-->

# Running locally

The Kluctl Webui can be run locally by simply invoking [`kluctl webui run`](../commands/webui-run.md).
It will by default connect to your local Kubeconfig Context and expose the Webui on `localhost`. It will also open
the browser for you.

## Multiple Clusters

The Webui can already handle multiple clusters. Simply pass `--context <context-name>` multiple times to `kluctl webui run`.
This will cause the Webui to listen for status updates on all passed clusters.

As noted in [State of the Webui](./README.md#state-of-the-webui), the Webui is still in early stage and thus currently
lacks sorting and filtering for clusters. This will be implemented in future releases.
