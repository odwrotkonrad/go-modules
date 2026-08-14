##[>] 🤖🤖
FROM debian:bookworm-slim
ENV DEBIAN_FRONTEND=noninteractive
RUN rm -f /etc/apt/apt.conf.d/docker-clean \
 && apt-get -qq update \
 && apt-get -qq install --yes --no-install-recommends curl ca-certificates git unzip \
 && rm -rf /var/lib/apt/lists/*
##[<] 🤖🤖
