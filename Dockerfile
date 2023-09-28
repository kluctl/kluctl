# We must use a glibc based distro due to embedded python not supporting musl libc for aarch64 (only amd64+musl is supported)
# see https://github.com/indygreg/python-build-standalone/issues/87
ARG WOLFI_DIGEST=sha256:0f5ba4905ca6ea9c43660cca233e663eb9bb21cbc6993656936d2aef80c43310
FROM cgr.dev/chainguard/wolfi-base@$WOLFI_DIGEST

# We need git for kustomize to support overlays from git
RUN apk update && apk add git tzdata

# Ensure helm is not trying to access /
ENV HELM_CACHE_HOME=/tmp/helm-cache

COPY bin/kluctl /usr/bin/

USER 65532:65532

ENTRYPOINT ["/usr/bin/kluctl"]
