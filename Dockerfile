# We must use a glibc based distro due to embedded python not supporting musl libc for aarch64
FROM debian:bullseye-slim

RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates && rm -rf /var/lib/apt/lists/*

COPY kluctl /usr/bin/
ENTRYPOINT ["/usr/bin/kluctl"]
