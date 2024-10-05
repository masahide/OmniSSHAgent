# OmniSSHAgent

![OmniSSHAgent](https://github.com/masahide/OmniSSHAgent/blob/main/build/appicon.png?raw=true)

## About

Unifies the chaotic ssh-agent state under Windows.

### The Chaotic State of SSH-Agent on Windows
There are several different communication methods for ssh-agent in Windows, and it is very complicated to use and configure them.
The following diagram shows the current communication methods for Windows ssh-agent.
![windows-ssh-agent-chaosmap](https://github.com/masahide/OmniSSHAgent/blob/main/doc/windows-ssh-agent-chaosmap.png?raw=true)


### Connection diagram of OmniSSHAgent
OmniSSHAgent is a program to simplify what used to be a chaotic situation, as shown in the following figure.
![OmniSSHAgentmap](https://github.com/masahide/OmniSSHAgent/blob/main/doc/OmniSSHAgent.png?raw=true)

## Required environment for operation

- Windows10
- [Microsoft Edge WebView2](https://developer.microsoft.com/en-us/microsoft-edge/webview2/) 

## The following interfaces are supported
- pageant.exe(PuTTY) shared memory
- Unix domain socket for WSL3
- NamedPipe on Windows
- Unix domain socket for WSL1
- Unix domain socket for MSYS2(Cygwin) ( [#1](https://github.com/masahide/OmniSSHAgent/issues/1) )

## Usage

1. Download `OmniSSHAgent.zip` from https://github.com/masahide/OmniSSHAgent/releases/latest, unzip it, and place it in a folder of your choice.
2. If you are using Windows native ssh-agent, you'll need to stop and disable it. Open powershell with administrator privileges and execute the following commands.
```bash
Stop-Service ssh-agent
Set-Service -StartupType Disabled ssh-agent
```
  - Alternatively, you can set it through the GUI if you prefer.Bring up the start menu and type Services. You’ll see the Services app listed. 
  Once the Services app is open, find the `OpenSSH Authentication Agent` service and set the `Service Status` to `Stop` and the `Startup Type` to `Disabled`.

3. If you are using [PuTTY Pageant](https://www.chiark.greenend.org.uk/~sgtatham/putty/index.html) - stop it.

4. Launch `OmniSSHAgent.exe` by double-clicking.
5. Press the `Open new file` button to add a private key file. Or you can use `ssh-add` command or [KeePassXC](https://keepassxc.org/) to add your private key.

### Registering for launch on boot

OmniSSHAgent does not have an installer, nor registers itself to start upon boot. You'll need to do the following to register it:

- Press the Windows logo key + R, type shell:startup, then click OK. This opens the Startup folder.
- Copy and paste the shortcut to the OmniSSHAgent.exe from the file location to the Startup


### Using with WSL2
#### Setting up wsl2-ssh-agent-proxy in Ubuntu or Rocky(WSL2).
Choose the instructions for your favourite shell below. If your shell isn't listed here you can convert the bash script to your shell syntax and send a PR to add it to the repo.

##### Bash (and all POSIX-compliant shells)
1. Download [ubuntu.setup.sh](hack/ubuntu.setup.sh) with the following command:
```bash
mkdir -p $HOME/wsl2-ssh-agent-proxy
curl -sL https://raw.githubusercontent.com/masahide/OmniSSHAgent/wsl2-ssh-agent-proxy/hack/ubuntu.wsl2-ssh-agent-proxy.sh -o $HOME/wsl2-ssh-agent-proxy/ubuntu.wsl2-ssh-agent-proxy.sh
```
2. Add the following line to `~/.bashrc`, `~/.zshrc`, or whatever file is applicable to your shell:
```bash
source $HOME/wsl2-ssh-agent-proxy/ubuntu.wsl2-ssh-agent-proxy.sh
```

##### Fish
1. Download [ubuntu-fish.setup.fish](hack/ubuntu-fish.setup.fish) with the following command:
```fish
mkdir -p $HOME/wsl2-ssh-agent-proxy
curl -sL https://raw.githubusercontent.com/masahide/OmniSSHAgent/wsl2-ssh-agent-proxy/hack/ubuntu.wsl2-ssh-agent-proxy-fish.sh -o $HOME/wsl2-ssh-agent-proxy/ubuntu.wsl2-ssh-agent-proxy-fish.sh
```
2. Add the following line to `~/.config/fish/config.fish`:
```fish
. $HOME/wsl2-ssh-agent-proxy/ubuntu.wsl2-ssh-agent-proxy-fish.sh
```

### Using with WSL1
Setting up Unix doman socket in ubuntu environment.


1. Check the setting of `Unix domain socket file path(WSL1):` in OmniSSHAgent.
For example, if you have the following settings.. (`UserName` varies depending on your environment)
`C:\Users\<UserName>\OmniSSHAgent.sock`
The WSL1 path will be `/mnt/c/Users/<UserName>/OmniSSHAgent.sock`.

2. Add the following line to `~/.bashrc`
```bash
export SSH_AUTH_SOCK=/mnt/c/Users/<UserName>/OmniSSHAgent.sock
```

### Using with Cygwin/MSYS2/Git for windows/(GitBash)
1. Check the setting of `Cygwin Unix domain socket file path(MSYS2):` in OmniSSHAgent.
   * For example, if you have the following settings.(`UserName` varies depending on your environment).
   * `C:\Users\<UserName>\OmniSSHCygwin.sock`.
   * The Cygwin path will be `/mnt/c/Users/<UserName>/OmniSSHCygwin.sock`.

2. On the Windows taskbar, right-click the Windows icon and select System.
In the Settings window, under Related Settings, click Advanced system settings.
   * On the Advanced tab, click Environment Variables.
   * `Users variables` Click on `Create new` to create a new environment variable.
   * Set the following values(`UserName` varies depending on your environment).
```
Variable name:  SSH_AUTH_SOCK
Variable Value: /mnt/c/Users/<UserName>/OmniSSHAgent.sock
```

## Using with OpenSSH ssh-agent NamedPipe (1Password etc.) proxy mode

This is a mode using [OpenSSH ssh-agent NamedPipe](https://learn.microsoft.com/windows-server/administration/openssh/openssh_keymanagement) or [1Password's ssh-agent function](https://developer.1password.com/docs/ssh/agent/) as a backend as shown in the following figure.
![NamedPipe-Proxy-mode](https://github.com/masahide/OmniSSHAgent/blob/main/doc/NamedPipeProxyMode.png?raw=true)

By setting "Enable proxy mode for 1Password key-agent" in the configuration, OmniSSHAgent becomes a Proxy that works with 1Password or OpenSSH's Namedpipe ssh-agent  as a backend.

When "Enable proxy mode for 1Password key-agent" is enabled, OmniSSHAgent operates as a mere proxy, and therefore, private keys cannot be added.

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

# Screen shot

<img src="https://github.com/masahide/OmniSSHAgent/blob/main/doc/screen.png?raw=true" width="500">
<img src="https://github.com/masahide/OmniSSHAgent/blob/main/doc/screen-setup.png?raw=true" width="500">
