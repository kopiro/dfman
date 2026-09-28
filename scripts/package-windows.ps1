param([Parameter(Mandatory=$true)][string]$Version,[ValidateSet('amd64','arm64')][string]$Arch='amd64')
$ErrorActionPreference='Stop'
if ($Version -notmatch '^v[0-9]+\.[0-9]+\.[0-9]+$') { throw 'Expected vX.Y.Z' }
$compiler = Get-Command ISCC.exe -ErrorAction SilentlyContinue
if (!$compiler) {
 $candidate = "${env:ProgramFiles(x86)}\Inno Setup 6\ISCC.exe"
 if (!(Test-Path $candidate)) { throw 'Install Inno Setup 6 to build the Windows installer.' }
 $compilerPath = $candidate
} else { $compilerPath = $compiler.Source }
& $compilerPath "/DVersion=$($Version.Substring(1))" "/DArch=$Arch" packaging/windows/dfman.iss
if ($LASTEXITCODE -ne 0) { throw 'Installer build failed' }
