##[>] 🤖🤖
FROM debian:bookworm-slim
ENV DEBIAN_FRONTEND=noninteractive
RUN rm -f /etc/apt/apt.conf.d/docker-clean /etc/dpkg/dpkg.cfg.d/docker \
 && apt-get -qq update \
 && apt-get -qq install --yes --no-install-recommends ca-certificates man-db \
 && rm -rf /var/lib/apt/lists/*
##[<] 🤖🤖
