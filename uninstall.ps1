[CmdletBinding()]
param(
    [string]$InstallDirectory = (Join-Path $env:LOCALAPPDATA "Programs\OmniSSHAgent"),
    [string]$ShortcutPath = (Join-Path ([Environment]::GetFolderPath("Programs")) "OmniSSHAgent.lnk"),
    [string]$ShutdownEventName = "Local\OmniSSHAgent-Shutdown",
    [switch]$Purge
)

$ErrorActionPreference = "Stop"
Set-StrictMode -Version Latest

if ($env:OS -ne "Windows_NT") {
    throw "OmniSSHAgent supports Windows only."
}

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

$executable = Join-Path $InstallDirectory "OmniSSHAgent.exe"
Stop-InstalledOmniSSHAgent $executable $ShutdownEventName

if (Test-Path -LiteralPath $executable) {
    Remove-Item -LiteralPath $executable -Force
}
if (Test-Path -LiteralPath $ShortcutPath) {
    Remove-Item -LiteralPath $ShortcutPath -Force
}

if (Test-Path -LiteralPath $InstallDirectory) {
    $remaining = @(Get-ChildItem -LiteralPath $InstallDirectory -Force)
    if ($remaining.Count -eq 0) {
        Remove-Item -LiteralPath $InstallDirectory -Force
    } else {
        Write-Warning "The install directory contains other files and was retained: $InstallDirectory"
    }
}

if ($Purge) {
    $dataDirectories = @(
        (Join-Path $env:APPDATA "OmniSSHAgent"),
        (Join-Path $env:LOCALAPPDATA "OmniSSHAgent")
    )
    foreach ($directory in $dataDirectories) {
        if (Test-Path -LiteralPath $directory) {
            Remove-Item -LiteralPath $directory -Recurse -Force
        }
    }
    Write-Host "Uninstalled OmniSSHAgent and removed its configuration and logs."
} else {
    Write-Host "Uninstalled OmniSSHAgent. Configuration and logs were retained."
}
