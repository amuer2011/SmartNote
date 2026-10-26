param([switch]$SkipInstall, [switch]$Installer, [string]$Output = 'build/bin/SmartNote.exe', [string]$ReleaseStagingDirectory = (Join-Path (Split-Path (Split-Path $PSScriptRoot -Parent) -Parent) '.SmartNote-build'))




$ErrorActionPreference = 'Stop'




Push-Location $PSScriptRoot




$previousTemp = $env:TEMP




$previousTmp = $env:TMP




try {




    $buildTemp = Join-Path $PSScriptRoot '.tmp\build'




    New-Item -ItemType Directory -Force $buildTemp | Out-Null




    $env:TEMP = $buildTemp




    $env:TMP = $buildTemp




    & (Join-Path $PSScriptRoot 'build/Prepare-Embedding.ps1')




    & (Join-Path $PSScriptRoot 'build/windows/Generate-Icon.ps1')




    go run github.com/tc-hib/go-winres@v0.3.3 make --in build/windows/winres.json --arch amd64 --out rsrc




    if ($LASTEXITCODE -ne 0) { throw 'Windows icon/manifest generation failed.' }




    if (-not $SkipInstall) {




        npm ci --prefix frontend




        if ($LASTEXITCODE -ne 0) { throw '安装前端依赖失败' }




    }




    npm run build --prefix frontend




    if ($LASTEXITCODE -ne 0) { throw '前端构建失败' }




    go test ./...




    if ($LASTEXITCODE -ne 0) { throw '后端测试失败' }




    # Stage outside the Low-labelled coding tree. A same-volume move preserves labels.




    $outputPath = [System.IO.Path]::GetFullPath($Output)




    $stagePath = [System.IO.Path]::GetFullPath($ReleaseStagingDirectory)




    if ([System.IO.Path]::GetPathRoot($stagePath) -ne [System.IO.Path]::GetPathRoot($outputPath)) {




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




    Write-Host "构建完成: $([System.IO.Path]::GetFullPath($Output))"




    if ($Installer) {




        $installerOutput = Join-Path (Split-Path $outputPath -Parent) 'SmartNote-Setup-v0.12.18-paste-editor.exe'




        & (Join-Path $PSScriptRoot 'build/Build-Installer.ps1') -AppBinary $outputPath -Output $installerOutput -ReleaseStagingDirectory $ReleaseStagingDirectory




    }




} finally {




    $env:TEMP = $previousTemp




    $env:TMP = $previousTmp




    Pop-Location




}




