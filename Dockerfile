FROM alpine:3.18.0

RUN apk add ca-certificates

COPY kluctl /usr/bin/
ENTRYPOINT ["/usr/bin/kluctl"]
