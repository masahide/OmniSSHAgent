#!/bin/sh

NAME=wsl2-ssh-agent-proxy
SSH_AUTH_SOCK="${HOME}/.ssh/${NAME}/${NAME}.sock"
PROXYCMD_DIR="${HOME}/${NAME}"
CMD="${PROXYCMD_DIR}/${NAME}"

RELEASE_NAME=$1
REPO_URL=https://github.com/masahide/OmniSSHAgent
if [ -z "${RELEASE_NAME}" ]; then
  VER_PATH="releases/latest"
else
  VER_PATH="download/${RELEASE_NAME}"
fi

__get_proxy() {
  echo "Downloading ${NAME}.gz"
  mkdir -p "${PROXYCMD_DIR}"
  curl "${REPO_URL}/releases/${VER_PATH}/${NAME}.gz" -sL | ungzip > "${CMD}"
  chmod +x "${CMD}"
}

setup_proxy() {
  [ -f "${CMD}" ] || __get_proxy

  # Checks wether $SSH_AUTH_SOCK is a socket or not
  (ps aux | grep "${CMD}" | grep -qv "grep") && [ -S "${SSH_AUTH_SOCK}" ] && return

  # Create directory for the socket, if it is missing
  SSH_AUTH_SOCK_DIR="$(dirname "${SSH_AUTH_SOCK}")"
  mkdir -p "${SSH_AUTH_SOCK_DIR}"
  # Applying best-practice permissions if we are creating ${HOME}/.ssh
  if [ "${SSH_AUTH_SOCK_DIR}" = "${HOME}/.ssh" ]; then
    chmod 700 "${SSH_AUTH_SOCK_DIR}"
  fi

  ${CMD} >> "${PROXYCMD_DIR}/${NAME}.log" 2>&1 &
}

setup_proxy
