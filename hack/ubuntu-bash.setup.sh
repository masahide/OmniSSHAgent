#!/bin/bash

OMNISOCAT=$HOME/omni-socat/omni-socat.exe
URL=https://github.com/masahide/OmniSSHAgent/releases/latest/download/omni-socat.zip

get_omnisocat () {
    echo "Get omni-socat.exe"
    curl -sL $URL -o omni-socat.zip
    unzip -o omni-socat.zip -d $(dirname $OMNISOCAT)
    rm omni-socat.zip
}

get_socat () {
    echo "Install socat"
    sudo apt -y install socat
}

[[ -f $OMNISOCAT ]] || get_omnisocat
[[ -f /usr/bin/socat ]] || get_socat

export SSH_AUTH_SOCK=$HOME/.ssh/agent.sock

ss -a | grep -q $SSH_AUTH_SOCK
if [[ $? -ne 0 ]]; then
    rm -f $SSH_AUTH_SOCK
    (setsid socat UNIX-LISTEN:$SSH_AUTH_SOCK,fork EXEC:"$OMNISOCAT",nofork &) >/dev/null 2>&1
fi

