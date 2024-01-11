# We must use a glibc based distro due to embedded python not supporting musl libc for aarch64 (only amd64+musl is supported)
# see https://github.com/indygreg/python-build-standalone/issues/87
# use `docker buildx imagetools inspect  cgr.dev/chainguard/wolfi-base:latest` to find latest sha256 of multiarch image
ARG WOLFI_DIGEST=sha256:5bab300be31cd472c6f45ba7c059415b0bdeb0a49b32ecc12060394d642fd192
FROM cgr.dev/chainguard/wolfi-base@$WOLFI_DIGEST

# We need git for kustomize to support overlays from git
RUN apk update && apk add git tzdata

# Ensure helm is not trying to access /
ENV HELM_CACHE_HOME=/tmp/helm-cache
ENV KLUCTL_CACHE_DIR=/tmp/kluctl-cache

COPY bin/kluctl /usr/bin/

USER 65532:65532

ENTRYPOINT ["/usr/bin/kluctl"]
