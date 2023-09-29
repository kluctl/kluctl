# We must use a glibc based distro due to embedded python not supporting musl libc for aarch64 (only amd64+musl is supported)
# see https://github.com/indygreg/python-build-standalone/issues/87
# use `docker buildx imagetools inspect  cgr.dev/chainguard/wolfi-base:latest` to find latest sha256 of multiarch image
ARG WOLFI_DIGEST=sha256:ccc5551b5dd1fdcff5fc76ac1605b4c217f77f43410e0bd8a56599d6504dbbdd
FROM cgr.dev/chainguard/wolfi-base@$WOLFI_DIGEST

# We need git for kustomize to support overlays from git
RUN apk update && apk add git tzdata

# Ensure helm is not trying to access /
ENV HELM_CACHE_HOME=/tmp/helm-cache

COPY bin/kluctl /usr/bin/

USER 65532:65532

ENTRYPOINT ["/usr/bin/kluctl"]
