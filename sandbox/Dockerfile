FROM alpine:3.24.1@sha256:28bd5fe8b56d1bd048e5babf5b10710ebe0bae67db86916198a6eec434943f8b

RUN apk add --no-cache python3~=3.14.7-r1 gcc~=15.2.0-r5 clang22~=22.1.3-r2 musl-dev~=1.2.6-r2 libstdc++~=15.2.0-r5

COPY --chmod=0755 sandbox/run.sh /run.sh

USER 65532:65532

ENTRYPOINT ["/bin/sh", "/run.sh"]
