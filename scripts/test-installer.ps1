[CmdletBinding()]
param()

$ErrorActionPreference = "Stop"
Set-StrictMode -Version Latest

$repositoryRoot = Split-Path -Parent $PSScriptRoot
$installer = Join-Path $repositoryRoot "install.ps1"
$uninstaller = Join-Path $repositoryRoot "uninstall.ps1"
$testRoot = Join-Path ([IO.Path]::GetTempPath()) (
    "omnisshagent-installer-test-" + [Guid]::NewGuid().ToString("N")
)
$runKey = "HKCU:\Software\Microsoft\Windows\CurrentVersion\Run"
$autoStartValueName = $null

New-Item -ItemType Directory -Path $testRoot | Out-Null
try {
    $releaseRoot = Join-Path $testRoot "releases"
    $latestDirectory = Join-Path $releaseRoot "latest\download"
    New-Item -ItemType Directory -Path $latestDirectory | Out-Null

    $assetName = "OmniSSHAgent-windows-amd64.exe"
    $asset = Join-Path $latestDirectory $assetName
    Push-Location $repositoryRoot
    try {
        go build -trimpath -ldflags="-H=windowsgui" -o $asset ./cmd/omnisshagent
        if ($LASTEXITCODE -ne 0) {
            throw "test release build failed"
        }
    } finally {
        Pop-Location
    }

    $hash = (Get-FileHash -LiteralPath $asset -Algorithm SHA256).Hash.ToLowerInvariant()
    "$hash  $assetName" |
        Set-Content -LiteralPath "$asset.sha256" -Encoding Ascii

    $baseUrl = ([Uri]$releaseRoot).AbsoluteUri
    $installDirectory = Join-Path $testRoot "installed"
    $shortcut = Join-Path $testRoot "OmniSSHAgent.lnk"
    & powershell.exe -NoProfile -NonInteractive -ExecutionPolicy Bypass `
        -File $installer `
        -InstallDirectory $installDirectory `
        -ShortcutPath $shortcut `
        -DownloadBaseUrl $baseUrl `
        -NoLaunch
    if ($LASTEXITCODE -ne 0) {
        throw "first install failed"
    }

    $installed = Join-Path $installDirectory "OmniSSHAgent.exe"
    if (-not (Test-Path -LiteralPath $installed)) {
        throw "installer did not create $installed"
    }
    if (-not (Test-Path -LiteralPath $shortcut)) {
        throw "installer did not create the Start menu shortcut"
    }
    if ((Get-FileHash -LiteralPath $installed -Algorithm SHA256).Hash.ToLowerInvariant() -ne $hash) {
        throw "installed executable differs from the release asset"
    }

    $shutdownOwner = Join-Path $testRoot "shutdown-owner.exe"
    Push-Location $repositoryRoot
    try {
        go build -tags=e2e -trimpath -o $shutdownOwner ./internal/e2e/shutdownowner
        if ($LASTEXITCODE -ne 0) {
            throw "shutdown owner build failed"
        }
    } finally {
        Pop-Location
    }

    Copy-Item -LiteralPath $shutdownOwner -Destination $installed -Force
    $updateEvent = "Local\OmniSSHAgent-Installer-Update-$([Guid]::NewGuid().ToString("N"))"
    $updateOwner = Start-Process -FilePath $installed -ArgumentList $updateEvent -PassThru -WindowStyle Hidden
    Start-Sleep -Milliseconds 500
    if ($updateOwner.HasExited) {
        throw "shutdown owner exited before the update test"
    }
    & powershell.exe -NoProfile -NonInteractive -ExecutionPolicy Bypass `
        -File $installer `
        -InstallDirectory $installDirectory `
        -ShortcutPath $shortcut `
        -DownloadBaseUrl $baseUrl `
        -ShutdownEventName $updateEvent `
        -NoLaunch
    if ($LASTEXITCODE -ne 0) {
        throw "update install failed"
    }
    [void]$updateOwner.WaitForExit(5000)
    if (-not $updateOwner.HasExited -or $updateOwner.ExitCode -ne 0) {
        throw "installer did not gracefully stop the running executable"
    }
    if ((Get-FileHash -LiteralPath $installed -Algorithm SHA256).Hash.ToLowerInvariant() -ne $hash) {
        throw "updated executable differs from the release asset"
    }

    Copy-Item -LiteralPath $shutdownOwner -Destination $installed -Force
    $legacyOwner = Start-Process -FilePath $installed -ArgumentList "--legacy" -PassThru -WindowStyle Hidden
    Start-Sleep -Milliseconds 500
    if ($legacyOwner.HasExited) {
        throw "legacy shutdown owner exited before the update test"
    }
    & powershell.exe -NoProfile -NonInteractive -ExecutionPolicy Bypass `
        -File $installer `
        -InstallDirectory $installDirectory `
        -ShortcutPath $shortcut `
        -DownloadBaseUrl $baseUrl `
        -ShutdownEventName "Local\OmniSSHAgent-Missing-$([Guid]::NewGuid().ToString("N"))" `
        -NoLaunch
    if ($LASTEXITCODE -ne 0) {
        throw "legacy update install failed"
    }
    [void]$legacyOwner.WaitForExit(5000)
    if (-not $legacyOwner.HasExited) {
        throw "installer did not force-stop an event-incompatible installed executable"
    }
    if ((Get-FileHash -LiteralPath $installed -Algorithm SHA256).Hash.ToLowerInvariant() -ne $hash) {
        throw "legacy update executable differs from the release asset"
    }

    $badReleaseRoot = Join-Path $testRoot "bad\releases"
    $badLatestDirectory = Join-Path $badReleaseRoot "latest\download"
    New-Item -ItemType Directory -Path $badLatestDirectory | Out-Null
    Copy-Item -LiteralPath $asset -Destination (Join-Path $badLatestDirectory $assetName)
    "$("0" * 64)  $assetName" |
        Set-Content -LiteralPath (Join-Path $badLatestDirectory "$assetName.sha256") -Encoding Ascii

    $badInstallDirectory = Join-Path $testRoot "bad-installed"
    & powershell.exe -NoProfile -NonInteractive -ExecutionPolicy Bypass `
        -File $installer `
        -InstallDirectory $badInstallDirectory `
        -DownloadBaseUrl ([Uri]$badReleaseRoot).AbsoluteUri `
        -NoLaunch `
        -NoShortcut 2>$null
    if ($LASTEXITCODE -eq 0) {
        throw "installer accepted a mismatched checksum"
    }
    if (Test-Path -LiteralPath (Join-Path $badInstallDirectory "OmniSSHAgent.exe")) {
        throw "installer placed an executable after checksum verification failed"
    }

    Copy-Item -LiteralPath $shutdownOwner -Destination $installed -Force
    $uninstallEvent = "Local\OmniSSHAgent-Installer-Uninstall-$([Guid]::NewGuid().ToString("N"))"
    $autoStartValueName = "OmniSSHAgent-Installer-Test-$([Guid]::NewGuid().ToString("N"))"
    New-Item -Path $runKey -Force | Out-Null
    New-ItemProperty -LiteralPath $runKey -Name $autoStartValueName -Value "`"$installed`"" -PropertyType String -Force | Out-Null
    $uninstallOwner = Start-Process -FilePath $installed -ArgumentList $uninstallEvent -PassThru -WindowStyle Hidden
    Start-Sleep -Milliseconds 500
    if ($uninstallOwner.HasExited) {
        throw "shutdown owner exited before the uninstall test"
    }
    & powershell.exe -NoProfile -NonInteractive -ExecutionPolicy Bypass `
        -File $uninstaller `
        -InstallDirectory $installDirectory `
        -ShortcutPath $shortcut `
        -ShutdownEventName $uninstallEvent `
        -AutoStartValueName $autoStartValueName
    if ($LASTEXITCODE -ne 0) {
        throw "uninstall failed"
    }
    [void]$uninstallOwner.WaitForExit(5000)
    if (-not $uninstallOwner.HasExited -or $uninstallOwner.ExitCode -ne 0) {
        throw "uninstaller did not gracefully stop the running executable"
    }
    if ((Test-Path -LiteralPath $installed) -or (Test-Path -LiteralPath $shortcut)) {
        throw "uninstaller left installed files behind"
    }
    $runProperties = Get-ItemProperty -LiteralPath $runKey
    $runPropertyNames = @($runProperties.PSObject.Properties | ForEach-Object { $_.Name })
    if ($runPropertyNames -contains $autoStartValueName) {
        throw "uninstaller left the autostart registration behind"
    }

    Write-Host "Installer and uninstaller integration tests passed."
} finally {
    if (-not [string]::IsNullOrWhiteSpace($autoStartValueName)) {
        Remove-ItemProperty -LiteralPath $runKey -Name $autoStartValueName -ErrorAction SilentlyContinue
    }
    $resolved = [IO.Path]::GetFullPath($testRoot)
    $tempRoot = [IO.Path]::GetFullPath([IO.Path]::GetTempPath())
    if (-not $resolved.StartsWith($tempRoot, [StringComparison]::OrdinalIgnoreCase)) {
        throw "Refusing unsafe test cleanup path: $resolved"
    }
    Get-CimInstance -ClassName Win32_Process |
        Where-Object {
            -not [string]::IsNullOrWhiteSpace($_.ExecutablePath) -and
            [IO.Path]::GetFullPath($_.ExecutablePath).StartsWith(
                $resolved + [IO.Path]::DirectorySeparatorChar,
                [StringComparison]::OrdinalIgnoreCase
            )
        } |
        ForEach-Object {
            Stop-Process -Id $_.ProcessId -Force -ErrorAction SilentlyContinue
        }
    Start-Sleep -Milliseconds 200
    if (Test-Path -LiteralPath $resolved) {
        Remove-Item -LiteralPath $resolved -Recurse -Force
    }
}
