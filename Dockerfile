# We must use a glibc based distro due to embedded python not supporting musl libc for aarch64
FROM debian:bullseye-slim
COPY kluctl /usr/bin/
ENTRYPOINT ["/usr/bin/kluctl"]
