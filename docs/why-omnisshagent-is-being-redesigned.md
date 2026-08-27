# Why OmniSSHAgent Is Being Redesigned

OmniSSHAgent was created to unify the fragmented SSH agent environment on Windows and allow different SSH clients to use the same keys.

That goal is not changing.

What has changed is the Windows SSH ecosystem itself. Windows OpenSSH has become a standard part of modern Windows installations, and several applications now provide OpenSSH-compatible SSH agent implementations. After reviewing the current environment and the responsibilities accumulated in OmniSSHAgent, the project was redesigned around a smaller and clearer separation between SSH agent backends and the Windows interfaces used to access them.

Windows OpenSSH remains the default backend. In that mode, OmniSSHAgent acts as a Windows-native compatibility bridge between an external OpenSSH-compatible backend and applications that require Pageant or Cygwin/MSYS2-compatible interfaces.

The project may also provide narrowly scoped alternative backends when they serve a different security or key-lifetime model. Those backends build on the same separation and do not restore the legacy persistent key-management architecture.

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

With an external backend, key storage and signing are handled by software dedicated to those responsibilities. OmniSSHAgent can focus on protocol and interface compatibility.

## Why an Embedded Agent Backend Exists

The Windows OpenSSH Authentication Agent remains the recommended default backend, but not every workflow requires the same key-lifetime semantics.

Some users intentionally want SSH keys to exist only for the lifetime of a desktop agent process. [KeePassXC SSH Agent integration](https://keepassxc.org/docs/KeePassXC_UserGuide) acts as a client of an existing agent: it can add keys when a database is unlocked and remove them when it is locked.

For these workflows, persistent key availability across Windows sessions may not be desirable. OmniSSHAgent therefore provides an optional embedded backend that accepts standard SSH agent requests and retains loaded keys only in process memory.

The embedded backend does not persist private keys, private-key paths, or decryption passphrases, and it does not automatically reload keys after OmniSSHAgent restarts. It is a small SSH agent protocol component rather than a persistent key-management subsystem.

## Windows SSH Agent Interfaces Are Still Fragmented

Although Windows OpenSSH has become widely available, Windows applications do not all use the same SSH agent interface.

The Windows SSH ecosystem still includes several incompatible mechanisms:

- Windows OpenSSH Named Pipes
- PuTTY Pageant shared memory and window messages
- Cygwin and MSYS2-compatible sockets
- Unix Domain Sockets inside WSL

Some applications use the Windows OpenSSH Named Pipe directly. Others, including established Windows tools, still expect a Pageant-compatible interface. Git for Windows, MSYS2, and Cygwin environments may expect a Cygwin-compatible socket instead.

This fragmentation remains the problem OmniSSHAgent is best positioned to solve.

The redesigned OmniSSHAgent will not introduce another persistent key store. It connects applications to a selected SSH agent backend.

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

The redesigned OmniSSHAgent is a Windows-only SSH agent backend and interface bridge.

Its default backend architecture is:

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

Its embedded backend architecture is:

```text
         OmniSSHAgent
               |
       embedded keyring
          /    |    \
   OpenSSH  Pageant  Cygwin/MSYS2
```

The two modes share the same Pageant and Cygwin/MSYS2 interface implementations. The difference is where key retention and signing take place. External mode forwards requests to the configured OpenSSH-compatible agent. Embedded mode signs with keys retained by the OmniSSHAgent process and also exposes the standard OpenSSH Named Pipe.

Neither mode persists private keys or decryption passphrases.

The initial redesigned MVP focused on:

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

The initial redesigned MVP did not include:

- Wails
- WebView2
- A web-based settings UI
- Private key file management
- An independent in-memory SSH agent
- Windows Credential Manager passphrase storage
- WSL proxy functionality
- Automatic update functionality
- A plugin system

These features were left for independent, clearly scoped additions.

The embedded backend is one such addition. It reintroduces only an ephemeral SSH agent protocol keyring and Named Pipe interface, not the legacy metadata store, Credential Manager integration, automatic key reload, Wails UI, PPK management, or WSL transport.

## Goals of the Redesign

The redesign is not simply a reduction in features. It is a redefinition of the project's value in the current Windows SSH ecosystem.

### Avoid Conflicting with Windows Standard Components by Default

Windows OpenSSH remains the default backend, and OmniSSHAgent does not disable or replace it automatically.

The embedded backend is an explicit opt-in mode. Its OpenSSH interface must own the standard OpenSSH Named Pipe, so another agent, including the Windows OpenSSH Authentication Agent, cannot own that pipe at the same time. OmniSSHAgent never stops or disables the Windows service automatically.

### Keep Persistent Key Management Outside OmniSSHAgent

When an external backend is selected, key storage, passphrase handling, hardware-backed security, signing policy, and signing remain the responsibility of that backend.

When the embedded backend is selected, OmniSSHAgent temporarily holds key material in process memory so it can perform SSH agent signing operations. It does not persist key material, remember private-key file paths, store decryption passphrases, or reload keys after restart.

| Legacy OmniSSHAgent | Embedded backend |
| --- | --- |
| Remembered key files | Does not remember paths |
| Stored passphrases in Credential Manager | Does not store passphrases |
| Restored configured keys after restart | Starts empty |
| Maintained custom key metadata | Has no metadata store |
| Used a Wails key manager | Uses the native shared Settings and Manage keys UI |
| Managed PPK files | Does not support PPK management |
| Included WSL transport | Leaves WSL transport to Pipeferry |
| Implemented SSH agent signing | Implements SSH agent signing |
| Exposed OpenSSH, Pageant, and Cygwin interfaces | Reuses the redesigned interface components |

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

The redesigned application does not restore legacy key configuration or persisted passphrases.

Keys must be added to the selected backend, such as:

- Windows OpenSSH Authentication Agent
- 1Password SSH Agent
- Another OpenSSH Named Pipe-compatible backend
- The embedded ephemeral backend, which starts empty after every process restart

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

Instead of becoming another monolithic key manager, OmniSSHAgent separates a selected SSH agent backend from the Windows interfaces that clients use.

## Conclusion

This redesign does not reject the direction of the existing project.

The current implementation was created to solve real limitations in the Windows SSH ecosystem, and many of its features were necessary when they were introduced.

The ecosystem has since matured.

Windows now includes OpenSSH, password managers can provide OpenSSH-compatible agents, and WSL can run independently managed systemd services. These changes make it possible to separate key management, Windows compatibility, and WSL transport into distinct components.

The redesigned OmniSSHAgent will therefore focus on one responsibility:

> Connect Windows applications that use different SSH agent interfaces to a selected SSH agent backend.

The redesign established boundaries between backends, compatibility interfaces, application lifecycle, and WSL transport. Adding an embedded backend builds on those boundaries rather than returning to the legacy monolithic architecture.

## Related Documents

- [OmniSSHAgent Windows MVP Requirements](./260720-omnisshagent-windows-mvp-requirements.md)
- [Pipeferry OpenSSH Agent Integration](https://github.com/masahide/pipeferry/blob/main/docs/openssh-agent.md)
- [Microsoft OpenSSH for Windows overview](https://learn.microsoft.com/windows-server/administration/openssh/openssh-overview)
- [Microsoft OpenSSH key management](https://learn.microsoft.com/windows-server/administration/openssh/openssh_keymanagement)
- [1Password SSH Agent documentation](https://developer.1password.com/docs/ssh/agent/)
