# Development

## Installing required dependencies

There are a number of dependencies required to be able to compile, run and test Kluctl:

- [Install Go](https://golang.org/doc/install)

In addition to the above, the following dependencies are also used by some of the `make` targets:

- `goreleaser` (latest)
- `setup-envtest` (latest)

If any of the above dependencies are not present on your system, the first invocation of a `make` target that requires them will install them.

## How to run the test suite

Prerequisites:
* Go >= 1.21

You can run the test suite by simply doing

```sh
make test
```

Tests are separated between unit tests (found in-tree next to the tested code) and e2e tests (found below `./e2e`).
Running `make test` will run all these tests. To only run unit tests, run `make test-unit`. To only run e2e tests, run
`make test-e2e`.

e2e tests rely on kubebuilders `envtest` package and thus also require `setup-envtest` and dependent binaries
(e.g. kube-apiserver) to be available as well. The Makefile will take care of downloading these if required.

## How to build Kluctl

Simply run `make build`, which will build the kluctl binary any put into `./bin/`. To use, either directly invoke it
with the relative or absolute path or update your `PATH` environment variable to point to the bin directory.

## Contributions

See [CONTRIBUTING](./CONTRIBUTING.md) for details.

## Releasing process

The release process is currently partly manual and partly automated. A maintainer has to create a version tag and
manually push it to GitHub. A GitHub workflow will then react to this tag by running `goreleaser`, which will then
create a draft release. The maintainer then has to manually update the pre-generated release notes and publish the
release.

This process is going to be modified in the future to contain more automation and more collaboration friendly processes.
