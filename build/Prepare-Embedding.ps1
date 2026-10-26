param()
$ErrorActionPreference = 'Stop'
$projectRoot = Split-Path $PSScriptRoot -Parent
$resourceRoot = Join-Path $projectRoot 'internal\embedding\assets'
$manifest = Get-Content -LiteralPath (Join-Path $resourceRoot 'manifest.json') -Raw -Encoding UTF8 | ConvertFrom-Json
$cacheRoot = Join-Path $projectRoot '.tmp\embedding-downloads'
Add-Type -AssemblyName System.IO.Compression.FileSystem
foreach ($property in $manifest.files.PSObject.Properties) {
    $name = $property.Name
    $entry = $property.Value
    if ([IO.Path]::GetFileName($name) -ne $name) { throw "Invalid resource name: $name" }
    $target = Join-Path $resourceRoot $name
    if ((Test-Path -LiteralPath $target) -and (Get-FileHash -LiteralPath $target -Algorithm SHA256).Hash -eq $entry.sha256) { continue }
    New-Item -ItemType Directory -Force $cacheRoot,$resourceRoot | Out-Null
    $download = Join-Path $cacheRoot ($entry.sha256 + '.download')
    if ($entry.source.entry) { $download = Join-Path $cacheRoot ($entry.source.archiveSha256 + '.archive') }
    if (-not (Test-Path -LiteralPath $download)) {
        Write-Host "Preparing embedded resource: $name"
        Invoke-WebRequest -UseBasicParsing -Uri $entry.source.url -OutFile ($download + '.part')
        Move-Item -LiteralPath ($download + '.part') -Destination $download -Force
    }
    $staged = Join-Path $cacheRoot ($name + '.verified')
    if ($entry.source.entry) {
        if ((Get-FileHash -LiteralPath $download -Algorithm SHA256).Hash -ne $entry.source.archiveSha256) { throw "Archive checksum mismatch: $name" }
        $zip = [IO.Compression.ZipFile]::OpenRead($download)
        try {
            $zipEntry = $zip.GetEntry($entry.source.entry)
            if ($null -eq $zipEntry) { throw "Archive resource missing: $name" }
            [IO.Compression.ZipFileExtensions]::ExtractToFile($zipEntry, $staged, $true)
        } finally { $zip.Dispose() }
    } else { Copy-Item -LiteralPath $download -Destination $staged -Force }
    if ((Get-FileHash -LiteralPath $staged -Algorithm SHA256).Hash -ne $entry.sha256 -or (Get-Item -LiteralPath $staged).Length -ne $entry.size) { throw "Resource checksum mismatch: $name. Do not update the manifest without verifying the upstream change." }
    Move-Item -LiteralPath $staged -Destination $target -Force
}
Write-Host 'Embedded model and runtime resource checks passed.'
