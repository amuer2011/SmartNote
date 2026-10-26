param(
    [switch]$SkipInstall,
    [switch]$Installer,
    [string]$Output = 'build/bin/SmartNote.exe',
    [string]$Version = (Get-Content -LiteralPath (Join-Path $PSScriptRoot 'frontend/package.json') -Raw | ConvertFrom-Json).version,
    [string]$ReleaseStagingDirectory = (Join-Path (Split-Path (Split-Path $PSScriptRoot -Parent) -Parent) '.SmartNote-build')
)

$ErrorActionPreference = 'Stop'
if ($Version -notmatch '^\d+\.\d+\.\d+$') { throw 'Version must use major.minor.patch format.' }
$packageVersion = (Get-Content -LiteralPath (Join-Path $PSScriptRoot 'frontend/package.json') -Raw | ConvertFrom-Json).version
if ($Version -ne $packageVersion) { throw "Version $Version differs from frontend/package.json ($packageVersion)." }

Push-Location $PSScriptRoot
$previousTemp = $env:TEMP
$previousTmp = $env:TMP
try {
    $buildTemp = Join-Path $PSScriptRoot '.tmp/build'
    New-Item -ItemType Directory -Force $buildTemp | Out-Null
    $env:TEMP = $buildTemp
    $env:TMP = $buildTemp

    & (Join-Path $PSScriptRoot 'build/Prepare-Embedding.ps1')
    & (Join-Path $PSScriptRoot 'build/windows/Generate-Icon.ps1')

    # Generate Windows resources from the same version used by the UI and installer.
    $resourceConfig = Get-Content -LiteralPath 'build/windows/winres.json' -Raw | ConvertFrom-Json
    foreach ($icon in $resourceConfig.RT_GROUP_ICON.PSObject.Properties) {
        $icon.Value.'0409' = '../../build/windows/icon.ico'
    }
    [xml]$manifest = Get-Content -LiteralPath 'build/windows/app.manifest' -Raw
    $manifest.assembly.assemblyIdentity.version = "$Version.0"
    $manifestPath = Join-Path $buildTemp 'app.manifest'
    $manifest.Save($manifestPath)
    $resourceConfig.RT_MANIFEST.'#1'.'0409' = 'app.manifest'
    $versionInfo = $resourceConfig.RT_VERSION.'#1'.'0000'
    $versionInfo.fixed.file_version = "$Version.0"
    $versionInfo.fixed.product_version = "$Version.0"
    $versionInfo.info.'0409'.FileVersion = $Version
    $versionInfo.info.'0409'.ProductVersion = $Version
    $resourcePath = Join-Path $buildTemp 'winres.json'
    [IO.File]::WriteAllText($resourcePath, ($resourceConfig | ConvertTo-Json -Depth 12), [Text.UTF8Encoding]::new($false))
    go run github.com/tc-hib/go-winres@v0.3.3 make --in $resourcePath --arch amd64 --out rsrc
    if ($LASTEXITCODE -ne 0) { throw 'Windows icon/manifest/version generation failed.' }

    if (-not $SkipInstall) {
        npm ci --prefix frontend
        if ($LASTEXITCODE -ne 0) { throw 'Frontend dependency installation failed.' }
    }
    npm run build --prefix frontend
    if ($LASTEXITCODE -ne 0) { throw 'Frontend build failed.' }
    go test ./...
    if ($LASTEXITCODE -ne 0) { throw 'Backend tests failed.' }

    # Stage outside the Low-labelled coding tree. A same-volume move preserves labels.
    $outputPath = [IO.Path]::GetFullPath($Output)
    $stagePath = [IO.Path]::GetFullPath($ReleaseStagingDirectory)
    if ([IO.Path]::GetPathRoot($stagePath) -ne [IO.Path]::GetPathRoot($outputPath)) {
        throw 'ReleaseStagingDirectory must be on the same volume as Output.'
    }
    New-Item -ItemType Directory -Force $stagePath | Out-Null
    $stagedExe = Join-Path $stagePath ('SmartNote-' + [guid]::NewGuid().ToString('N') + '.exe')
    go build -tags desktop,production -ldflags '-H windowsgui -s -w' -o $stagedExe .
    if ($LASTEXITCODE -ne 0) { throw 'Desktop build failed.' }
    $labels = icacls $stagedExe
    if ($LASTEXITCODE -ne 0 -or ($labels -join "`n") -match 'Low Mandatory Level|低强制|S-1-16-4096') {
        throw 'Choose a ReleaseStagingDirectory outside the Low integrity workspace.'
    }
    New-Item -ItemType Directory -Force (Split-Path $outputPath -Parent) | Out-Null
    Move-Item -LiteralPath $stagedExe -Destination $outputPath -Force
    Write-Host "Built SmartNote $Version`: $outputPath"

    if ($Installer) {
        $installerOutput = Join-Path (Split-Path $outputPath -Parent) "SmartNote-Setup-v$Version-windows-x64.exe"
        & (Join-Path $PSScriptRoot 'build/Build-Installer.ps1') -AppBinary $outputPath -Output $installerOutput -Version $Version -ReleaseStagingDirectory $ReleaseStagingDirectory
    }
} finally {
    $env:TEMP = $previousTemp
    $env:TMP = $previousTmp
    Pop-Location
}
