<!-- This comment is uncommented when auto-synced to www-kluctl.io

---
title: "Validation"
linkTitle: "Validation"
weight: 3
description: >
    Annotations to control validation
---
-->

# Validation

The following annotations influence the [validate](../../commands/validate) command.

### validate-result.kluctl.io/xxx
If this annotation is found on a resource that is checked while validation, the key and the value of the annotation
are added to the validation result, which is then returned by the validate command.

The annotation key is dynamic, meaning that all annotations that begin with `validate-result.kluctl.io/` are taken
into account.

### kluctl.io/validate-ignore
If this annotation is set to `true`, the object will be ignored while `kluctl validate` is run.