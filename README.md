# OmniSSHAgent

![OmniSSHAgent](https://github.com/masahide/OmniSSHAgent/blob/main/build/appicon.png?raw=true)

## About

Unifies the chaotic ssh-agent state under Windows.

### The Chaotic State of SSH-Agent on Windows
On Windows, there are multiple communication methods for SSH agents, leading to complexity in usage and configuration. The following diagram illustrates the current SSH agent communication landscape on Windows.
![windows-ssh-agent-chaosmap](https://github.com/masahide/OmniSSHAgent/blob/main/doc/windows-ssh-agent-chaosmap.png?raw=true)


### OmniSSHAgent Connection Diagram
OmniSSHAgent simplifies this chaotic situation, as shown in the diagram below.
![OmniSSHAgentmap](https://github.com/masahide/OmniSSHAgent/blob/main/doc/OmniSSHAgent.png?raw=true)

## System Requirements

- Windows 11
- [Microsoft Edge WebView2](https://developer.microsoft.com/en-us/microsoft-edge/webview2/) 

## Supported Interfaces
- Pageant.exe (PuTTY) shared memory
- Unix domain socket for WSL2
- NamedPipe on Windows
- Unix domain socket for WSL1
- Unix domain socket for MSYS2 (Cygwin) ([#1](https://github.com/masahide/OmniSSHAgent/issues/1))

## Usage
1. Download `OmniSSHAgent-amd64-installer.exe` from [the latest release](https://github.com/masahide/OmniSSHAgent/releases/latest), and run the installer.
2. If you are using the native Windows SSH agent, you will need to stop and disable it. Open PowerShell with administrator privileges and run the following commands:
```powershell
Stop-Service ssh-agent
Set-Service -StartupType Disabled ssh-agent
```
  - Alternatively, you can do this through the GUI: open the Start menu, type "Services," and select the Services app. 
  Once open, find the `OpenSSH Authentication Agent` service, set `Service Status` to `Stop`, and `Startup Type` to `Disabled`.

3. If you are using [PuTTY Pageant](https://www.chiark.greenend.org.uk/~sgtatham/putty/index.html), stop it.

4. Launch `OmniSSHAgent.exe` by double-clicking it.
5. Press the `Open new file` button to add a private key file, or use the `ssh-add` command or [KeePassXC](https://keepassxc.org/) to add your private key.

### Registering for Startup

OmniSSHAgent does not have an installer to register itself for startup automatically. To add it manually:

- Press the Windows logo key + R, type `shell:startup`, and click OK. This opens the Startup folder.
- Copy and paste a shortcut to `OmniSSHAgent.exe` into the Startup folder.

### Using with WSL2
#### Setting up wsl2-ssh-agent-proxy in Ubuntu or Rocky (WSL2)
Choose the instructions for your preferred shell below. If your shell is not listed, you can convert the Bash script syntax and submit a pull request to add it to the repository.

##### Bash (and all POSIX-compliant shells)
1. Download [ubuntu.wsl2-ssh-agent-proxy.sh](hack/ubuntu.wsl2-ssh-agent-proxy.sh) using the following command:
```bash
mkdir -p $HOME/wsl2-ssh-agent-proxy
curl -sL https://raw.githubusercontent.com/masahide/OmniSSHAgent/refs/heads/main/hack/ubuntu.wsl2-ssh-agent-proxy.sh -o $HOME/wsl2-ssh-agent-proxy/ubuntu.wsl2-ssh-agent-proxy.sh
```
2. Add the following line to `~/.bashrc`, `~/.zshrc`, or the appropriate file for your shell:
```bash
source $HOME/wsl2-ssh-agent-proxy/ubuntu.wsl2-ssh-agent-proxy.sh
```

##### Fish
1. Download [ubuntu.wsl2-ssh-agent-proxy.fish](hack/ubuntu.wsl2-ssh-agent-proxy.fish) using the following command:
```fish
mkdir -p $HOME/wsl2-ssh-agent-proxy
curl -sL https://raw.githubusercontent.com/masahide/OmniSSHAgent/refs/heads/main/hack/ubuntu.wsl2-ssh-agent-proxy.fish -o $HOME/wsl2-ssh-agent-proxy/ubuntu.wsl2-ssh-agent-proxy.fish
```
2. Add the following line to `~/.config/fish/config.fish`:
```fish
. $HOME/wsl2-ssh-agent-proxy/ubuntu.wsl2-ssh-agent-proxy.fish
```

### Using with WSL1
Setting up a Unix domain socket in the Ubuntu environment:

1. Check the setting for `Unix domain socket file path (WSL1)` in OmniSSHAgent.
For example, if the path is set as follows (`UserName` will vary based on your environment):
`C:\Users\<UserName>\OmniSSHAgent.sock`
The WSL1 path would be `/mnt/c/Users/<UserName>/OmniSSHAgent.sock`.

2. Add the following line to `~/.bashrc`:
```bash
export SSH_AUTH_SOCK=/mnt/c/Users/<UserName>/OmniSSHAgent.sock
```

### Using with Cygwin/MSYS2/Git for Windows (Git Bash)
1. Check the setting for `Cygwin Unix domain socket file path (MSYS2)` in OmniSSHAgent.
   * For example, if the path is (`UserName` will vary based on your environment):
   * `C:\Users\<UserName>\OmniSSHCygwin.sock`
   * The Cygwin path would be `/mnt/c/Users/<UserName>/OmniSSHCygwin.sock`.

2. To set the `SSH_AUTH_SOCK` variable:
   * On the Windows taskbar, right-click the Windows icon and select System.
   * In the Settings window, under Related Settings, click Advanced system settings.
   * On the Advanced tab, click Environment Variables.
   * In `User variables`, click `New` to create a new environment variable:
```
Variable name:  SSH_AUTH_SOCK
Variable value: /mnt/c/Users/<UserName>/OmniSSHAgent.sock
```

## Using with OpenSSH ssh-agent NamedPipe (also compatible with 1Password) in Proxy Mode

This mode uses the [OpenSSH ssh-agent NamedPipe](https://learn.microsoft.com/windows-server/administration/openssh/openssh_keymanagement) as a backend. It can also be used with [1Password’s ssh-agent function](https://developer.1password.com/docs/ssh/agent/), as shown in the diagram below.  
![NamedPipe-Proxy-mode](https://github.com/masahide/OmniSSHAgent/blob/main/doc/NamedPipeProxyMode.png?raw=true)

By enabling **"Proxy mode for OpenSSH agent (also compatible with 1Password)"** in the configuration, OmniSSHAgent functions as a proxy for Windows OpenSSH's NamedPipe SSH agent.  
This mode also works with the 1Password key-agent.

**Note:** When "Proxy mode for OpenSSH agent (also compatible with 1Password)" is enabled, OmniSSHAgent operates solely as a proxy, and private keys cannot be added directly to it.


## Supported Key File Formats
- PuTTY private key file (.ppk)
- OpenSSH format

## Supported Key Types
- RSA
- ECDSA 
- ED25519 

(DSA, ECDSA-SK, ED25519-SK are not supported)

## FAQ

### Where is the passphrase for the private key stored?

Passphrases are stored in the [Windows Credential Manager](https://support.microsoft.com/en-us/windows/accessing-credential-manager-1b5c916a-6a16-889f-8581-fc16e8165ac0).

## Screenshots

<img src="https://github.com/masahide/OmniSSHAgent/blob/main/doc/screen.png?raw=true" width="500">
<img src="https://github.com/masahide/OmniSSHAgent/blob/main/doc/screen-setup.png?raw=true" width="500">
