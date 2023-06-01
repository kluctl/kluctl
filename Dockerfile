# We must use a glibc based distro due to embedded python not supporting musl libc for aarch64 (only amd64+musl is supported)
# see https://github.com/indygreg/python-build-standalone/issues/87
FROM debian:bullseye-slim

RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates && rm -rf /var/lib/apt/lists/*

# We need git for kustomize to support overlays from git
RUN apt-get update && apt-get install -y --no-install-recommends git && rm -rf /var/lib/apt/lists/*

# Ensure helm is not trying to access /
ENV HELM_CACHE_HOME=/tmp/helm-cache

ARG BIN_PATH=kluctl
COPY $BIN_PATH /usr/bin/

USER 65532:65532

ENTRYPOINT ["/usr/bin/kluctl"]
