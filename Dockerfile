FROM alpine:3

RUN apk add -U ca-certificates tzdata && rm -Rf /var/cache/apk/*
COPY cm /

WORKDIR /root
ENTRYPOINT ["/cm"]
