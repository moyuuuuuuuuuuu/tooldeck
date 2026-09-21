ARG BASE_IMAGE=php:8.3-cli-bookworm
FROM ${BASE_IMAGE}
ARG DEBIAN_MIRROR=mirrors.aliyun.com
RUN find /etc/apt -type f \( -name '*.list' -o -name '*.sources' \) -exec sed -i "s|deb.debian.org/debian-security|security.debian.org/debian-security|g; s|deb.debian.org/debian|${DEBIAN_MIRROR}/debian|g" {} +
RUN if grep -q '^VERSION_CODENAME=bullseye$' /etc/os-release; then find /etc/apt -type f \( -name '*.list' -o -name '*.sources' \) -exec sed -i 's|http://security.debian.org/debian-security|https://snapshot.debian.org/archive/debian-security/20260831T194148Z|g' {} +; printf 'Acquire::Check-Valid-Until "false";\n' > /etc/apt/apt.conf.d/99snapshot; fi
RUN apt-get -o Acquire::Retries=3 update && apt-get -o Acquire::Retries=3 install -y --no-install-recommends socat ca-certificates git unzip && rm -rf /var/lib/apt/lists/*
COPY runtimes/entrypoint.sh /entrypoint.sh
RUN chmod 755 /entrypoint.sh
ENV TOOLDECK_RUNTIME=php
ENTRYPOINT ["/entrypoint.sh"]

COPY runtimes/build.sh /build.sh
RUN chmod 755 /build.sh
COPY --from=composer:2 /usr/bin/composer /usr/local/bin/composer
