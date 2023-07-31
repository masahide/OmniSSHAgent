set OMNISOCATCMD $HOME/omni-socat/omni-socat.exe
export SSH_AUTH_SOCK=$HOME/.ssh/agent.sock

function __get_omnisocat
  echo "Get omni-socat.exe"
  curl https://github.com/masahide/OmniSSHAgent/releases/latest/download/omni-socat.zip \
      -sLo omni-socat.zip
  unzip -o omni-socat.zip -d (dirname $OMNISOCATCMD)
  chmod +x $OMNISOCATCMD
  rm omni-socat.zip
end

function __get_socat
  echo "Install socat"
  sudo apt -y install socat
end


function setup_omnisocat
  if not test -f $OMNISOCATCMD
    __get_omnisocat
  end
  if not test -f /usr/bin/socat
    __get_socat
  end
  
  # Checks wether $SSH_AUTH_SOCK is a socket or not
  if test -S $SSH_AUTH_SOCK
    return
  end
  
  rm -f $SSH_AUTH_SOCK

  setsid nohup socat UNIX-LISTEN:$SSH_AUTH_SOCK,fork EXEC:"$HOME/omni-socat/omni-socat.exe" >/dev/null 2>&1 & disown
end

setup_omnisocat
