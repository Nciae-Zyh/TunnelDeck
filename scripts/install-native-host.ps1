param(
  [Parameter(Mandatory = $true)]
  [ValidatePattern('^[a-p]{32}$')]
  [string]$ExtensionId,

  [string]$BinaryPath = "$env:LOCALAPPDATA\Programs\TunnelDeck\TunnelDeck.exe"
)

$ErrorActionPreference = 'Stop'
$HostName = 'com.tunneldeck.native'
$ResolvedBinary = (Resolve-Path -LiteralPath $BinaryPath).Path
$ManifestDirectory = Join-Path $env:LOCALAPPDATA 'TunnelDeck\NativeMessagingHosts'
$ManifestPath = Join-Path $ManifestDirectory "$HostName.json"

New-Item -ItemType Directory -Force -Path $ManifestDirectory | Out-Null
$Manifest = [ordered]@{
  name = $HostName
  description = 'TunnelDeck SSH tunnel controller'
  path = $ResolvedBinary
  type = 'stdio'
  allowed_origins = @("chrome-extension://$ExtensionId/")
} | ConvertTo-Json -Depth 4

$Utf8WithoutBom = New-Object System.Text.UTF8Encoding($false)
[System.IO.File]::WriteAllText($ManifestPath, $Manifest, $Utf8WithoutBom)

$RegistryPath = "HKCU:\Software\Google\Chrome\NativeMessagingHosts\$HostName"
New-Item -Path $RegistryPath -Force | Out-Null
Set-Item -Path $RegistryPath -Value $ManifestPath

Write-Host "Installed $HostName for Chrome extension $ExtensionId"
Write-Host "Manifest: $ManifestPath"
Write-Host "Binary:   $ResolvedBinary"
