FROM alpine:3.5

RUN apk add -U ca-certificates && rm -Rf /var/cache/apk/*
COPY cm /

ENTRYPOINT ["/cm"]
