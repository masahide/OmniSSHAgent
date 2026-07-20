[CmdletBinding()]
param(
    [switch]$IncludeWSL
)

$ErrorActionPreference = "Stop"
Set-StrictMode -Version Latest

function Write-Section {
    param([Parameter(Mandatory)][string]$Title)

    Write-Host ""
    Write-Host "== $Title =="
}

function Write-Detail {
    param(
        [Parameter(Mandatory)][string]$Name,
        [AllowNull()][object]$Value
    )

    if ($null -eq $Value -or [string]::IsNullOrWhiteSpace([string]$Value)) {
        $Value = "(not found)"
    }
    Write-Host ("{0}: {1}" -f $Name, $Value)
}

function Get-OptionalProperty {
    param(
        [AllowNull()][object]$InputObject,
        [Parameter(Mandatory)][string]$Name
    )

    if ($null -eq $InputObject) {
        return $null
    }
    $property = $InputObject.PSObject.Properties[$Name]
    if ($null -eq $property) {
        return $null
    }
    return $property.Value
}

function Get-LegacySettingsPaths {
    $paths = @()
    if (-not [string]::IsNullOrWhiteSpace($env:APPDATA)) {
        $paths += Join-Path $env:APPDATA "OmniSSHAgent.exe\settings.json"
        $paths += Join-Path $env:APPDATA "OmniSSHAgent\settings.json"
        $directories = @(
            Get-ChildItem -LiteralPath $env:APPDATA -Directory -Filter "OmniSSHAgent*" -ErrorAction SilentlyContinue
        )
        foreach ($directory in $directories) {
            $paths += Join-Path $directory.FullName "settings.json"
        }
    }
    return @($paths | Sort-Object -Unique)
}

function Get-StartupShortcuts {
    $startupDirectories = @(
        [Environment]::GetFolderPath("Startup"),
        [Environment]::GetFolderPath("CommonStartup")
    ) | Where-Object { -not [string]::IsNullOrWhiteSpace($_) } | Sort-Object -Unique

    $shell = $null
    try {
        $shell = New-Object -ComObject WScript.Shell
        foreach ($directory in $startupDirectories) {
            if (-not (Test-Path -LiteralPath $directory)) {
                continue
            }
            foreach ($file in @(Get-ChildItem -LiteralPath $directory -Filter "*.lnk" -File -ErrorAction SilentlyContinue)) {
                $shortcut = $shell.CreateShortcut($file.FullName)
                if ($file.Name -like "*OmniSSHAgent*" -or $shortcut.TargetPath -like "*OmniSSHAgent*") {
                    [pscustomobject]@{
                        Shortcut = $file.FullName
                        Target   = $shortcut.TargetPath
                    }
                }
            }
        }
    } catch {
        Write-Warning "Could not inspect Startup shortcuts: $($_.Exception.Message)"
    } finally {
        if ($null -ne $shell) {
            [void][Runtime.InteropServices.Marshal]::FinalReleaseComObject($shell)
        }
    }
}

function Get-InstalledOmniSSHAgent {
    $roots = @(
        "Registry::HKEY_CURRENT_USER\Software\Microsoft\Windows\CurrentVersion\Uninstall",
        "Registry::HKEY_LOCAL_MACHINE\Software\Microsoft\Windows\CurrentVersion\Uninstall",
        "Registry::HKEY_LOCAL_MACHINE\Software\WOW6432Node\Microsoft\Windows\CurrentVersion\Uninstall"
    )
    foreach ($root in $roots) {
        if (-not (Test-Path -LiteralPath $root)) {
            continue
        }
        foreach ($key in @(Get-ChildItem -LiteralPath $root -ErrorAction SilentlyContinue)) {
            $entry = Get-ItemProperty -LiteralPath $key.PSPath -ErrorAction SilentlyContinue
            $displayName = Get-OptionalProperty $entry "DisplayName"
            if ($displayName -like "*OmniSSHAgent*") {
                [pscustomobject]@{
                    Scope           = $root
                    DisplayName     = $displayName
                    DisplayVersion  = Get-OptionalProperty $entry "DisplayVersion"
                    InstallLocation = Get-OptionalProperty $entry "InstallLocation"
                    UninstallString = Get-OptionalProperty $entry "UninstallString"
                }
            }
        }
    }
}

function Get-WSLDiagnostic {
    if (-not (Get-Command wsl.exe -ErrorAction SilentlyContinue)) {
        Write-Detail "WSL" "wsl.exe is unavailable"
        return
    }

    $distributions = @(
        & wsl.exe --list --quiet 2>$null |
            ForEach-Object { ([string]$_).Replace([char]0, "").Trim() } |
            Where-Object { -not [string]::IsNullOrWhiteSpace($_) }
    )
    if ($distributions.Count -eq 0) {
        Write-Detail "WSL distributions" "none"
        return
    }

    $probe = @'
for f in "$HOME/.bashrc" "$HOME/.zshrc" "$HOME/.profile" "$HOME/.config/fish/config.fish"; do
  if [ -f "$f" ]; then
    grep -nE 'wsl2-ssh-agent-proxy|omni-socat|OmniSSHAgent|SSH_AUTH_SOCK' "$f" 2>/dev/null | sed "s#^#$f:#"
  fi
done
ps -ef 2>/dev/null | grep -E 'wsl2-ssh-agent-proxy|omni-socat' | grep -v grep || true
for p in "$HOME/wsl2-ssh-agent-proxy" "$HOME/.ssh/wsl2-ssh-agent-proxy" "$HOME/omni-socat" "$HOME/.ssh/agent.sock"; do
  [ -e "$p" ] && printf 'legacy-path: %s\n' "$p"
done
'@

    foreach ($distribution in $distributions) {
        Write-Host "-- $distribution --"
        try {
            $output = @(& wsl.exe -d $distribution -- sh -c $probe 2>&1)
            if ($output.Count -eq 0) {
                Write-Host "(no legacy WSL references found)"
            } else {
                $output | ForEach-Object { Write-Host $_ }
            }
        } catch {
            Write-Warning "Could not inspect WSL distribution ${distribution}: $($_.Exception.Message)"
        }
    }
}

Write-Host "OmniSSHAgent legacy migration diagnostic"
Write-Host "Read-only mode: no processes, services, registry values, files, or shell profiles will be changed."
Write-Host "Stored passphrases are not read or displayed."

Write-Section "Running processes"
$currentInstallPath = $null
if (-not [string]::IsNullOrWhiteSpace($env:LOCALAPPDATA)) {
    $currentInstallPath = [IO.Path]::GetFullPath(
        (Join-Path $env:LOCALAPPDATA "Programs\OmniSSHAgent\OmniSSHAgent.exe")
    )
}
try {
    $processes = @(
        Get-CimInstance -ClassName Win32_Process -Filter "Name = 'OmniSSHAgent.exe'" -ErrorAction Stop
    )
    if ($processes.Count -eq 0) {
        Write-Host "(none)"
    }
    foreach ($process in $processes) {
        $kind = "legacy or unknown"
        if (-not [string]::IsNullOrWhiteSpace($process.ExecutablePath) -and
            $null -ne $currentInstallPath -and
            [IO.Path]::GetFullPath($process.ExecutablePath).Equals(
                $currentInstallPath,
                [StringComparison]::OrdinalIgnoreCase
            )) {
            $kind = "redesigned default installation"
        }
        Write-Host ("PID {0}: {1} [{2}]" -f $process.ProcessId, $process.ExecutablePath, $kind)
    }
} catch {
    Write-Warning "Could not inspect OmniSSHAgent processes: $($_.Exception.Message)"
}
$pageantProcesses = @(Get-Process -Name "pageant" -ErrorAction SilentlyContinue)
Write-Detail "PuTTY Pageant processes" $pageantProcesses.Count

Write-Section "Legacy settings and key references"
$settingsPaths = @(Get-LegacySettingsPaths)
$settingsFound = 0
$cmdkeyOutput = ""
if (Get-Command cmdkey.exe -ErrorAction SilentlyContinue) {
    $cmdkeyOutput = (& cmdkey.exe /list 2>$null | Out-String)
}
foreach ($settingsPath in $settingsPaths) {
    if (-not (Test-Path -LiteralPath $settingsPath -PathType Leaf)) {
        continue
    }
    $settingsFound++
    Write-Host "-- $settingsPath --"
    try {
        $settings = Get-Content -LiteralPath $settingsPath -Raw | ConvertFrom-Json
        foreach ($name in @(
                "StartHidden",
                "PageantAgent",
                "NamedPipeAgent",
                "UnixSocketAgent",
                "CygWinAgent",
                "ShowBalloon",
                "UnixSocketPath",
                "CygWinSocketPath",
                "ProxyModeOfNamedPipe",
                "DebugLog"
            )) {
            Write-Detail $name (Get-OptionalProperty $settings $name)
        }

        $keys = @(Get-OptionalProperty $settings "Keys")
        if ($keys.Count -eq 0 -or $null -eq $keys[0]) {
            Write-Host "Keys: (none)"
        } else {
            Write-Host "Keys:"
            foreach ($key in $keys) {
                $id = [string](Get-OptionalProperty $key "ID")
                $filePath = [string](Get-OptionalProperty $key "FilePath")
                $publicKey = Get-OptionalProperty $key "PublicKey"
                $fingerprint = Get-OptionalProperty $publicKey "SHA256"
                $exists = -not [string]::IsNullOrWhiteSpace($filePath) -and
                    (Test-Path -LiteralPath $filePath -PathType Leaf)
                $credentialTarget = if ([string]::IsNullOrWhiteSpace($id)) {
                    "(missing key ID)"
                } else {
                    "OmniSSHAgent:$id"
                }
                $credentialListed = -not [string]::IsNullOrWhiteSpace($id) -and
                    $cmdkeyOutput.IndexOf($credentialTarget, [StringComparison]::OrdinalIgnoreCase) -ge 0
                Write-Host ("  File: {0}" -f $filePath)
                Write-Host ("    Exists: {0}" -f $exists)
                Write-Host ("    Fingerprint: {0}" -f $fingerprint)
                Write-Host ("    Credential target: {0}" -f $credentialTarget)
                Write-Host ("    Credential listed by cmdkey: {0}" -f $credentialListed)
            }
        }
    } catch {
        Write-Warning "Could not decode ${settingsPath}: $($_.Exception.Message)"
    }
}
if ($settingsFound -eq 0) {
    Write-Host "(no legacy settings.json found)"
}

Write-Section "Windows OpenSSH backend"
try {
    $service = Get-CimInstance -ClassName Win32_Service -Filter "Name = 'ssh-agent'" -ErrorAction Stop
    if ($null -eq $service) {
        Write-Host "ssh-agent service: (not installed)"
    } else {
        Write-Detail "State" $service.State
        Write-Detail "Start mode" $service.StartMode
        Write-Detail "Service account" $service.StartName
    }
} catch {
    Write-Warning "Could not inspect ssh-agent service: $($_.Exception.Message)"
}
Write-Detail "OpenSSH agent Named Pipe exists" (Test-Path -LiteralPath "\\.\pipe\openssh-ssh-agent")

Write-Section "Autostart"
$runKey = "Registry::HKEY_CURRENT_USER\Software\Microsoft\Windows\CurrentVersion\Run"
if (Test-Path -LiteralPath $runKey) {
    $runValues = Get-ItemProperty -LiteralPath $runKey
    $runValue = $runValues.PSObject.Properties["OmniSSHAgent"]
    Write-Detail "HKCU Run: OmniSSHAgent" $(if ($null -eq $runValue) { $null } else { $runValue.Value })
} else {
    Write-Detail "HKCU Run: OmniSSHAgent" $null
}
$shortcuts = @(Get-StartupShortcuts)
if ($shortcuts.Count -eq 0) {
    Write-Host "Startup shortcuts: (none)"
} else {
    Write-Host "Startup shortcuts:"
    $shortcuts | ForEach-Object {
        Write-Host ("  {0} -> {1}" -f $_.Shortcut, $_.Target)
    }
}

Write-Section "Installed application records"
$installedEntries = @(Get-InstalledOmniSSHAgent)
if ($installedEntries.Count -eq 0) {
    Write-Host "(none)"
} else {
    $installedEntries | Format-List
}

Write-Section "Socket files and SSH_AUTH_SOCK"
$socketCandidates = @()
if (-not [string]::IsNullOrWhiteSpace($env:USERPROFILE)) {
    $socketCandidates = @(
        (Join-Path $env:USERPROFILE "OmniSSHAgent.sock"),
        (Join-Path $env:USERPROFILE "OmniSSHCygwin.sock"),
        (Join-Path $env:USERPROFILE ".ssh\omnisshagent-cygwin.sock"),
        (Join-Path $env:USERPROFILE ".ssh\omnisshagent-cygwin.sock.owner")
    )
}
foreach ($socketPath in $socketCandidates) {
    Write-Detail $socketPath (Test-Path -LiteralPath $socketPath)
}
foreach ($scope in @("Process", "User", "Machine")) {
    Write-Detail "SSH_AUTH_SOCK ($scope)" ([Environment]::GetEnvironmentVariable("SSH_AUTH_SOCK", $scope))
}

Write-Section "WSL"
if ($IncludeWSL) {
    Write-Host "WSL inspection was requested. Distributions may be started temporarily."
    Get-WSLDiagnostic
} else {
    Write-Host "Skipped. Re-run from a checkout with -IncludeWSL to inspect WSL shell profiles and legacy proxy paths."
}

Write-Section "Next steps"
Write-Host "Review docs/migration-from-legacy.md."
Write-Host "Back up every reported settings.json before making changes."
Write-Host "Do not delete private-key files or Credential Manager entries until the redesigned installation is verified."
