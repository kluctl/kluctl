<!-- This comment is uncommented when auto-synced to www-kluctl.io

---
title: "Common Arguments"
linkTitle: "Common Arguments"
weight: 1
description: >
    Common arguments
---
-->

# Common Arguments

A few sets of arguments are common between multiple commands. These arguments are still part of the command itself and
must be placed *after* the command name.

## Global arguments

These arguments are available for all commands.

<!-- BEGIN SECTION "deploy" "Global arguments" true -->
```
Global arguments:
      --cpu-profile string   Enable CPU profiling and write the result to the given path
      --debug                Enable debug logging
      --no-color             Disable colored output
      --no-update-check      Disable update check on startup

```
<!-- END SECTION -->

## Project arguments

These arguments are available for all commands that are based on a Kluctl project.
They control where and how to load the kluctl project and deployment project.

<!-- BEGIN SECTION "deploy" "Project arguments" true -->
```
Project arguments:
  Define where and how to load the kluctl project and its components from.

  -a, --arg stringArray                      Passes a template argument in the form of name=value. Nested args can
                                             be set with the '-a my.nested.arg=value' syntax. Values are
                                             interpreted as yaml values, meaning that 'true' and 'false' will lead
                                             to boolean values and numbers will be treated as numbers. Use quotes
                                             if you want these to be treated as strings. If the value starts with
                                             @, it is treated as a file, meaning that the contents of the file
                                             will be loaded and treated as yaml.
      --args-from-file stringArray           Loads a yaml file and makes it available as arguments, meaning that
                                             they will be available thought the global 'args' variable.
      --context string                       Overrides the context name specified in the target. If the selected
                                             target does not specify a context or the no-name target is used,
                                             --context will override the currently active context.
      --git-cache-update-interval duration   Specify the time to wait between git cache updates. Defaults to not
                                             wait at all and always updating caches.
      --local-git-override stringArray       Specify a local git override in the form of
                                             'github.com:my-org/my-repo=/local/path/to/override'. This will cause
                                             kluctl to not use git to clone for the specified repository but
                                             instead use the local directory. This is useful in case you need to
                                             test out changes in external git repositories without pushing them.
                                             To only override a single branch of the repo, use
                                             'github.com:my-org/my-repo:my-branch=/local/path/to/override'
  -c, --project-config existingfile          Location of the .kluctl.yaml config file. Defaults to
                                             $PROJECT/.kluctl.yaml
  -t, --target string                        Target name to run command for. Target must exist in .kluctl.yaml.
  -T, --target-name-override string          Overrides the target name. If -t is used at the same time, then the
                                             target will be looked up based on -t <name> and then renamed to the
                                             value of -T. If no target is specified via -t, then the no-name
                                             target is renamed to the value of -T.
      --timeout duration                     Specify timeout for all operations, including loading of the project,
                                             all external api calls and waiting for readiness. (default 10m0s)

```
<!-- END SECTION -->

## Image arguments

These arguments are available on some target based commands.
They control image versions requested by `images.get_image(...)` [calls](../deployments/images#imagesget_image).

<!-- BEGIN SECTION "deploy" "Image arguments" true -->
```
Image arguments:
  Control fixed images and update behaviour.

  -F, --fixed-image stringArray          Pin an image to a given version. Expects
                                         '--fixed-image=image<:namespace:deployment:container>=result'
      --fixed-images-file existingfile   Use .yaml file to pin image versions. See output of list-images
                                         sub-command or read the documentation for details about the output format
      --offline-images                   Omit contacting image registries and do not query for latest image tags.
  -u, --update-images                    This causes kluctl to prefer the latest image found in registries, based
                                         on the 'latest_image' filters provided to 'images.get_image(...)' calls.
                                         Use this flag if you want to update to the latest versions/tags of all
                                         images. '-u' takes precedence over '--fixed-image/--fixed-images-file',
                                         meaning that the latest images are used even if an older image is given
                                         via fixed images.

```
<!-- END SECTION -->

## Inclusion/Exclusion arguments

These arguments are available for some target based commands.
They control inclusion/exclusion based on tags and deployment item pathes.

<!-- BEGIN SECTION "deploy" "Inclusion/Exclusion arguments" true -->
```
Inclusion/Exclusion arguments:
  Control inclusion/exclusion.

      --exclude-deployment-dir stringArray   Exclude deployment dir. The path must be relative to the root
                                             deployment project. Exclusion has precedence over inclusion, same as
                                             in --exclude-tag
  -E, --exclude-tag stringArray              Exclude deployments with given tag. Exclusion has precedence over
                                             inclusion, meaning that explicitly excluded deployments will always
                                             be excluded even if an inclusion rule would match the same deployment.
      --include-deployment-dir stringArray   Include deployment dir. The path must be relative to the root
                                             deployment project.
  -I, --include-tag stringArray              Include deployments with given tag.

```
<!-- END SECTION -->
