FROM alpine:3.18.0

RUN apk add ca-certificates

# We need git for kustomize to support overlays from git
RUN apk add git

# Ensure helm is not trying to access /
ENV HELM_CACHE_HOME=/tmp/helm-cache

ARG BIN_PATH=kluctl
COPY $BIN_PATH /usr/bin/

USER 65532:65532

ENTRYPOINT ["/usr/bin/kluctl"]
