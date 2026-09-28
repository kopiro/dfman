#ifndef Version
#error Version is required
#endif
#ifndef Arch
#error Arch is required
#endif
#define ProjectRoot AddBackslash(SourcePath) + "..\.."

[Setup]
AppId=com.kopiro.dfman
AppName=dfman
AppVersion={#Version}
AppPublisher=kopiro
AppPublisherURL=https://github.com/kopiro/dfman
DefaultDirName={%USERPROFILE}\.local\bin
DisableDirPage=yes
PrivilegesRequired=lowest
UninstallFilesDir={localappdata}\dfman\uninstall
OutputDir={#ProjectRoot}\dist
OutputBaseFilename=dfman_v{#Version}_windows_{#Arch}_setup
Compression=lzma2
SolidCompression=yes
WizardStyle=modern
ChangesEnvironment=yes
CloseApplications=yes
RestartApplications=no
#if Arch == "arm64"
ArchitecturesAllowed=arm64
ArchitecturesInstallIn64BitMode=arm64
#else
ArchitecturesAllowed=x64compatible
ArchitecturesInstallIn64BitMode=x64compatible
#endif

[Files]
Source: "{#ProjectRoot}\dist\windows-{#Arch}\dfman.exe"; DestDir: "{app}"; Flags: ignoreversion
Source: "{#ProjectRoot}\dist\windows-{#Arch}\dfman-notify.exe"; DestDir: "{app}"; Flags: ignoreversion

[UninstallRun]
Filename: "{app}\dfman.exe"; Parameters: "_package remove"; Flags: runhidden waituntilterminated; RunOnceId: "RemoveAgent"

[Code]
function PrepareToInstall(var NeedsRestart: Boolean): String;
var Code: Integer;
begin
  Result := '';
  if FileExists(ExpandConstant('{app}\dfman.exe')) then
    if not Exec(ExpandConstant('{app}\dfman.exe'), '_package remove', '', SW_HIDE, ewWaitUntilTerminated, Code) or (Code <> 0) then
      Result := 'Could not stop the existing dfman agent. Run dfman status before retrying.';
end;

procedure CurStepChanged(CurStep: TSetupStep);
var Code: Integer; CurrentPath, BinDir: String;
begin
  if CurStep = ssPostInstall then begin
    BinDir := ExpandConstant('{app}');
    RegQueryStringValue(HKCU, 'Environment', 'Path', CurrentPath);
    if Pos(';' + Lowercase(BinDir) + ';', ';' + Lowercase(CurrentPath) + ';') = 0 then
      RegWriteExpandStringValue(HKCU, 'Environment', 'Path', BinDir + ';' + CurrentPath);
    if not Exec(BinDir + '\dfman.exe', '_package setup', '', SW_HIDE, ewWaitUntilTerminated, Code) or (Code <> 0) then
      RaiseException('dfman was installed but agent setup failed. Run dfman status, correct the reported problem, then run this installer again.');
  end;
end;
