$ErrorActionPreference = 'Stop'
$HostName = 'com.tunneldeck.native'
$ManifestPath = Join-Path $env:LOCALAPPDATA "TunnelDeck\NativeMessagingHosts\$HostName.json"
$RegistryPath = "HKCU:\Software\Google\Chrome\NativeMessagingHosts\$HostName"

if (Test-Path -LiteralPath $RegistryPath) {
  Remove-Item -LiteralPath $RegistryPath -Recurse -Force
}
if (Test-Path -LiteralPath $ManifestPath) {
  Remove-Item -LiteralPath $ManifestPath -Force
}

Write-Host "Removed $HostName for Google Chrome"
