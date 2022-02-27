
OMNISOCATCMD=$HOME/omni-socat/omni-socat.exe
export SSH_AUTH_SOCK=$HOME/.ssh/agent.sock

__get_omnisocat () {
  echo "Get omni-socat.exe"
  curl https://github.com/masahide/OmniSSHAgent/releases/latest/download/omni-socat.zip \
      -sLo omni-socat.zip
  unzip -o omni-socat.zip -d $(dirname $OMNISOCATCMD)
  rm omni-socat.zip
}

__get_socat () {
  echo "Install socat"
  sudo apt -y install socat
}


setup_omnisocat () {
  [[ -f $OMNISOCATCMD ]]  || __get_omnisocat
  [[ -f /usr/bin/socat ]] || __get_socat
  
  ss -a | grep -q $SSH_AUTH_SOCK
  [[ $? -ne 0 ]]  || return

  rm -f $SSH_AUTH_SOCK
  (setsid socat UNIX-LISTEN:$SSH_AUTH_SOCK,fork EXEC:"$OMNISOCATCMD",nofork &) >/dev/null 2>&1
}

setup_omnisocat

