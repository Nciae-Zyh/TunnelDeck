param(
  [string]$Version = $(if ($env:TUNNELDECK_VERSION) { $env:TUNNELDECK_VERSION } else { 'v0.3.3' }),
  [string]$ExtensionId = $env:TUNNELDECK_EXTENSION_ID,
  [string]$ChromeStoreId = $env:TUNNELDECK_CHROME_STORE_ID,
  [string]$InstallDirectory = $env:TUNNELDECK_INSTALL_DIR
)

$ErrorActionPreference = 'Stop'
$ProgressPreference = 'SilentlyContinue'

$GoVersion = '1.26.5'
$NodeVersion = '22.23.2'
$Repository = 'Nciae-Zyh/TunnelDeck'
$OfficialChromeExtensionId = 'jnfkjehpbkmfnidfcilehhkpbjjinmod'

if ($Version -notmatch '^v[0-9]+\.[0-9]+\.[0-9]+$') {
  throw 'Version must use the vX.Y.Z format.'
}
if (-not $ExtensionId -and -not $ChromeStoreId) {
  $ChromeStoreId = $OfficialChromeExtensionId
}
if ($ExtensionId -and $ChromeStoreId) {
  throw 'Use either TUNNELDECK_EXTENSION_ID or TUNNELDECK_CHROME_STORE_ID, not both.'
}
foreach ($CandidateId in @($ExtensionId, $ChromeStoreId)) {
  if ($CandidateId -and $CandidateId -notmatch '^[a-p]{32}$') {
    throw 'Chrome extension ID must contain exactly 32 letters from a to p.'
  }
}

if (-not [System.Runtime.InteropServices.RuntimeInformation]::IsOSPlatform(
    [System.Runtime.InteropServices.OSPlatform]::Windows
  )) {
  throw 'This installer is for Windows. Use install.sh on macOS or Linux.'
}
if ([System.Runtime.InteropServices.RuntimeInformation]::OSArchitecture -ne 'X64') {
  throw 'The Windows source installer currently supports AMD64/x64 systems.'
}

function Test-VersionAtLeast {
  param([string]$Current, [string]$Minimum)
  try {
    return [version]$Current -ge [version]$Minimum
  } catch {
    return $false
  }
}

function Invoke-VerifiedDownload {
  param(
    [string]$Uri,
    [string]$Destination,
    [string]$Sha256
  )
  Invoke-WebRequest -UseBasicParsing -Uri $Uri -OutFile $Destination
  $Actual = (Get-FileHash -Algorithm SHA256 -LiteralPath $Destination).Hash.ToLowerInvariant()
  if ($Actual -ne $Sha256.ToLowerInvariant()) {
    throw "SHA-256 verification failed for $Destination. Expected $Sha256; got $Actual."
  }
}

$InstalledGoVersion = ''
$GoCommand = Get-Command go -ErrorAction SilentlyContinue
if ($GoCommand) {
  $GoVersionOutput = (& go env GOVERSION 2>$null)
  if ($GoVersionOutput -match '^go(.+)$') { $InstalledGoVersion = $Matches[1] }
}
$InstalledNodeVersion = ''
$NodeCommand = Get-Command node -ErrorAction SilentlyContinue
$NpmCommand = Get-Command npm -ErrorAction SilentlyContinue
if ($NodeCommand) {
  $InstalledNodeVersion = (& node -p 'process.versions.node' 2>$null)
}

$WebViewRoots = @(
  (Join-Path ${env:ProgramFiles(x86)} 'Microsoft\EdgeWebView\Application'),
  (Join-Path $env:ProgramFiles 'Microsoft\EdgeWebView\Application'),
  (Join-Path $env:LOCALAPPDATA 'Microsoft\EdgeWebView\Application')
) | Where-Object { $_ -and (Test-Path -LiteralPath $_) }

Write-Host 'TunnelDeck prerequisite check'
Write-Host '  Platform:       Windows/x64'
if ($InstalledGoVersion -and (Test-VersionAtLeast $InstalledGoVersion '1.25.0')) {
  Write-Host "  Go:             $InstalledGoVersion (ready)"
} else {
  Write-Host "  Go:             missing or older than 1.25; isolated $GoVersion will be installed"
}
if ($InstalledNodeVersion -and $NpmCommand -and (Test-VersionAtLeast $InstalledNodeVersion '20.0.0')) {
  Write-Host "  Node.js/npm:    $InstalledNodeVersion / $(& npm --version) (ready)"
} else {
  Write-Host "  Node.js/npm:    missing or older than 20; isolated $NodeVersion will be installed"
}
if ($WebViewRoots.Count -gt 0) {
  Write-Host '  WebView2:       ready'
} else {
  throw 'Microsoft Edge WebView2 Runtime was not detected. Install Evergreen Runtime from https://developer.microsoft.com/microsoft-edge/webview2/ and run this command again.'
}

if ($env:TUNNELDECK_CHECK_ONLY -eq '1') {
  Write-Host 'Prerequisite check passed. Missing Go/Node toolchains can be installed in user space.'
  return
}

$DataHome = Join-Path $env:LOCALAPPDATA 'TunnelDeck'
$CacheHome = Join-Path $DataHome 'Cache'
$ToolchainsHome = Join-Path $DataHome 'Toolchains'
$SourceHome = Join-Path $DataHome 'Source'
New-Item -ItemType Directory -Force -Path $CacheHome, $ToolchainsHome, $SourceHome | Out-Null

if (-not $InstalledGoVersion -or -not (Test-VersionAtLeast $InstalledGoVersion '1.25.0')) {
  $GoHome = Join-Path $ToolchainsHome "Go\$GoVersion"
  $GoExecutable = Join-Path $GoHome 'bin\go.exe'
  if (-not (Test-Path -LiteralPath $GoExecutable)) {
    $GoArchive = Join-Path $CacheHome "go$GoVersion.windows-amd64.zip"
    Write-Host "Downloading Go $GoVersion for Windows/x64..."
    Invoke-VerifiedDownload `
      -Uri "https://go.dev/dl/go$GoVersion.windows-amd64.zip" `
      -Destination $GoArchive `
      -Sha256 '97e6b2a833b6d89f9ff17d25419ac0a7e3b482a044e9ab18cdef834bd834fd38'
    $GoTemp = Join-Path $CacheHome "go-expand-$([guid]::NewGuid())"
    Expand-Archive -LiteralPath $GoArchive -DestinationPath $GoTemp
    New-Item -ItemType Directory -Force -Path (Split-Path $GoHome -Parent) | Out-Null
    if (Test-Path -LiteralPath $GoHome) {
      Move-Item -LiteralPath $GoHome -Destination "$GoHome.incomplete-$(Get-Date -Format yyyyMMddHHmmss)"
    }
    Move-Item -LiteralPath (Join-Path $GoTemp 'go') -Destination $GoHome
    Remove-Item -LiteralPath $GoTemp -Recurse -Force
  }
  $env:PATH = "$(Join-Path $GoHome 'bin');$env:PATH"
}

if (-not $InstalledNodeVersion -or -not $NpmCommand -or -not (Test-VersionAtLeast $InstalledNodeVersion '20.0.0')) {
  $NodeHome = Join-Path $ToolchainsHome "Node\$NodeVersion"
  $NodeExecutable = Join-Path $NodeHome 'node.exe'
  if (-not (Test-Path -LiteralPath $NodeExecutable)) {
    $NodeArchive = Join-Path $CacheHome "node-v$NodeVersion-win-x64.zip"
    Write-Host "Downloading Node.js $NodeVersion for Windows/x64..."
    Invoke-VerifiedDownload `
      -Uri "https://nodejs.org/dist/v$NodeVersion/node-v$NodeVersion-win-x64.zip" `
      -Destination $NodeArchive `
      -Sha256 '1177b4137ba5adaa56354ae40f1080c7450e8ae09cecb47da459d1c52ac99f97'
    $NodeTemp = Join-Path $CacheHome "node-expand-$([guid]::NewGuid())"
    Expand-Archive -LiteralPath $NodeArchive -DestinationPath $NodeTemp
    New-Item -ItemType Directory -Force -Path (Split-Path $NodeHome -Parent) | Out-Null
    if (Test-Path -LiteralPath $NodeHome) {
      Move-Item -LiteralPath $NodeHome -Destination "$NodeHome.incomplete-$(Get-Date -Format yyyyMMddHHmmss)"
    }
    Move-Item -LiteralPath (Join-Path $NodeTemp "node-v$NodeVersion-win-x64") -Destination $NodeHome
    Remove-Item -LiteralPath $NodeTemp -Recurse -Force
  }
  $env:PATH = "$NodeHome;$env:PATH"
}

Write-Host "Environment ready: $(& go version); node $(& node --version); npm $(& npm --version)"

$SourceDirectory = if ($env:TUNNELDECK_SOURCE_DIR) {
  $env:TUNNELDECK_SOURCE_DIR
} else {
  Join-Path $SourceHome $Version
}
$NestedInstaller = Join-Path $SourceDirectory 'scripts\install-from-source.ps1'
if (-not (Test-Path -LiteralPath $NestedInstaller)) {
  $SourceArchive = Join-Path $CacheHome "TunnelDeck-$Version.zip"
  Write-Host "Downloading TunnelDeck $Version source..."
  Invoke-WebRequest -UseBasicParsing `
    -Uri "https://github.com/$Repository/archive/refs/tags/$Version.zip" `
    -OutFile $SourceArchive
  $SourceTemp = Join-Path $CacheHome "source-expand-$([guid]::NewGuid())"
  Expand-Archive -LiteralPath $SourceArchive -DestinationPath $SourceTemp
  $ExtractedDirectory = Join-Path $SourceTemp "TunnelDeck-$($Version.Substring(1))"
  if (-not (Test-Path -LiteralPath (Join-Path $ExtractedDirectory 'scripts\install-from-source.ps1'))) {
    throw 'Downloaded source archive has an unexpected layout.'
  }
  New-Item -ItemType Directory -Force -Path (Split-Path $SourceDirectory -Parent) | Out-Null
  if (Test-Path -LiteralPath $SourceDirectory) {
    $IncompleteSource = "$SourceDirectory.incomplete-$(Get-Date -Format yyyyMMddHHmmss)"
    Move-Item -LiteralPath $SourceDirectory -Destination $IncompleteSource
    Write-Host "Incomplete source moved to: $IncompleteSource"
  }
  Move-Item -LiteralPath $ExtractedDirectory -Destination $SourceDirectory
  Remove-Item -LiteralPath $SourceTemp -Recurse -Force
  $NestedInstaller = Join-Path $SourceDirectory 'scripts\install-from-source.ps1'
}

$InstallerArguments = @{}
if ($InstallDirectory) { $InstallerArguments.InstallDirectory = $InstallDirectory }
if ($ChromeStoreId) {
  $InstallerArguments.ExtensionId = $ChromeStoreId
  $InstallerArguments.SkipExtensionBuild = $true
} elseif ($ExtensionId) {
  $InstallerArguments.ExtensionId = $ExtensionId
}

Set-ExecutionPolicy -Scope Process -ExecutionPolicy Bypass -Force
& $NestedInstaller @InstallerArguments

if ($ChromeStoreId) {
  Write-Host "Chrome Web Store: https://chromewebstore.google.com/detail/$ChromeStoreId"
}
