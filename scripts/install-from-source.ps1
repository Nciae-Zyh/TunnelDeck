param(
  [ValidatePattern('^[a-p]{32}$')]
  [string]$ExtensionId = '',

  [string]$InstallDirectory = "$env:LOCALAPPDATA\Programs\TunnelDeck"
)

$ErrorActionPreference = 'Stop'

foreach ($CommandName in @('go', 'npm')) {
  if (-not (Get-Command $CommandName -ErrorAction SilentlyContinue)) {
    throw "Required command not found: $CommandName. Install the prerequisites in docs/SOURCE_INSTALL.md, then retry."
  }
}

$RepositoryRoot = (Resolve-Path (Join-Path $PSScriptRoot '..')).Path
Push-Location $RepositoryRoot
try {
  Write-Host 'Building the desktop frontend...'
  & npm --prefix frontend ci
  if ($LASTEXITCODE -ne 0) { throw 'Desktop frontend dependency installation failed.' }

  Write-Host 'Building the Chrome extension...'
  & npm --prefix extension ci
  if ($LASTEXITCODE -ne 0) { throw 'Chrome extension dependency installation failed.' }
  & npm --prefix extension run build
  if ($LASTEXITCODE -ne 0) { throw 'Chrome extension build failed.' }

  Write-Host 'Building TunnelDeck for this Windows system...'
  & go run github.com/wailsapp/wails/v2/cmd/wails@v2.13.0 build -clean
  if ($LASTEXITCODE -ne 0) { throw 'TunnelDeck build failed.' }

  $SourceBinary = Join-Path $RepositoryRoot 'build\bin\TunnelDeck.exe'
  if (-not (Test-Path -LiteralPath $SourceBinary)) {
    throw "Built executable not found: $SourceBinary"
  }
  New-Item -ItemType Directory -Force -Path $InstallDirectory | Out-Null
  $BinaryPath = Join-Path $InstallDirectory 'TunnelDeck.exe'
  Copy-Item -LiteralPath $SourceBinary -Destination $BinaryPath -Force

  $StartMenu = Join-Path $env:APPDATA 'Microsoft\Windows\Start Menu\Programs'
  $ShortcutPath = Join-Path $StartMenu 'TunnelDeck.lnk'
  $Shell = New-Object -ComObject WScript.Shell
  $Shortcut = $Shell.CreateShortcut($ShortcutPath)
  $Shortcut.TargetPath = $BinaryPath
  $Shortcut.WorkingDirectory = $InstallDirectory
  $Shortcut.Save()

  if ($ExtensionId) {
    & (Join-Path $RepositoryRoot 'scripts\install-native-host.ps1') `
      -ExtensionId $ExtensionId `
      -BinaryPath $BinaryPath
  }

  Write-Host ''
  Write-Host 'TunnelDeck installed from locally built source.'
  Write-Host "Desktop application: $BinaryPath"
  Write-Host "Chrome extension directory: $(Join-Path $RepositoryRoot 'extension\dist')"
  if ($ExtensionId) {
    Write-Host "Chrome integration registered for: $ExtensionId"
  } else {
    Write-Host 'After loading extension\dist in chrome://extensions, enter its ID in TunnelDeck to register Chrome integration.'
  }
} finally {
  Pop-Location
}
