FROM alpine:3.17.1

RUN apk add ca-certificates

COPY kluctl /usr/bin/
ENTRYPOINT ["/usr/bin/kluctl"]
