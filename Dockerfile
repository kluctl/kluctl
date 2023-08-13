# We must use a glibc based distro due to embedded python not supporting musl libc for aarch64 (only amd64+musl is supported)
# see https://github.com/indygreg/python-build-standalone/issues/87
ARG WOLFI_DIGEST=sha256:e8411b9629bd13123a978a2d1a27c3ee64c7751d3032ce80ad6e673e329ef964
FROM cgr.dev/chainguard/wolfi-base@$WOLFI_DIGEST

# We need git for kustomize to support overlays from git
RUN apk add git tzdata

# Ensure helm is not trying to access /
ENV HELM_CACHE_HOME=/tmp/helm-cache

COPY bin/kluctl /usr/bin/

USER 65532:65532

ENTRYPOINT ["/usr/bin/kluctl"]
