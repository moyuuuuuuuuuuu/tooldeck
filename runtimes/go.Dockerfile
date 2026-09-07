ARG BASE_IMAGE=golang:1.24-bookworm
FROM ${BASE_IMAGE}
RUN apt-get update && apt-get install -y --no-install-recommends socat ca-certificates && rm -rf /var/lib/apt/lists/*
COPY runtimes/entrypoint.sh /entrypoint.sh
RUN chmod 755 /entrypoint.sh
ENV TOOLDECK_RUNTIME=go CGO_ENABLED=0
ENTRYPOINT ["/entrypoint.sh"]

COPY runtimes/build.sh /build.sh
RUN chmod 755 /build.sh
