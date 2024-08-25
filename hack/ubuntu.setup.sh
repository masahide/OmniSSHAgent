#!/bin/sh

NAME=wsl2-ssh-agent-proxy
SSH_AUTH_SOCK="${HOME}/.ssh/${NAME}/${NAME}.sock"
PROXYCMD_DIR="${HOME}/${NAME}"
CMD="${PROXYCMD_DIR}/${NAME}"

RELEASSES_NAME=$1
REPO_URL=https://github.com/masahide/OmniSSHAgent
VERSION=$1
if [ -z "$VERSION" ]; then
  $VER_PATH="download/${RELEASSES_NAME}"
else
  $VER_PATH="releases/latest"
fi

__get_proxy() {
  echo "Downloading ${NAME}.gz"
  mkdir -p ${PROXYCMD_DIR}
  curl "${REPO_URL}/releases/${VER_PATH}/${NAME}.gz" -sL |ungzip >${CMD}
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

  rm -f "${SSH_AUTH_SOCK}"
  ${CMD} & > /dev/null 2>&1
}

setup_proxy
