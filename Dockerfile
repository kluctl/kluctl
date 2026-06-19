# We must use a glibc based distro due to embedded python not supporting musl libc for aarch64 (only amd64+musl is supported)
# see https://github.com/indygreg/python-build-standalone/issues/87
# use `docker buildx imagetools inspect cgr.dev/chainguard/wolfi-base:latest` to find latest sha256 of multiarch image
FROM --platform=$TARGETPLATFORM cgr.dev/chainguard/wolfi-base@sha256:e161445c05b19e668cb5cc44df2f0403329fd4f0ac892794255e328e760612a1

# See https://docs.docker.com/engine/reference/builder/#automatic-platform-args-in-the-global-scope
ARG TARGETPLATFORM

# We need git for kustomize to support overlays from git
# gpg is needed for SOPS. tini is needed to reap gpg-agent zombies
RUN apk update && apk add git tzdata gpg gpg-agent tini

# go-git is failing with no valid known_hosts file exists, even if we provide known hosts via code
RUN mkdir -p /etc/ssh && touch /etc/ssh/ssh_known_hosts

# Ensure helm is not trying to access /
ENV HELM_CACHE_HOME=/tmp/helm-cache
ENV KLUCTL_CACHE_DIR=/tmp/kluctl-cache

COPY bin/kluctl /usr/bin/

USER 65532:65532

ENTRYPOINT ["/usr/bin/kluctl"]
