<!-- This comment is uncommented when auto-synced to www-kluctl.io

---
title: Metrics of the KluctlDeployment Controller
linkTitle: KluctlDeployment Controller Metrics
description: KluctlDeployment documentation
weight: 20
---
-->

# Exported Metrics References

| Metrics name                | Type      | Description                                                                          |
|-----------------------------|-----------|--------------------------------------------------------------------------------------|
| deployment_duration_seconds | Histogram | How long a single deployment takes in seconds.                                       |
| number_of_changed_objects   | Gauge     | How many objects have been changed by a single deployment.                           |
| number_of_deleted_objects   | Gauge     | How many objects have been deleted by a single deployment.                           |
| number_of_errors            | Gauge     | How many errors are related to a single deployment.                                  |
| number_of_images            | Gauge     | Number of images of a single deployment.                                             |
| number_of_orphan_objects    | Gauge     | How many orphans are related to a single deployment.                                 |
| number_of_warnings          | Gauge     | How many warnings are related to a single deployment.                                |
| prune_duration_seconds      | Histogram | How long a single prune takes in seconds.                                            |
| validate_duration_seconds   | Histogram | How long a single validate takes in seconds.                                         |
| deployment_interval_seconds | Gauge     | The configured deployment interval of a single deployment.                           |
| dry_run_enabled             | Gauge     | Is dry-run enabled for a single deployment.                                          |
| last_object_status          | Gauge     | Last object status of a single deployment. Zero means failure and one means success. |
| prune_enabled               | Gauge     | Is pruning enabled for a single deployment.                                          |
| delete_enabled              | Gauge     | Is deletion enabled for a single deployment.                                         |
| source_spec                 | Gauge     | The configured source spec of a single deployment exported via labels.               |
