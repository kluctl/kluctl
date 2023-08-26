<!-- This comment is uncommented when auto-synced to www-kluctl.io

---
title: v1beta1 metrics
linkTitle: v1beta1 metrics
description: gitops.kluctl.io/v1beta1 metrics
weight: 10
---
-->

# Prometheus Metrics

The controller exports several metrics in the [OpenMetrics compatible format](https://github.com/OpenObservability/OpenMetrics/blob/main/specification/OpenMetrics.md).
They can be scraped by all sorts of monitoring solutions (e.g. Prometheus) or stored in a database. Because the
controller is based on [controller-runtime](https://github.com/kubernetes-sigs/controller-runtime), all
the [default metrics](https://book.kubebuilder.io/reference/metrics-reference.html) as well as the
following controller-specific custom metrics are exported:

- [kluctldeployment_controller](kluctldeployment_controller.md)
