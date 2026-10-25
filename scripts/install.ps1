# Installs the mugon CLI. Served at https://tools.mugon.net/cli/install.ps1.
#   $env:MUGON_INSTALL    install folder (default ~\.mugon\bin)
#   $env:MUGON_TOOLS_URL  tools host (default https://tools.mugon.net)
$ErrorActionPreference = 'Stop'

$arch = if ($env:PROCESSOR_ARCHITECTURE -eq 'ARM64') { 'arm64' } else { 'amd64' }
$platform = "windows-$arch"

$toolsUrl = if ($env:MUGON_TOOLS_URL) { $env:MUGON_TOOLS_URL.TrimEnd('/') } else { 'https://tools.mugon.net' }
$index = Invoke-RestMethod "$toolsUrl/cli/latest.json"
$asset = $index.assets.$platform
if (-not $asset) { throw "no mugon release for $platform" }
$version = $index.version

$installDir = if ($env:MUGON_INSTALL) { $env:MUGON_INSTALL } else { "$env:USERPROFILE\.mugon\bin" }
New-Item -ItemType Directory -Force -Path $installDir | Out-Null

$tmp = [System.IO.Path]::Combine([System.IO.Path]::GetTempPath(), [System.IO.Path]::GetRandomFileName() + '.zip')
try {
    Write-Host "Downloading mugon ${version}..."
    Invoke-WebRequest -Uri $asset.url -OutFile $tmp -UseBasicParsing
    $actual = (Get-FileHash -Path $tmp -Algorithm SHA256).Hash
    if ($actual -ne $asset.sha256.ToUpper()) { throw "checksum mismatch for $($asset.url)" }
    Expand-Archive -Path $tmp -DestinationPath $installDir -Force
} finally {
    Remove-Item $tmp -ErrorAction SilentlyContinue
}

Write-Host "Installed mugon ${version} to ${installDir}\mugon.exe" -ForegroundColor Green

# Add to user PATH permanently (registry) and to the current session
$user = [System.EnvironmentVariableTarget]::User
$currentPath = [System.Environment]::GetEnvironmentVariable('PATH', $user)
if (!(";${currentPath};".ToLower() -like "*;${installDir};*".ToLower())) {
    [System.Environment]::SetEnvironmentVariable('PATH', "${currentPath};${installDir}", $user)
    $env:PATH += ";${installDir}"
    Write-Host ""
    Write-Host "Added ${installDir} to PATH." -ForegroundColor White
}

Write-Host ""
Write-Host "Run 'mugon --help' to get started" -ForegroundColor Green
