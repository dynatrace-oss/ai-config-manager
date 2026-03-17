param(
    [string]$Version = $env:AIMGR_VERSION,
    [string]$InstallDir = $env:AIMGR_INSTALL_DIR,
    [string]$Repo = $(if ($env:AIMGR_GITHUB_REPO) { $env:AIMGR_GITHUB_REPO } else { "dynatrace-oss/ai-config-manager" })
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

function Write-Info {
    param([string]$Message)

    Write-Host "aimgr install: $Message"
}

function Get-Architecture {
    $architecture = [System.Runtime.InteropServices.RuntimeInformation]::OSArchitecture.ToString().ToLowerInvariant()

    switch ($architecture) {
        'x64' { return 'amd64' }
        'arm64' { return 'arm64' }
        default { throw "Unsupported architecture: $architecture" }
    }
}

function Get-NormalizedPathEntry {
    param([string]$PathEntry)

    return $PathEntry.Trim().TrimEnd('\\').ToLowerInvariant()
}

function Test-PathContainsEntry {
    param(
        [string]$PathValue,
        [string]$Entry
    )

    if ([string]::IsNullOrWhiteSpace($PathValue)) {
        return $false
    }

    $normalizedEntry = Get-NormalizedPathEntry -PathEntry $Entry

    foreach ($existingEntry in ($PathValue -split ';')) {
        if ([string]::IsNullOrWhiteSpace($existingEntry)) {
            continue
        }

        if ((Get-NormalizedPathEntry -PathEntry $existingEntry) -ieq $normalizedEntry) {
            return $true
        }
    }

    return $false
}

if ([string]::IsNullOrWhiteSpace($InstallDir)) {
    $InstallDir = Join-Path $env:LOCALAPPDATA 'Programs\aimgr\bin'
}

if ([string]::IsNullOrWhiteSpace($Version)) {
    $latestRelease = Invoke-RestMethod -Uri "https://api.github.com/repos/$Repo/releases/latest" -Headers @{ 'User-Agent' = 'aimgr-install-script' }
    $Version = $latestRelease.tag_name
}

$architecture = Get-Architecture
$asset = "aimgr_${Version}_windows_${architecture}.zip"
$checksumsAsset = 'checksums.txt'
$releaseBaseUrl = "https://github.com/$Repo/releases/download/$Version"
$tempDir = Join-Path ([System.IO.Path]::GetTempPath()) ("aimgr-install-" + [System.Guid]::NewGuid().ToString())

New-Item -ItemType Directory -Path $tempDir -Force | Out-Null

try {
    $zipPath = Join-Path $tempDir $asset
    $checksumsPath = Join-Path $tempDir $checksumsAsset
    $extractDir = Join-Path $tempDir 'extract'
    $binaryPath = Join-Path $extractDir 'aimgr.exe'

    Write-Info "Downloading aimgr $Version for windows/$architecture..."
    Invoke-WebRequest -Uri "$releaseBaseUrl/$asset" -OutFile $zipPath
    Invoke-WebRequest -Uri "$releaseBaseUrl/$checksumsAsset" -OutFile $checksumsPath

    $expectedChecksumLine = Get-Content $checksumsPath | Where-Object {
        $_ -match ("(?:\s|\*)" + [regex]::Escape($asset) + '$')
    } | Select-Object -First 1

    if (-not $expectedChecksumLine) {
        throw "Checksum not found for $asset"
    }

    $expectedChecksum = ($expectedChecksumLine -split '\s+')[0].ToLowerInvariant()
    $actualChecksum = (Get-FileHash -Path $zipPath -Algorithm SHA256).Hash.ToLowerInvariant()

    if ($actualChecksum -ne $expectedChecksum) {
        throw "Checksum verification failed for $asset"
    }

    Expand-Archive -Path $zipPath -DestinationPath $extractDir -Force

    if (-not (Test-Path -LiteralPath $binaryPath)) {
        throw 'Archive did not contain aimgr.exe'
    }

    New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
    Copy-Item -LiteralPath $binaryPath -Destination (Join-Path $InstallDir 'aimgr.exe') -Force

    $userPath = [Environment]::GetEnvironmentVariable('Path', 'User')

    if (-not (Test-PathContainsEntry -PathValue $userPath -Entry $InstallDir)) {
        $newUserPath = if ([string]::IsNullOrWhiteSpace($userPath)) {
            $InstallDir
        } else {
            "$userPath;$InstallDir"
        }

        [Environment]::SetEnvironmentVariable('Path', $newUserPath, 'User')

        if ([string]::IsNullOrWhiteSpace($env:Path)) {
            $env:Path = $InstallDir
        } else {
            $env:Path = "$env:Path;$InstallDir"
        }

        Write-Info "Added $InstallDir to your user PATH"
    }

    Write-Info "Installed aimgr to $(Join-Path $InstallDir 'aimgr.exe')"
    Write-Info 'Open a new terminal, then run: aimgr --version'
}
finally {
    if (Test-Path -LiteralPath $tempDir) {
        Remove-Item -LiteralPath $tempDir -Recurse -Force
    }
}
