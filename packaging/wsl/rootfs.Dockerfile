FROM ubuntu:24.04
ENV DEBIAN_FRONTEND=noninteractive
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates curl zstd systemd systemd-sysv dbus passwd iproute2 procps \
    && rm -rf /var/lib/apt/lists/*
RUN mkdir -p /opt/locas/bin /opt/locas/web /var/lib/locas /run/locas \
    && useradd --system --create-home --shell /usr/sbin/nologin locas-core \
    && useradd --system --create-home --shell /usr/sbin/nologin locas-agent \
    && useradd --system --create-home --shell /usr/sbin/nologin locas-ollama \
    && chmod 0750 /var/lib/locas
COPY packaging/wsl/wsl.conf /etc/wsl.conf
COPY packaging/wsl/locas-core.service /etc/systemd/system/locas-core.service
COPY packaging/wsl/locas-agent.service /etc/systemd/system/locas-agent.service
COPY packaging/wsl/locas-ollama.service /etc/systemd/system/locas-ollama.service
COPY packaging/wsl/install-ollama.sh /usr/local/sbin/locas-install-ollama
COPY packaging/wsl/firstboot.sh /usr/local/sbin/locas-firstboot
COPY packaging/wsl/install-open-webui.sh /usr/local/sbin/locas-install-open-webui
COPY packaging/wsl/locas-app-worker.sh /usr/local/sbin/locas-app-worker
COPY packaging/wsl/locas-app-worker.service /etc/systemd/system/locas-app-worker.service
COPY packaging/wsl/locas-app-worker.path /etc/systemd/system/locas-app-worker.path
COPY packaging/wsl/locas-firstboot.service /etc/systemd/system/locas-firstboot.service
COPY build/locas-core /opt/locas/bin/locas-core
COPY build/locas-agent /opt/locas/bin/locas-agent
RUN chmod 0755 /usr/local/sbin/locas-firstboot /usr/local/sbin/locas-install-ollama /usr/local/sbin/locas-install-open-webui /usr/local/sbin/locas-app-worker /opt/locas/bin/* \
    && systemctl enable locas-firstboot.service locas-agent.service locas-core.service locas-app-worker.path
