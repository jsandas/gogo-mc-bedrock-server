FROM golang:1.24 AS builder

WORKDIR /app

COPY . .

RUN go build -o minecraft-server-wrapper ./cmd/minecraft-server-wrapper


FROM debian:bookworm

ARG MC_VER=1.21.94.2

ENV DEBIAN_FRONTEND=noninteractive
ENV MINECRAFT_VER=${MC_VER}
ENV APP_DIR=/opt/minecraft

RUN apt-get update && apt-get upgrade -y \
    && apt-get install -y curl unzip \
    && apt-get autoremove -y \
    && rm -rf /var/lib/apt/lists/*

RUN useradd -m -d ${APP_DIR} -s /bin/bash minecraft \
    && chown -R minecraft ${APP_DIR}

WORKDIR ${APP_DIR}

COPY --from=itzg/mc-monitor /mc-monitor /usr/local/bin/mc-monitor
COPY docker-entrypoint.sh ${APP_DIR}

RUN chmod +x ${APP_DIR}/docker-entrypoint.sh \
    && chown -R minecraft /opt/minecraft

COPY --from=builder /app/minecraft-server-wrapper ${APP_DIR}/minecraft-server-wrapper

USER minecraft

ENTRYPOINT ["/opt/minecraft/docker-entrypoint.sh"]
