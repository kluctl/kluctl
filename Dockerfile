FROM alpine:3.15 as builder

RUN apk add --no-cache ca-certificates curl

ARG ARCH=linux-amd64

# We must use a glibc based distro due to embedded python not supporting musl libc for aarch64
FROM debian:bullseye-slim
COPY --from=builder /helm /usr/bin
COPY kluctl /usr/bin/
ENTRYPOINT ["/usr/bin/kluctl"]
