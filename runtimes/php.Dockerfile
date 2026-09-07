ARG BASE_IMAGE=php:8.3-cli-bookworm
FROM ${BASE_IMAGE}
RUN apt-get update && apt-get install -y --no-install-recommends socat ca-certificates git unzip && rm -rf /var/lib/apt/lists/*
COPY runtimes/entrypoint.sh /entrypoint.sh
RUN chmod 755 /entrypoint.sh
ENV TOOLDECK_RUNTIME=php
ENTRYPOINT ["/entrypoint.sh"]

COPY runtimes/build.sh /build.sh
RUN chmod 755 /build.sh
COPY --from=composer:2 /usr/bin/composer /usr/local/bin/composer
