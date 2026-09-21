ARG BASE_IMAGE=golang:1.24-bookworm
FROM ${BASE_IMAGE}
ARG DEBIAN_MIRROR=mirrors.aliyun.com
RUN find /etc/apt -type f \( -name '*.list' -o -name '*.sources' \) -exec sed -i "s|deb.debian.org/debian-security|security.debian.org/debian-security|g; s|deb.debian.org/debian|${DEBIAN_MIRROR}/debian|g" {} +
RUN apt-get -o Acquire::Retries=3 update && apt-get -o Acquire::Retries=3 install -y --no-install-recommends socat ca-certificates && rm -rf /var/lib/apt/lists/*
COPY runtimes/entrypoint.sh /entrypoint.sh
RUN chmod 755 /entrypoint.sh
ENV TOOLDECK_RUNTIME=go CGO_ENABLED=0
ENTRYPOINT ["/entrypoint.sh"]

COPY runtimes/build.sh /build.sh
RUN chmod 755 /build.sh
