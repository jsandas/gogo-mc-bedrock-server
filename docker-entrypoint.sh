#!/bin/bash

APP_DIR=${APP_DIR:-/opt/minecraft}

function download() {
    curl -H "User-Agent: Mozilla/5.0" -O https://www.minecraft.net/bedrockdedicatedserver/bin-linux/bedrock-server-${MINECRAFT_VER}.zip
    unzip -qq bedrock-server-${MINECRAFT_VER}.zip
    rm bedrock-server-${MINECRAFT_VER}.zip
}

if [[ "$@" == "" ]]; then
    if [[ $EULA_ACCEPT != 'true' ]]; then
        echo " Please accept the Minecraft EULA and Microsoft Privacy Policy"
        echo " with env var EULA_ACCEPT=true"
        echo " Links:"
        echo "   https://minecraft.net/eula"
        echo "   https://go.microsoft.com/fwlink/?LinkId=521839"
        echo
        exit 1
    fi

    download 

    exec ./minecraft-server-wrapper
fi

exec "$@"
