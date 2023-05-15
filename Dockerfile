FROM alpine:3.18.0

RUN apk add ca-certificates

# We need git for kustomize to support overlays from git
RUN apk add git

# Ensure helm is not trying to access /
ENV HELM_CACHE_HOME=/tmp/helm-cache

COPY kluctl /usr/bin/
ENTRYPOINT ["/usr/bin/kluctl"]
