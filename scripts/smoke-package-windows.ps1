param([Parameter(Mandatory=$true)][string]$Version)
$ErrorActionPreference='Stop'
$config=Join-Path $HOME '.config\dfman.conf'
New-Item -ItemType Directory -Force (Split-Path $config) | Out-Null
$content="notification=false`n`n[agent]`ninterval=`"1m`"`nsync_mode=`"reset`"`n"
[IO.File]::WriteAllText($config,$content)
$installer=Get-Item "dist/dfman_${Version}_windows_amd64_setup.exe"
foreach($attempt in 1..2) {
 $p=Start-Process $installer.FullName -ArgumentList '/VERYSILENT','/SUPPRESSMSGBOXES','/NORESTART' -Wait -PassThru
 if($p.ExitCode -ne 0){throw "Installer failed: $($p.ExitCode)"}
 if([IO.File]::ReadAllText($config) -ne $content){throw 'Config changed'}
 $task=Get-ScheduledTask -TaskName dfman-agent
 if($task.Principal.LogonType -ne 'Interactive' -or $task.Principal.RunLevel -ne 'Limited'){throw 'Wrong task principal'}
 if($task.Triggers.Repetition.Interval -ne 'PT1M'){throw 'Wrong task interval'}
}
$exe=Join-Path $HOME '.local\bin\dfman.exe'
& $exe agent run
if($LASTEXITCODE -ne 0){throw 'Empty agent run failed'}
& $exe agent install
if($LASTEXITCODE -eq 0){throw 'Removed command still works'}
$uninstaller=Join-Path $env:LOCALAPPDATA 'dfman\uninstall\unins000.exe'
$p=Start-Process $uninstaller -ArgumentList '/VERYSILENT','/SUPPRESSMSGBOXES','/NORESTART' -Wait -PassThru
if($p.ExitCode -ne 0){throw 'Uninstall failed'}
if(Get-ScheduledTask -TaskName dfman-agent -ErrorAction SilentlyContinue){throw 'Task remains'}
if([IO.File]::ReadAllText($config) -ne $content){throw 'Uninstall removed config'}
