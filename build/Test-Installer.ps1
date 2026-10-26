param(
 [Parameter(Mandatory=$true)][string]$Installer,
 [Parameter(Mandatory=$true)][string]$Binary,
 [string]$StageRoot=(Join-Path ([IO.Path]::GetTempPath()) 'SmartNote-installer-tests')
)
$ErrorActionPreference='Stop'
$testRoot=[IO.Path]::GetFullPath((Join-Path $stageRoot ('installer-'+[guid]::NewGuid().ToString('N').Substring(0,8))))
if(-not $testRoot.StartsWith([IO.Path]::GetFullPath($stageRoot)+[IO.Path]::DirectorySeparatorChar,[StringComparison]::OrdinalIgnoreCase)){throw 'Invalid isolated test root'}
$installDir=Join-Path $testRoot 'program'
$diagnosticDir=Join-Path $testRoot 'diagnostic'
$profileDir=Join-Path $testRoot 'profile'
New-Item -ItemType Directory -Force $testRoot | Out-Null
$beforeRegistry=Get-ItemPropertyValue 'HKCU:\Software\SmartNote' 'InstallDir' -ErrorAction SilentlyContinue
$shortcut=Join-Path ([Environment]::GetFolderPath('Desktop')) 'SmartNote.lnk'
$beforeShortcut=if(Test-Path -LiteralPath $shortcut){(Get-FileHash -LiteralPath $shortcut).Hash}else{''}
function Start-TestProcess([string]$file,[string]$arguments,[bool]$cleanPath=$false){
 $start=New-Object Diagnostics.ProcessStartInfo
 $start.FileName=$file;$start.Arguments=$arguments;$start.UseShellExecute=$false;$start.CreateNoWindow=$true
 $start.EnvironmentVariables['SMARTNOTE_DATA_DIR']=$profileDir
 if($cleanPath){$start.EnvironmentVariables['PATH']="$env:WINDIR\System32;$env:WINDIR"}
 $p=[Diagnostics.Process]::Start($start)
 if(-not $p.WaitForExit(60000)){throw "Test process timed out: $($p.Id)"}
 if($p.ExitCode -ne 0){throw "Test process failed ($($p.ExitCode)): $file"}
}
Start-TestProcess ([IO.Path]::GetFullPath($Installer)) "/S /ISOLATED /D=$installDir"
$installed=Join-Path $installDir 'SmartNote.exe'
if((Get-FileHash -LiteralPath $installed).Hash -ne (Get-FileHash -LiteralPath $Binary).Hash){throw 'Installed program differs from release binary'}
if(-not(Test-Path -LiteralPath (Join-Path $installDir 'licenses\MODEL-LICENSE'))){throw 'Installed licenses missing'}
Start-TestProcess $installed "--self-test-embedding $diagnosticDir" $true
$result=Get-Content -LiteralPath (Join-Path $diagnosticDir 'embedding-self-test.json') -Raw | ConvertFrom-Json
if(-not $result.ok -or $result.dimensions -ne 512){throw 'Installed offline model check failed'}
New-Item -ItemType Directory -Force $profileDir | Out-Null
Set-Content -LiteralPath (Join-Path $profileDir 'notes-preserved.md') -Value 'isolated user data sentinel'
Set-Content -LiteralPath (Join-Path $installDir 'keep-user-file.md') -Value 'unrelated file must survive uninstall'
# _?= runs the uninstaller directly so the process wait covers all deletion.
Start-TestProcess (Join-Path $installDir 'Uninstall.exe') "/S _?=$installDir"
if(Test-Path -LiteralPath $installed){throw 'Uninstaller did not remove program'}
if(-not(Test-Path -LiteralPath (Join-Path $installDir 'keep-user-file.md')) -or -not(Test-Path -LiteralPath (Join-Path $profileDir 'notes-preserved.md'))){throw 'Uninstaller removed user files'}
$afterRegistry=Get-ItemPropertyValue 'HKCU:\Software\SmartNote' 'InstallDir' -ErrorAction SilentlyContinue
$afterShortcut=if(Test-Path -LiteralPath $shortcut){(Get-FileHash -LiteralPath $shortcut).Hash}else{''}
if($beforeRegistry -ne $afterRegistry -or $beforeShortcut -ne $afterShortcut){throw 'Isolated installation touched the user environment'}
Write-Output "PASS: install, packaged CPU inference with clean PATH, uninstall, user-data preservation and unchanged user registry/shortcut. Evidence: $testRoot"
$result | ConvertTo-Json -Compress
