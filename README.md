# OmniSSHAgent

![OmniSSHAgent](https://github.com/masahide/OmniSSHAgent/blob/main/build/appicon.png?raw=true)

## About

The chaotic windows ssh-agent has been integrated into one program.

### Chaos Map of SSH-Agent on Windows
There are several different communication methods for ssh-agent in windows, and it is very complicated to use and configure them.
The following diagram shows the current communication methods for windows ssh-agent.
![windows-ssh-agent-chaosmap](https://github.com/masahide/OmniSSHAgent/blob/main/doc/windows-ssh-agent-chaosmap.png?raw=true)


### Connection diagram of OmniSSHAgent 
OmniSSHAgent is a program to simplify what used to be a chaotic situation, as shown in the following figure.
![OmniSSHAgentmap](https://github.com/masahide/OmniSSHAgent/blob/main/doc/OmniSSHAgent.png?raw=true)

## Required environment for operation

- Windows10
- [Microsoft Edge WebView2](https://developer.microsoft.com/en-us/microsoft-edge/webview2/) 

## The following interfaces are supported
- pageant.exe(PuTTY) shared memory
- Unix domain socket for WSL2
- NamedPipe on Windows
- Unix domain socket for WSL1
- ~~Unix domain socket for MSYS2(Cygwin)~~ ( [#1](https://github.com/masahide/OmniSSHAgent/issues/1) )


## Supported key file formats
- PuTTY private key file (.ppk) file format
- OpenSSH format

## Supported key formats
- rsa
- ecdsa 
- ed25519 

(dsa, ecdsa-sk, ed25519-sk are not supported)


## FAQ

### Where is the passphrase for the private key stored?

It's stored in [Windows Credential Manager](https://support.microsoft.com/en-us/windows/accessing-credential-manager-1b5c916a-6a16-889f-8581-fc16e8165ac0).
