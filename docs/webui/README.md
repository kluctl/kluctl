<!-- This comment is uncommented when auto-synced to www-kluctl.io

---
title: "Kluctl Webui"
linkTitle: "Kluctl Webui"
description: "Kluctl Webui documentation."
weight: 30
---
-->

# Kluctl Webui

The Kluctl Webui is a powerful UI which allows you to monitor and control your Kluctl GitOps deployments.

You can [run it locally](./running-locally.md) or [install](./installation.md) it to your Kubernetes cluster.

## State of the Webui

Please note that the Kluctl Webui is still in early stage of development, missing many planned features. It might
also contain bugs and be unstable in some situations. If you encounter any such problems, please report these
to https://github.com/kluctl/kluctl/issues.

## Screenshots

### Projects overview

The **Projects** view groups GitOps deployments by repository and project directory. Search by project or deployment, or filter for errors, warnings, and drift. Open a deployment to inspect its current state, validation, and configuration. Recorded CLI targets are labeled separately.

![Projects view showing grouped deployments with ready, error, warning, drift, and suspended statuses](https://kluctl.io/images/webui/targets.png)

### Changes in a recorded run

Open **Run history**, select a run, and choose **Changes** to see resource diffs. This recorded run changed `spec.replicas` from `2` to `3`.

![Changes tab for nginx-production showing the replica count changing from 2 to 3](https://kluctl.io/images/webui/targets-result.png)

Filter the history by command or outcome, or search by commit, ref, or run ID. **Newer** and **Older** step through retained runs. Copy the browser URL to share the selected run. Live GitOps targets also expose controller logs when available.

## Static snapshots

Use `kluctl webui build --path <directory> --max-results <count>` to export a static snapshot with history. Exports are read-only and may contain sensitive deployment data.
