param([string]$Version='latest',[string]$InstallDirectory=(Join-Path $HOME '.local\bin'))
$ErrorActionPreference='Stop'
if($Version -eq 'latest'){$Version=(Invoke-RestMethod 'https://api.github.com/repos/kopiro/dfman/releases/latest').tag_name}
if($Version -notmatch '^v[0-9][0-9A-Za-z.\-]*$'){throw 'Invalid release version'}
$arch=if([System.Runtime.InteropServices.RuntimeInformation]::OSArchitecture -eq 'Arm64'){'arm64'}else{'amd64'}
$name="dfman_${Version}_windows_${arch}.zip"
$base="https://github.com/kopiro/dfman/releases/download/$Version"
$temp=Join-Path ([IO.Path]::GetTempPath()) ([guid]::NewGuid().ToString())
New-Item -ItemType Directory -Path $temp | Out-Null
try {
 Invoke-WebRequest "$base/$name" -OutFile (Join-Path $temp $name)
 $sums=(Invoke-WebRequest "$base/checksums.txt").Content
 $expected=($sums -split "`n" | Where-Object {($_ -split '\s+')[1] -eq $name}) -split '\s+' | Select-Object -First 1
 $actual=(Get-FileHash (Join-Path $temp $name) -Algorithm SHA256).Hash
 if(!$expected -or $actual -ne $expected){throw 'Checksum verification failed'}
 Expand-Archive (Join-Path $temp $name) (Join-Path $temp 'package')
 New-Item -ItemType Directory -Force -Path $InstallDirectory | Out-Null
 foreach($file in @('dfman.exe','dfman-notify.exe')){Copy-Item (Join-Path $temp "package\$file") (Join-Path $InstallDirectory $file) -Force}
 $current=[Environment]::GetEnvironmentVariable('Path','User')
 if(($current -split ';') -notcontains $InstallDirectory){[Environment]::SetEnvironmentVariable('Path',"$InstallDirectory;$current",'User')}
 Write-Host "Installed $Version. Open a new terminal. Agent installation is separate: dfman agent install"
} finally {Remove-Item -LiteralPath $temp -Recurse -Force}
