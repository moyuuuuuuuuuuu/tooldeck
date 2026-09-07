ARG BASE_IMAGE=python:3.12-slim-bookworm
FROM ${BASE_IMAGE}
RUN apt-get update && apt-get install -y --no-install-recommends socat ca-certificates build-essential && rm -rf /var/lib/apt/lists/*
COPY runtimes/entrypoint.sh /entrypoint.sh
RUN chmod 755 /entrypoint.sh
ENV TOOLDECK_RUNTIME=python
ENTRYPOINT ["/entrypoint.sh"]

COPY runtimes/build.sh /build.sh
RUN chmod 755 /build.sh
