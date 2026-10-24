$ErrorActionPreference = 'Stop'

$version = '1.0.6'
$url64 = "https://github.com/wrenyn/kexoz-cli/releases/download/v$version/kexoz-windows-amd64.exe"

$packageArgs = @{
  packageName   = $env:ChocolateyPackageName
  unzipLocation = $env:ChocolateyBinRoot
  fileType      = 'exe'
  url64bit      = $url64
  softwareName  = 'Kexoz*'
  silentArgs    = '/S'
  validExitCodes = @(0)
  checksum64    = 'PLACEHOLDER_SHA256'
  checksumType64 = 'sha256'
}

Install-ChocolateyPackage @packageArgs
