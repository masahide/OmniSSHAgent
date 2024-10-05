#!/usr/bin/env fish

set NAME "wsl2-ssh-agent-proxy"
set -x SSH_AUTH_SOCK "$HOME/.ssh/$NAME/$NAME.sock"
set PROXYCMD_DIR "$HOME/$NAME"
set CMD "$PROXYCMD_DIR/$NAME"

set RELEASE_NAME $argv[1]
set REPO_URL "https://github.com/masahide/OmniSSHAgent"

if test -z "$RELEASE_NAME"
    set VER_PATH "releases/latest"
else
    set VER_PATH "download/$RELEASE_NAME"
end

function __get_proxy
    echo "Downloading $NAME.gz"
    mkdir -p "$PROXYCMD_DIR"
    curl "$REPO_URL/releases/$VER_PATH/$NAME.gz" -sL | gunzip > "$CMD"
    chmod +x "$CMD"
end

function setup_proxy
    if not test -f "$CMD"
        __get_proxy
    end

    # Checks whether $SSH_AUTH_SOCK is a socket or not
    if pgrep -f "$CMD" > /dev/null; and test -S "$SSH_AUTH_SOCK"
        return
    end

    # Create directory for the socket, if it is missing
    set SSH_AUTH_SOCK_DIR (dirname "$SSH_AUTH_SOCK")
    mkdir -p "$SSH_AUTH_SOCK_DIR"

    # Applying best-practice permissions if we are creating $HOME/.ssh
    if test "$SSH_AUTH_SOCK_DIR" = "$HOME/.ssh"
        chmod 700 "$SSH_AUTH_SOCK_DIR"
    end

    setsid "$CMD" >> "$PROXYCMD_DIR/$NAME.log" 2>&1 &
end

setup_proxy
