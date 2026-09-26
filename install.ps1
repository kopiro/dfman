param(
    [string]$InstallDirectory = "$env:USERPROFILE\.local\bin",
    [string]$Ref = 'main',
    [string]$SourceDirectory
)

$ErrorActionPreference = 'Stop'
# Use Git for Windows, not WSL's unrelated bash.exe.
$git = (Get-Command git.exe -ErrorAction Stop).Source
$gitRoot = Split-Path (Split-Path $git)
$bash = Join-Path $gitRoot 'bin\bash.exe'
if (!(Test-Path $bash)) {
    throw 'Install Git for Windows and put git.exe on PATH first.'
}

New-Item -ItemType Directory -Force $InstallDirectory | Out-Null
$script = Join-Path $InstallDirectory 'dfman'
$temporary = Join-Path $InstallDirectory ('.dfman-' + [guid]::NewGuid())
try {
    if ($SourceDirectory) {
        Copy-Item (Join-Path $SourceDirectory 'dfman') $temporary
    } else {
        Invoke-WebRequest "https://raw.githubusercontent.com/kopiro/dfman/$Ref/dfman" -OutFile $temporary
    }
    $content = [IO.File]::ReadAllText($temporary).Replace("`r`n", "`n")
    if (!$content.StartsWith("#!/bin/bash`n")) { throw 'Invalid dfman download.' }
    [IO.File]::WriteAllText($temporary, $content, [Text.UTF8Encoding]::new($false))
    Move-Item -Force $temporary $script
} finally {
    if (Test-Path $temporary) { Remove-Item $temporary }
}

# Start a clean Bash so shell profiles cannot change unattended behavior.
# Supply Git's utilities on PATH for SSH sessions and Task Scheduler too.
$launcher = @"
@echo off
setlocal
set "PATH=$gitRoot\bin;$gitRoot\usr\bin;%PATH%"
"$bash" --noprofile --norc "%~dp0dfman" %*
exit /b %ERRORLEVEL%
"@
[IO.File]::WriteAllText((Join-Path $InstallDirectory 'dfman.cmd'), $launcher.Replace("`n", "`r`n"), [Text.UTF8Encoding]::new($false))
$userPath = [Environment]::GetEnvironmentVariable('Path', 'User')
if ($InstallDirectory -notin ($userPath -split ';')) {
    [Environment]::SetEnvironmentVariable('Path', "$InstallDirectory;$userPath", 'User')
}
$env:Path = "$InstallDirectory;$env:Path"
Write-Host "Installed dfman in $InstallDirectory. Open a new terminal to use dfman."
Write-Host 'Linking requires Windows Developer Mode or an elevated terminal.'
