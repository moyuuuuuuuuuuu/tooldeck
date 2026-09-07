ARG BASE_IMAGE=node:22-bookworm-slim
FROM ${BASE_IMAGE}
RUN apt-get update && apt-get install -y --no-install-recommends socat ca-certificates build-essential python3 && rm -rf /var/lib/apt/lists/*
COPY runtimes/entrypoint.sh /entrypoint.sh
RUN chmod 755 /entrypoint.sh
ENV TOOLDECK_RUNTIME=node
ENTRYPOINT ["/entrypoint.sh"]

COPY runtimes/build.sh /build.sh
RUN chmod 755 /build.sh
