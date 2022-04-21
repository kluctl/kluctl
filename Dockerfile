FROM alpine
COPY kluctl /
ENTRYPOINT ["/kluctl"]
