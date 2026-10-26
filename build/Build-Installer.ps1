param(

 [Parameter(Mandatory=$true)][string]$AppBinary,

 [string]$Output='',

 [string]$Version=(Get-Content -LiteralPath (Join-Path $PSScriptRoot '../frontend/package.json') -Raw | ConvertFrom-Json).version,

 [string]$ReleaseStagingDirectory=(Join-Path (Split-Path (Split-Path (Split-Path $PSScriptRoot -Parent) -Parent) -Parent) '.SmartNote-build')

)

$ErrorActionPreference='Stop'
if ($Version -notmatch '^\d+\.\d+\.\d+$') { throw 'Version must use major.minor.patch format.' }
if (-not $Output) { $Output = "build/bin/SmartNote-Setup-v$Version-windows-x64.exe" }

$projectRoot=Split-Path $PSScriptRoot -Parent

$stagePath=[IO.Path]::GetFullPath($ReleaseStagingDirectory)

$compilerRoot=Join-Path $stagePath 'tools\nsis-3.11'

$compiler=Join-Path $compilerRoot 'makensis.exe'

$compilerHash='F497E92DEB9F179F7B8F7553FCB3BD04F511BB2949E5E4A2AEE80A10F7B20431'

if (-not(Test-Path -LiteralPath $compiler)) {

 $archive=Join-Path $projectRoot '.tmp\nsis-3.11.zip'

 New-Item -ItemType Directory -Force (Split-Path $archive -Parent) | Out-Null

 if (-not(Test-Path -LiteralPath $archive)) {Invoke-WebRequest -UseBasicParsing -Uri 'https://downloads.sourceforge.net/project/nsis/NSIS%203/3.11/nsis-3.11.zip' -OutFile $archive}

 if((Get-FileHash -LiteralPath $archive -Algorithm SHA256).Hash -ne 'C7D27F780DDB6CFFB4730138CD1591E841F4B7EDB155856901CDF5F214394FA1'){throw 'NSIS archive checksum mismatch'}

 New-Item -ItemType Directory -Force (Split-Path $compilerRoot -Parent) | Out-Null

 Add-Type -AssemblyName System.IO.Compression.FileSystem

 [IO.Compression.ZipFile]::ExtractToDirectory($archive, (Split-Path $compilerRoot -Parent))

}

if((Get-FileHash -LiteralPath $compiler -Algorithm SHA256).Hash -ne $compilerHash){throw 'NSIS compiler checksum mismatch'}

$binaryPath=[IO.Path]::GetFullPath($AppBinary)

$outputPath=[IO.Path]::GetFullPath($Output)

$stagePath=[IO.Path]::GetFullPath($ReleaseStagingDirectory)

if([IO.Path]::GetPathRoot($stagePath) -ne [IO.Path]::GetPathRoot($outputPath)){throw 'Installer staging and output must be on the same volume'}

New-Item -ItemType Directory -Force $stagePath | Out-Null

$staged=Join-Path $stagePath ('SmartNote-Setup-'+[guid]::NewGuid().ToString('N')+'.exe')

& $compiler /V2 "/DPROJECT_ROOT=$projectRoot" "/DAPP_BINARY=$binaryPath" "/DAPP_VERSION=$Version" "/DINSTALLER_OUTPUT=$staged" (Join-Path $projectRoot 'build\windows\installer.nsi')

if($LASTEXITCODE -ne 0){throw 'Installer compilation failed'}

$labels=icacls $staged

if($LASTEXITCODE -ne 0 -or ($labels -join "`n") -match 'Low Mandatory Level|低强制|S-1-16-4096'){throw 'Installer has an unsafe Low integrity label'}

New-Item -ItemType Directory -Force (Split-Path $outputPath -Parent) | Out-Null

Move-Item -LiteralPath $staged -Destination $outputPath -Force

Write-Host "安装包构建完成: $outputPath"

