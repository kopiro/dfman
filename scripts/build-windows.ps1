param([string]$OutputDirectory='dist/windows-amd64',[string]$Arch='amd64',[string]$Version='dev')
$ErrorActionPreference='Stop'
New-Item -ItemType Directory -Force $OutputDirectory | Out-Null
cl.exe /nologo /std:c++20 /EHsc /DUNICODE /D_UNICODE native/windows/notify.cpp /Fe:"$OutputDirectory/dfman-notify.exe" /Fo:"$OutputDirectory/notify.obj" /link /SUBSYSTEM:WINDOWS /ENTRY:wmainCRTStartup runtimeobject.lib shell32.lib ole32.lib oleaut32.lib propsys.lib advapi32.lib
if ($LASTEXITCODE -ne 0) { throw 'Native notification helper build failed' }
$env:GOOS='windows'; $env:GOARCH=$Arch; $env:CGO_ENABLED='0'
go build -trimpath -ldflags "-s -w -X main.version=$Version" -o "$OutputDirectory/dfman.exe" ./cmd/dfman
if ($LASTEXITCODE -ne 0) { throw 'Go build failed' }
Remove-Item "$OutputDirectory/notify.obj"
