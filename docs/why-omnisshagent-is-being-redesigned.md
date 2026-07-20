# Why OmniSSHAgent Is Being Redesigned

OmniSSHAgent was created to unify the fragmented SSH agent environment on Windows and allow different SSH clients to use the same keys.

That goal is not changing.

What has changed is the Windows SSH ecosystem itself. Windows OpenSSH has become a standard part of modern Windows installations, and several applications now provide OpenSSH-compatible SSH agent implementations. After reviewing the current environment and the responsibilities accumulated in OmniSSHAgent, the project is being redesigned around a smaller and clearer role.

The new OmniSSHAgent will no longer try to be a complete SSH agent and key manager. Instead, it will act as a Windows-native compatibility bridge between an OpenSSH-compatible backend and applications that still require Pageant or Cygwin/MSYS2-compatible interfaces.

## Windows OpenSSH Has Become the Standard Foundation

OpenSSH has been available as a Windows optional feature since Windows 10 version 1809 and continues to be supported in Windows 11. Windows includes the OpenSSH client tools and the OpenSSH Authentication Agent service, including commands such as `ssh`, `ssh-add`, and `ssh-agent`.

The current OmniSSHAgent implementation conflicts with this model.

To expose the standard OpenSSH agent Named Pipe, the existing setup requires users to stop and disable the Windows OpenSSH Authentication Agent service before starting OmniSSHAgent. This made sense when OmniSSHAgent was intended to replace the native agent completely, but it is no longer the most natural architecture for modern Windows systems.

The redesigned architecture reverses that relationship.

Instead of disabling Windows OpenSSH, OmniSSHAgent will use an OpenSSH-compatible agent as its backend. Windows OpenSSH becomes the default source of keys and signing operations, while OmniSSHAgent provides only the compatibility interfaces required by other Windows applications.

## OpenSSH-Compatible Agents Are Becoming More Common

Windows OpenSSH is no longer the only application that can provide an OpenSSH-compatible SSH agent.

Applications such as 1Password can expose the same Windows Named Pipe interface:

```text
\\.\pipe\openssh-ssh-agent
```

By treating this Named Pipe as the standard backend interface, OmniSSHAgent can work with the Windows OpenSSH Authentication Agent, 1Password, and potentially other compatible implementations without needing to know how each backend stores or protects its keys.

This also reduces the need for OmniSSHAgent to implement its own private key handling, passphrase storage, and in-memory keyring.

Key storage and signing should be handled by software dedicated to those responsibilities. OmniSSHAgent should focus on protocol and interface compatibility.

## Windows SSH Agent Interfaces Are Still Fragmented

Although Windows OpenSSH has become widely available, Windows applications do not all use the same SSH agent interface.

The Windows SSH ecosystem still includes several incompatible mechanisms:

- Windows OpenSSH Named Pipes
- PuTTY Pageant shared memory and window messages
- Cygwin and MSYS2-compatible sockets
- Unix Domain Sockets inside WSL

Some applications use the Windows OpenSSH Named Pipe directly. Others, including established Windows tools, still expect a Pageant-compatible interface. Git for Windows, MSYS2, and Cygwin environments may expect a Cygwin-compatible socket instead.

This fragmentation remains the problem OmniSSHAgent is best positioned to solve.

The redesigned OmniSSHAgent will not introduce another key store. It will connect existing applications to an existing OpenSSH-compatible agent.

## Problems in the Current Architecture

The current OmniSSHAgent implementation has accumulated many responsibilities in a single application:

- An independent in-memory SSH agent
- OpenSSH and PuTTY PPK private key loading
- Passphrase storage in Windows Credential Manager
- Key addition and removal
- A Pageant-compatible interface
- A Windows OpenSSH-compatible Named Pipe
- A Cygwin and MSYS2-compatible socket
- A WSL1 Unix Domain Socket
- A WSL2 proxy
- A Wails and WebView2 configuration interface
- A system tray application
- Lifecycle management for all listeners and UI components

Each feature is useful by itself, but combining them in one process has made the boundaries between responsibilities unclear.

The current application must coordinate Windows-specific APIs, WSL integration, private key parsing, credential storage, multiple agent protocols, a web-based UI, system tray behavior, and shutdown handling.

This has several consequences:

- A change in one interface can affect unrelated interfaces
- Testing requires a broad set of Windows and WSL environments
- The runtime and build dependencies are larger than necessary
- Diagnosing startup and shutdown failures is difficult
- Key-management concerns are mixed with protocol-compatibility concerns
- Small changes require a wider regression-testing scope

The current configuration also enables several agent interfaces in the same application and stores private key metadata alongside interface settings. Proxy mode is available, but the independent in-memory keyring remains the primary design.

The redesign makes the proxy model the default and removes the independent key-management role from the initial implementation.

## WSL Integration Is Moving to Pipeferry

WSL integration has also changed significantly since OmniSSHAgent was first designed.

Modern WSL supports systemd, which makes it practical to run and manage a dedicated user service inside a WSL distribution. This allows the Windows-to-WSL bridge to be implemented and operated independently from the Windows compatibility layer.

WSL integration will therefore move to [Pipeferry](https://github.com/masahide/pipeferry).

Pipeferry provides the cross-boundary transport between WSL and the Windows OpenSSH-compatible Named Pipe. Its SSH agent setup creates a systemd user service inside WSL and exposes a normal Unix Domain Socket through `SSH_AUTH_SOCK`.

The resulting responsibility split is:

- OmniSSHAgent handles compatibility between Windows SSH agent interfaces
- Pipeferry handles communication between Windows and WSL

This separation makes both tools easier to develop, test, diagnose, and update.

OmniSSHAgent will no longer include WSL proxy binaries, WSL socket management, shell setup scripts, or PowerShell-based Named Pipe proxies.

For WSL setup, see:

- [Use the Windows OpenSSH Agent from WSL with Pipeferry](https://github.com/masahide/pipeferry/blob/main/docs/openssh-agent.md)

## The New Role of OmniSSHAgent

The redesigned OmniSSHAgent will be a Windows-only SSH agent compatibility bridge.

Its default architecture is:

```text
OpenSSH-compatible Windows agent
\\.\pipe\openssh-ssh-agent
               |
               v
         OmniSSHAgent
               |
               +-- Pageant-compatible interface
               |
               +-- Cygwin/MSYS2-compatible interface
```

OmniSSHAgent will not own or persist private keys.

It will receive SSH agent requests from Pageant-compatible or Cygwin/MSYS2-compatible clients, forward those requests to the configured OpenSSH-compatible backend, and return the backend response to the client.

The first MVP will focus on:

- Windows system tray residency
- TOML-based configuration
- Windows OpenSSH-compatible backend support
- Pageant compatibility
- Cygwin and MSYS2 compatibility
- Per-interface enable and disable settings
- Single-instance enforcement
- Failure isolation between interfaces
- File logging
- Predictable and safe shutdown behavior
- Diagnostic CLI commands

The first MVP will not include:

- Wails
- WebView2
- A web-based settings UI
- Private key file management
- An independent in-memory SSH agent
- Windows Credential Manager passphrase storage
- WSL proxy functionality
- Automatic update functionality
- A plugin system

These features may be reconsidered later as independent, clearly scoped additions.

## Goals of the Redesign

The redesign is not simply a reduction in features. It is a redefinition of the project's value in the current Windows SSH ecosystem.

### Avoid Conflicting with Windows Standard Components

Users should no longer need to disable the Windows OpenSSH Authentication Agent service in order to use OmniSSHAgent.

Windows OpenSSH will be the default backend rather than a competing implementation.

### Delegate Key Security to Specialized Backends

Private key storage, passphrase handling, hardware-backed security, and signing policy should be handled by the selected backend, such as Windows OpenSSH or 1Password.

OmniSSHAgent will not duplicate those security-sensitive responsibilities.

### Preserve Compatibility with Existing Applications

Applications that depend on Pageant or Cygwin/MSYS2-compatible interfaces will still be able to use keys from the OpenSSH-compatible backend.

This is the primary compatibility problem OmniSSHAgent will continue to solve.

### Isolate Failures

Each compatibility interface will be treated as an independent component.

For example:

- A Pageant interface conflict should not stop the Cygwin/MSYS2 interface
- A Cygwin socket conflict should not stop the Pageant interface
- A temporarily unavailable backend should fail only the current request
- Starting the backend later should not require restarting OmniSSHAgent
- A configuration error should still allow access to logs and the configuration file

### Reduce Runtime and Build Dependencies

Removing Wails, WebView2, Node.js, and Svelte from the MVP will produce a smaller Windows-native application with fewer runtime assumptions and fewer failure points.

### Make WSL Integration Independently Maintainable

Moving WSL support to Pipeferry allows WSL-specific behavior to evolve without changing the Windows compatibility bridge.

Each project can have its own release cycle, diagnostics, documentation, and test environment.

## Impact on Existing Users

Users of Pageant-compatible applications, Git for Windows, MSYS2, or Cygwin will still be able to use OmniSSHAgent as a compatibility bridge.

However, some workflows will change.

### Users Who Currently Store Keys in OmniSSHAgent

The redesigned MVP will not load or store private keys directly.

Keys must instead be added to the selected backend, such as:

- Windows OpenSSH Authentication Agent
- 1Password SSH Agent
- Another future OpenSSH Named Pipe-compatible backend

### Users Who Currently Use OmniSSHAgent from WSL

WSL integration will be provided by Pipeferry instead of OmniSSHAgent.

Pipeferry connects directly from a Unix Domain Socket in WSL to the OpenSSH-compatible Named Pipe on Windows.

### Users Who Disable the Windows OpenSSH Agent

Disabling the Windows OpenSSH Authentication Agent will no longer be the default installation procedure.

When using the Microsoft OpenSSH agent, the service should be enabled and used as the OmniSSHAgent backend.

When using 1Password or another compatible backend, that application will provide the same Named Pipe interface.

### Existing Configuration

The existing GUI-managed JSON settings will not be reused directly by the new MVP.

The redesigned application will use a versioned TOML configuration file. Migration documentation will be provided when the new implementation becomes available.

## What Is Not Changing

The original problem OmniSSHAgent addresses still exists.

Windows OpenSSH, PuTTY, WinSCP, TortoiseGit, Git for Windows, MSYS2, Cygwin, and WSL do not all use the same SSH agent connection mechanism.

OmniSSHAgent will continue to reduce that fragmentation.

What changes is how it solves the problem.

Instead of becoming another complete SSH agent, OmniSSHAgent will become a small, focused, and robust compatibility bridge around the OpenSSH-compatible interface that has emerged as the standard foundation on Windows.

## Conclusion

This redesign does not reject the direction of the existing project.

The current implementation was created to solve real limitations in the Windows SSH ecosystem, and many of its features were necessary when they were introduced.

The ecosystem has since matured.

Windows now includes OpenSSH, password managers can provide OpenSSH-compatible agents, and WSL can run independently managed systemd services. These changes make it possible to separate key management, Windows compatibility, and WSL transport into distinct components.

The redesigned OmniSSHAgent will therefore focus on one responsibility:

> Connect Windows applications that use legacy or environment-specific SSH agent interfaces to an OpenSSH-compatible Windows SSH agent.

This narrower role should make OmniSSHAgent easier to understand, safer to operate, simpler to test, and more sustainable to maintain.

## Related Documents

- [OmniSSHAgent Windows MVP Requirements](./260720-omnisshagent-windows-mvp-requirements.md)
- [Pipeferry OpenSSH Agent Integration](https://github.com/masahide/pipeferry/blob/main/docs/openssh-agent.md)
- [Microsoft OpenSSH for Windows overview](https://learn.microsoft.com/windows-server/administration/openssh/openssh-overview)
- [Microsoft OpenSSH key management](https://learn.microsoft.com/windows-server/administration/openssh/openssh_keymanagement)
- [1Password SSH Agent documentation](https://developer.1password.com/docs/ssh/agent/)
