#!/bin/sh

OMNISOCATCMD="$HOME/omni-socat/omni-socat.exe"
export SSH_AUTH_SOCK="$HOME/.ssh/agent.sock"

__get_omnisocat() {
  echo "Downloading omni-socat.exe"
  curl https://github.com/masahide/OmniSSHAgent/releases/latest/download/omni-socat.zip \
    -sLo omni-socat.zip
  unzip -o omni-socat.zip -d "$(dirname "$OMNISOCATCMD")"
  chmod +x "$OMNISOCATCMD"
  rm -f omni-socat.zip
}

__get_ss() {
  echo "Installing ss"
  sudo dnf -y install iproute
}

__get_socat() {
  echo "Installing socat"
  sudo dnf -y install socat
}

setup_omnisocat() {
  [ -f "$OMNISOCATCMD" ] || __get_omnisocat
  command -v ss > /dev/null 2>&1 || __get_ss
  command -v socat > /dev/null 2>&1 || __get_socat

  # Checks wether $SSH_AUTH_SOCK is a socket or not
  (ss -a | grep -q "$SSH_AUTH_SOCK") && [ -S "$SSH_AUTH_SOCK" ] && return

  # Create directory for the socket, if it is missing
  SSH_AUTH_SOCK_DIR="$(dirname "$SSH_AUTH_SOCK")"
  mkdir -p "$SSH_AUTH_SOCK_DIR"
  # Applying best-practice permissions if we are creating $HOME/.ssh
  if [ "$SSH_AUTH_SOCK_DIR" = "$HOME/.ssh" ]; then
    chmod 700 "$SSH_AUTH_SOCK_DIR"
  fi

  rm -f "$SSH_AUTH_SOCK"
  (setsid socat UNIX-LISTEN:"$SSH_AUTH_SOCK",fork EXEC:"$OMNISOCATCMD",nofork &) > /dev/null 2>&1
}

setup_omnisocat
