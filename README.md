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
- Unix domain socket for MSYS2(Cygwin) ( [#1](https://github.com/masahide/OmniSSHAgent/issues/1) )

## Usage

1. Download `OmniSSHAgent.zip` from https://github.com/masahide/OmniSSHAgent/releases/latest, unzip it, and place it in a folder of your choice.
2. If you are using Windows native ssh-agent - stop it. Open powershell with administrator privileges and execute the following.
```bash
Stop-Service ssh-agent
Set-Service -StartupType Disabled ssh-agent
```
  - Alternatively, you can set it through the GUI if you prefer.Bring up the start menu and type Services. You’ll see the Services app listed. 
Once the Services app is open, find the `OpenSSH Authentication Agent` service and set the `Service Status` to `Stop` and the `Startup Type` to `Disabled`.

3. If you are using [PuTTY Pageant](https://www.chiark.greenend.org.uk/~sgtatham/putty/index.html) - stop it.

4. Run `OmniSSHAgent.exe`
5. Press the `NEW OPEN FILE` button to add a private key file. Or you can use `ssh-add` command or [KeePassXC](https://keepassxc.org/) to add your private key.

### For use with WSL2
#### Setting up socat pipe in ubuntu environment.
Choose the instructions of your favourite shell below. If your shell isn't listed here you can convert the bash script to your shell syntax and send a PR to add it to the repo.

##### Bash
1. Download [ubuntu-bash.setup.sh](hack/ubuntu-bash.setup.sh) with the following command:
```bash
mkdir -p $HOME/omni-socat
curl -sL https://raw.githubusercontent.com/masahide/OmniSSHAgent/main/hack/ubuntu-bash.setup.sh -o $HOME/omni-socat/ubuntu-bash.setup.sh
```
2. Add the following line to `~/.bashrc`:
```bash
source $HOME/omni-socat/ubuntu-bash.setup.sh
```

##### Fish
1. Download [ubuntu-fish.setup.fish](hack/ubuntu-fish.setup.fish) with the following command:
```fish
mkdir -p $HOME/omni-socat
curl -sL https://raw.githubusercontent.com/masahide/OmniSSHAgent/main/hack/ubuntu-fish.setup.fish -o $HOME/omni-socat/ubuntu-fish.setup.fish
```
2. Add the following line to `~/.config/fish/config.fish`:
```fish
. $HOME/omni-socat/ubuntu-fish.setup.fish
```

#### Setting up socat pipe in rocky linux environment.

1. Download [rocky-bash.setup.sh](hack/rocky-bash.setup.sh) with the following command
```bash
mkdir -p $HOME/omni-socat
curl -sL https://raw.githubusercontent.com/masahide/OmniSSHAgent/main/hack/rocky-bash.setup.sh -o $HOME/omni-socat/rocky-bash.setup.sh
```
2. Add the following line to `~/.bashrc`:
```bash
source $HOME/omni-socat/rocky-bash.setup.sh
```

### For use with WSL1
Setting up Unix doman socket in ubuntu environment.


1. Check the setting of `Unix domain socket file path(WSL1):` in OmniSSHAgent.
For example, if you have the following settings.. (`UserName` varies depending on your environment)
`C:\Users\<UserName>\OmniSSHAgent.sock`
The WSL1 path will be `/mnt/c/Users/<UserName>/OmniSSHAgent.sock`.

2. Add the following line to `~/.bashrc`
```bash
export SSH_AUTH_SOCK=/mnt/c/Users/<UserName>/OmniSSHAgent.sock`
```

### For use with Cygwin/MSYS2/Git for windows/(GitBash)
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

### 1Password proxy mode

Mode to use [1Password's ssh-agent function](https://developer.1password.com/docs/ssh/) as a backend as shown in the following figure.
![1Password-Proxy-mode](https://github.com/masahide/OmniSSHAgent/blob/main/doc/1passwordProxyMode.png?raw=true)

By setting "Enable proxy mode for 1Password key-agent" in the configuration, OmniSSHAgent becomes a Proxy that works with 1Password's ssh-agent as a backend.

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

## Screen shot

<img src="https://github.com/masahide/OmniSSHAgent/blob/main/doc/screen.png?raw=true" width="500">
<img src="https://github.com/masahide/OmniSSHAgent/blob/main/doc/screen-setup.png?raw=true" width="500">
