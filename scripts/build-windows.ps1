param([string]$OutputDirectory='dist/windows-amd64',[string]$Arch='amd64',[string]$Version='dev')
$ErrorActionPreference='Stop'
New-Item -ItemType Directory -Force $OutputDirectory | Out-Null
rc.exe /nologo /I assets /fo "$OutputDirectory/dfman.res" native/windows/dfman.rc
if ($LASTEXITCODE -ne 0) { throw 'Icon resource build failed' }
cl.exe /nologo /std:c++20 /EHsc /DUNICODE /D_UNICODE native/windows/notify.cpp /Fe:"$OutputDirectory/dfman-notify.exe" /Fo:"$OutputDirectory/notify.obj" "$OutputDirectory/dfman.res" /link /SUBSYSTEM:WINDOWS /ENTRY:wmainCRTStartup runtimeobject.lib shell32.lib ole32.lib oleaut32.lib propsys.lib advapi32.lib
if ($LASTEXITCODE -ne 0) { throw 'Native notification helper build failed' }
$resource = "cmd/dfman/icon_windows_$Arch.syso"
go run github.com/akavel/rsrc@v0.10.2 -ico assets/dfman.ico -arch $Arch -o $resource
if ($LASTEXITCODE -ne 0) { throw 'Go icon resource build failed' }
$env:GOOS='windows'; $env:GOARCH=$Arch; $env:CGO_ENABLED='0'
try {
  go build -trimpath -ldflags "-s -w -X main.version=$Version" -o "$OutputDirectory/dfman.exe" ./cmd/dfman
  if ($LASTEXITCODE -ne 0) { throw 'Go build failed' }
} finally { Remove-Item $resource -ErrorAction SilentlyContinue }
Copy-Item assets/dfman-256.png "$OutputDirectory/dfman.png"
Remove-Item "$OutputDirectory/notify.obj", "$OutputDirectory/dfman.res"
