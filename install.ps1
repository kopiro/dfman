param([string]$Version='latest')
$ErrorActionPreference='Stop'
if($Version -eq 'latest'){$Version=(Invoke-RestMethod 'https://api.github.com/repos/kopiro/dfman/releases/latest').tag_name}
if($Version -notmatch '^v[0-9][0-9A-Za-z.\-]*$'){throw 'Invalid release version'}
$arch=if([System.Runtime.InteropServices.RuntimeInformation]::OSArchitecture -eq 'Arm64'){'arm64'}else{'amd64'}
$name="dfman_${Version}_windows_${arch}_setup.exe"
$base="https://github.com/kopiro/dfman/releases/download/$Version"
$temp=Join-Path ([IO.Path]::GetTempPath()) ([guid]::NewGuid().ToString())
New-Item -ItemType Directory -Path $temp | Out-Null
try {
 Invoke-WebRequest -UseBasicParsing "$base/$name" -OutFile (Join-Path $temp $name)
 $sums=(Invoke-WebRequest -UseBasicParsing "$base/checksums.txt").Content
 $expected=($sums -split "`n" | Where-Object {($_ -split '\s+')[1] -eq $name}) -split '\s+' | Select-Object -First 1
 $actual=(Get-FileHash (Join-Path $temp $name) -Algorithm SHA256).Hash
 if(!$expected -or $actual -ne $expected){throw 'Checksum verification failed'}
 $process=Start-Process -FilePath (Join-Path $temp $name) -Wait -PassThru
 if($process.ExitCode -ne 0){throw "Installer exited with code $($process.ExitCode)"}
 Write-Host "Installed $Version. Open a new terminal. The agent is configured automatically."
} finally {Remove-Item -LiteralPath $temp -Recurse -Force}
