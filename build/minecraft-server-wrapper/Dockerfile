FROM debian:bookworm

ARG TARGETPLATFORM
ARG TARGETARCH

ARG MC_VER=1.21.100.7

ENV DEBIAN_FRONTEND=noninteractive
ENV MINECRAFT_VER=${MC_VER}
ENV APP_DIR=/opt/minecraft

RUN apt-get update && apt-get upgrade -y \
    && apt-get install -y curl \
    && apt-get autoremove -y \
    && rm -rf /var/lib/apt/lists/*

RUN useradd -m -d ${APP_DIR} -s /bin/bash minecraft \
    && chown -R minecraft ${APP_DIR}

WORKDIR ${APP_DIR}

COPY dist/minecraft-server-wrapper_linux_${TARGETARCH}_v1/minecraft-server-wrapper ${APP_DIR}/minecraft-server-wrapper
# COPY $TARGETPLATFORM/minecraft-server-wrapper ${APP_DIR}/minecraft-server-wrapper

RUN chown -R minecraft ${APP_DIR}

USER minecraft

EXPOSE 19132/udp
EXPOSE 19133/udp

ENTRYPOINT ["/opt/minecraft/minecraft-server-wrapper"]
