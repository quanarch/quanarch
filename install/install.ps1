# Installs qu and quanarch on Windows.
#
#   irm https://quanarch.org/install.ps1 | iex
#
# $env:QU_VERSION      a release such as 0.2.0 (default: the latest)
# $env:QU_INSTALL_DIR  where the binaries go (default: %LOCALAPPDATA%\quanarch\bin)
$ErrorActionPreference = 'Stop'

$repo = 'quanarch/quanarch'
$dir = if ($env:QU_INSTALL_DIR) { $env:QU_INSTALL_DIR } else { Join-Path $env:LOCALAPPDATA 'quanarch\bin' }
$version = $env:QU_VERSION
if (-not $version) {
  $version = (Invoke-RestMethod "https://api.github.com/repos/$repo/releases/latest").tag_name
}
$version = $version.TrimStart('v')

$arch = if ([System.Runtime.InteropServices.RuntimeInformation]::OSArchitecture -eq 'Arm64') { 'arm64' } else { 'amd64' }
$archive = "quanarch_${version}_windows_$arch.zip"
$base = "https://github.com/$repo/releases/download/v$version"
$tmp = New-Item -ItemType Directory -Path (Join-Path ([IO.Path]::GetTempPath()) ([Guid]::NewGuid()))

try {
  Write-Host "Downloading qu $version for windows/$arch..."
  Invoke-WebRequest "$base/$archive" -OutFile (Join-Path $tmp $archive)
  Invoke-WebRequest "$base/checksums.txt" -OutFile (Join-Path $tmp 'checksums.txt')

  $line = Get-Content (Join-Path $tmp 'checksums.txt') | Where-Object { $_ -match (' ' + [regex]::Escape($archive) + '$') }
  $expected = ($line -split '\s+')[0]
  $actual = (Get-FileHash (Join-Path $tmp $archive) -Algorithm SHA256).Hash.ToLower()
  if (-not $expected -or $expected -ne $actual) { throw "checksum mismatch for $archive" }

  Expand-Archive (Join-Path $tmp $archive) -DestinationPath $tmp
  New-Item -ItemType Directory -Force -Path $dir | Out-Null
  Copy-Item (Join-Path $tmp 'quanarch.exe'), (Join-Path $tmp 'qu.exe') $dir -Force

  $path = [Environment]::GetEnvironmentVariable('Path', 'User')
  if (($path -split ';') -notcontains $dir) {
    [Environment]::SetEnvironmentVariable('Path', "$dir;$path", 'User')
    Write-Host "Added $dir to your PATH; open a new terminal to use qu."
  }
  Write-Host "Installed qu and quanarch $version to $dir"
}
finally {
  Remove-Item -Recurse -Force $tmp
}
