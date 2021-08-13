# Deployment tags

Every kustomize deployment has a set of tags assigned to it. These tags are defined in multiple places, which is
documented in [deployments](./deployments.md). Look for the `tags` field, which is available in multiple places per
deployment project.

Tags are useful when only one or more specific kustomize deployments need to be deployed or deleted.

## Default tags

[kustomizeDirs](./deployments.md#kustomizedirs) and [includes](./deployments.md#includes) in deployment projects can
have an optional list of tags assigned.

If this list is completely omitted, one single entry is added by default. This single entry equals to the last element
of the `path` in the `kustomizeDirs`/`includes` entry.

Consider the following example:

```yaml
kustomizeDirs:
  - path: nginx
  - path: some/subdir
```

In this example, two kustomize deployments are defined. The first would get the tag `nginx` while the second
would get the tag `subdir`.

In most cases this heuristic is enough to get proper tags with which you can work. It might however lead to strange
or even conflicting tags (e.g. `subdir` is really a bad tag), in which case you'd have to explicitly set tags.

## Tag inheritance

Deployment projects and kustomize deployments inherit the tags of their parents. For example, if a deployment project
has a [tags](./deployments.md#tags-2) field defined, all `kustomizeDirs` entries and all `includes` entries would
inherit all these tags. Also, the sub-deployment projects included via `includes` inherit the tags of the `includes`
entry (which might have been inherited as described before), leading to further inheritance by deeper `kustomizeDirs`
and `includes`.

Consider the following example `deployment.yml`:

```yaml
includes:
  - path: sub-deployment1
    tags:
      - tag1
      - tag2
  - path: sub-deployment2
    tags:
      - tag3
      - tag4
  - path: subdir/subsub
```

Any kustomize deployment found in `sub-deployment1` would now inherit `tag1` and `tag2`. If `sub-deployment1` performs
any further includes, these would also inherit these two tags. Inheriting is additive and recursive.

The last sub-deployment project in the example is subject to the same default-tags logic as described
in [Default tags](#default-tags), meaning that it will get the default tag `subsub`.

## Deploying with tag inclusion/exclusion

Special care needs to be taken when trying to deploy only a specific part of your deployment which requires some base
resources to be deployed as well.

Imagine a large deployment is able to deploy 10 applications, but you only want to deploy one of them. When using tags
to achieve this, there might be some base resources (e.g. Namespaces) which are needed no matter if everything or just
this single application is deployed. In that case, you'd need to set [alwaysDeploy](./deployments.md#alwaysdeploy)
to `true`.

## Deleting with tag inclusion/exclusion

Also, in most cases, even more special care has to be taken for the same types of resources as decribed before.

Imagine a kustomize deployment being responsible for namespaces deployments. If you now want to delete everything except
deployments that have the `persistency` tag assigned, the exclusion logic would NOT exclude deletion of the namespace.
This would ultimately lead to everything being deleted, and the exclusion tag having no effect.

In such a case, you'd need to set [skipDeleteIfTags](./deployments.md#skipdeleteiftags) to `true` as well.

In most cases, setting `alwaysDeploy` to `true` also requires setting `skipDeleteIfTags` to `true`.
