[CmdletBinding()]
param(
    [string]$Version = $env:OMNISSHAGENT_VERSION,
    [string]$InstallDirectory = (Join-Path $env:LOCALAPPDATA "Programs\OmniSSHAgent"),
    [string]$ShortcutPath = (Join-Path ([Environment]::GetFolderPath("Programs")) "OmniSSHAgent.lnk"),
    [string]$DownloadBaseUrl = "https://github.com/masahide/OmniSSHAgent/releases",
    [string]$ShutdownEventName = "Local\OmniSSHAgent-Shutdown",
    [switch]$NoLaunch,
    [switch]$NoShortcut
)

$ErrorActionPreference = "Stop"
$ProgressPreference = "SilentlyContinue"
Set-StrictMode -Version Latest

$assetName = "OmniSSHAgent-windows-amd64.exe"
$checksumName = "$assetName.sha256"
$executableName = "OmniSSHAgent.exe"

if (-not ("OmniSSHAgent.ProcessControlNativeMethods" -as [type])) {
    Add-Type -Namespace "OmniSSHAgent" -Name "ProcessControlNativeMethods" -MemberDefinition @"
[System.Runtime.InteropServices.DllImport("kernel32.dll", SetLastError = true, CharSet = System.Runtime.InteropServices.CharSet.Unicode)]
public static extern System.IntPtr OpenEvent(uint desiredAccess, bool inheritHandle, string name);
[System.Runtime.InteropServices.DllImport("kernel32.dll", SetLastError = true)]
public static extern bool SetEvent(System.IntPtr handle);
[System.Runtime.InteropServices.DllImport("kernel32.dll")]
public static extern bool CloseHandle(System.IntPtr handle);
"@
}

function Get-InstalledOmniSSHAgentProcess {
    param([Parameter(Mandatory)][string]$Executable)

    $fullExecutable = [IO.Path]::GetFullPath($Executable)
    @(
        Get-CimInstance -ClassName Win32_Process -Filter "Name = 'OmniSSHAgent.exe'" |
            Where-Object {
                try {
                    -not [string]::IsNullOrWhiteSpace($_.ExecutablePath) -and
                    [IO.Path]::GetFullPath($_.ExecutablePath).Equals(
                        $fullExecutable,
                        [StringComparison]::OrdinalIgnoreCase
                    )
                } catch {
                    $false
                }
            }
    )
}

function Request-OmniSSHAgentShutdown {
    param([Parameter(Mandatory)][string]$Name)

    $eventModifyState = 0x0002
    $handle = [OmniSSHAgent.ProcessControlNativeMethods]::OpenEvent(
        $eventModifyState,
        $false,
        $Name
    )
    if ($handle -eq [IntPtr]::Zero) {
        return $false
    }
    try {
        if (-not [OmniSSHAgent.ProcessControlNativeMethods]::SetEvent($handle)) {
            $errorCode = [Runtime.InteropServices.Marshal]::GetLastWin32Error()
            throw [ComponentModel.Win32Exception]::new($errorCode)
        }
        return $true
    } finally {
        [void][OmniSSHAgent.ProcessControlNativeMethods]::CloseHandle($handle)
    }
}

function Stop-InstalledOmniSSHAgent {
    param(
        [Parameter(Mandatory)][string]$Executable,
        [Parameter(Mandatory)][string]$EventName
    )

    $processes = @(Get-InstalledOmniSSHAgentProcess $Executable)
    if ($processes.Count -eq 0) {
        return
    }

    Write-Host "Stopping the running OmniSSHAgent..."
    $requested = Request-OmniSSHAgentShutdown $EventName
    if ($requested) {
        $deadline = [DateTime]::UtcNow.AddSeconds(25)
        do {
            Start-Sleep -Milliseconds 100
            $processes = @(Get-InstalledOmniSSHAgentProcess $Executable)
        } while ($processes.Count -gt 0 -and [DateTime]::UtcNow -lt $deadline)
    }

    $processes = @(Get-InstalledOmniSSHAgentProcess $Executable)
    if ($processes.Count -gt 0) {
        Write-Warning "The running version did not support or complete graceful shutdown; stopping only the installed executable."
        $processes | ForEach-Object {
            Stop-Process -Id $_.ProcessId -Force -ErrorAction Stop
        }
        $deadline = [DateTime]::UtcNow.AddSeconds(5)
        do {
            Start-Sleep -Milliseconds 100
            $processes = @(Get-InstalledOmniSSHAgentProcess $Executable)
        } while ($processes.Count -gt 0 -and [DateTime]::UtcNow -lt $deadline)
    }
    if ($processes.Count -gt 0) {
        throw "Could not stop the installed OmniSSHAgent process."
    }
}

function Receive-InstallerFile {
    param(
        [Parameter(Mandatory)]
        [string]$Uri,
        [Parameter(Mandatory)]
        [string]$Destination
    )

    $parsed = [Uri]$Uri
    if ($parsed.IsFile) {
        Copy-Item -LiteralPath $parsed.LocalPath -Destination $Destination
        return
    }
    if ($parsed.Scheme -ne "https") {
        throw "Refusing non-HTTPS download URL: $Uri"
    }
    Invoke-WebRequest -UseBasicParsing -Uri $Uri -OutFile $Destination
}

if ($env:OS -ne "Windows_NT") {
    throw "OmniSSHAgent supports Windows only."
}
if (-not [Environment]::Is64BitOperatingSystem) {
    throw "OmniSSHAgent requires 64-bit Windows."
}
if ([string]::IsNullOrWhiteSpace($env:LOCALAPPDATA) -and
    [string]::IsNullOrWhiteSpace($InstallDirectory)) {
    throw "LOCALAPPDATA is unavailable; specify -InstallDirectory."
}

$releaseUrl = $DownloadBaseUrl.TrimEnd("/")
if ([string]::IsNullOrWhiteSpace($Version) -or $Version -eq "latest") {
    $releaseUrl += "/latest/download"
} else {
    $releaseUrl += "/download/$([Uri]::EscapeDataString($Version))"
}

$temporaryDirectory = Join-Path ([IO.Path]::GetTempPath()) (
    "omnisshagent-install-" + [Guid]::NewGuid().ToString("N")
)
New-Item -ItemType Directory -Path $temporaryDirectory | Out-Null

try {
    $downloadedExecutable = Join-Path $temporaryDirectory $assetName
    $downloadedChecksum = Join-Path $temporaryDirectory $checksumName

    Write-Host "Downloading OmniSSHAgent..."
    Receive-InstallerFile "$releaseUrl/$assetName" $downloadedExecutable
    Receive-InstallerFile "$releaseUrl/$checksumName" $downloadedChecksum

    $checksumText = Get-Content -LiteralPath $downloadedChecksum -Raw
    $pattern = "(?im)^\s*([a-f0-9]{64})\s+\*?$([regex]::Escape($assetName))\s*$"
    $match = [regex]::Match($checksumText, $pattern)
    if (-not $match.Success) {
        throw "The release checksum file has an invalid format."
    }
    $expectedHash = $match.Groups[1].Value.ToUpperInvariant()
    $actualHash = (Get-FileHash -LiteralPath $downloadedExecutable -Algorithm SHA256).Hash
    if ($actualHash -ne $expectedHash) {
        throw "SHA-256 verification failed for $assetName."
    }

    New-Item -ItemType Directory -Path $InstallDirectory -Force | Out-Null
    $destination = Join-Path $InstallDirectory $executableName
    Stop-InstalledOmniSSHAgent $destination $ShutdownEventName
    try {
        Move-Item -LiteralPath $downloadedExecutable -Destination $destination -Force
    } catch {
        throw "Could not replace $destination. $($_.Exception.Message)"
    }

    if (-not $NoShortcut) {
        try {
            $shell = New-Object -ComObject WScript.Shell
            $shortcut = $shell.CreateShortcut($ShortcutPath)
            $shortcut.TargetPath = $destination
            $shortcut.WorkingDirectory = $InstallDirectory
            $shortcut.Description = "OmniSSHAgent"
            $shortcut.Save()
        } catch {
            Write-Warning "Installed successfully, but the Start menu shortcut could not be created: $($_.Exception.Message)"
        }
    }

    Write-Host "Installed OmniSSHAgent to $destination"
    if (-not $NoLaunch) {
        try {
            Start-Process -FilePath $destination | Out-Null
            Write-Host "OmniSSHAgent is running in the notification area."
        } catch {
            Write-Warning "Installed successfully, but could not start OmniSSHAgent: $($_.Exception.Message)"
        }
    }
} finally {
    if (Test-Path -LiteralPath $temporaryDirectory) {
        Remove-Item -LiteralPath $temporaryDirectory -Recurse -Force
    }
}
