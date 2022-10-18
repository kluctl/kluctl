---
title: "Validation"
linkTitle: "Validation"
weight: 3
description: >
    Annotations to control validation
---

The following annotations influence the [validate]({{< ref "docs/reference/commands/validate" >}}) command.

### validate-result.kluctl.io/xxx
If this annotation is found on a resource that is checked while validation, the key and the value of the annotation
are added to the validation result, which is then returned by the validate command.

The annotation key is dynamic, meaning that all annotations that begin with `validate-result.kluctl.io/` are taken
into account.
