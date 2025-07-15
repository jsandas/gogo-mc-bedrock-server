#!/bin/bash

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

    exec ./minecraft-server-wrapper
fi

exec "$@"
