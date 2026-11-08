#define MyAppName "Gojo"
#ifndef MyAppId
  #define MyAppId "{{B490F3B8-BC4A-4D5A-9ACD-898759A00F9E}"
#endif
#ifndef MyStartMenuGroup
  #define MyStartMenuGroup "Gojo"
#endif
#ifndef MyStartupName
  #define MyStartupName "Gojo"
#endif
#ifndef MyAppVersion
  #define MyAppVersion "0.1.0"
#endif
#define MyAppPublisher "velumix"
#define MyAppExeName "gojo.exe"

[Setup]
AppId={#MyAppId}
AppName={#MyAppName}
AppVersion={#MyAppVersion}
AppPublisher={#MyAppPublisher}
AppPublisherURL=https://github.com/velumix/gojo
AppSupportURL=https://github.com/velumix/gojo/issues
AppComments=Roblox Studio MCP proxy and optional background watcher
AppReadmeFile={app}\Getting Started.txt
DefaultDirName={localappdata}\Programs\Gojo
DefaultGroupName={#MyStartMenuGroup}
DisableProgramGroupPage=yes
DisableWelcomePage=no
PrivilegesRequired=lowest
OutputDir=dist
OutputBaseFilename=Gojo-Setup-{#MyAppVersion}-win64
Compression=lzma2
SolidCompression=yes
WizardStyle=modern
ArchitecturesAllowed=x64compatible
ArchitecturesInstallIn64BitMode=x64compatible
CloseApplications=yes
RestartApplications=no
SetupLogging=yes
UninstallDisplayIcon={app}\{#MyAppExeName}
LicenseFile=LICENSE
SetupIconFile=assets\gojo.ico
WizardImageFile=assets\wizard-large.bmp
WizardSmallImageFile=assets\wizard-small.bmp

[Files]
Source: "build\gojo.exe"; DestDir: "{app}"; Flags: ignoreversion
Source: "README.md"; DestDir: "{app}"; Flags: ignoreversion
Source: "LICENSE"; DestDir: "{app}"; Flags: ignoreversion
Source: "docs\Getting Started.txt"; DestDir: "{app}"; Flags: ignoreversion
Source: "tools\stop-installed-gojo.ps1"; DestDir: "{app}"; Flags: ignoreversion

[Tasks]
Name: "autostart"; Description: "Start the Gojo watcher when I sign in"; Flags: checkedonce

[Icons]
Name: "{userstartup}\{#MyStartupName}"; Filename: "{app}\{#MyAppExeName}"; Parameters: "--watch"; WorkingDir: "{app}"; Tasks: autostart
Name: "{group}\Getting Started"; Filename: "{app}\Getting Started.txt"
Name: "{group}\Gojo installation folder"; Filename: "{app}"
Name: "{group}\Uninstall Gojo"; Filename: "{uninstallexe}"

[Run]
Filename: "{app}\{#MyAppExeName}"; Parameters: "--watch"; Description: "Start the Gojo watcher now"; Flags: nowait postinstall skipifsilent; Tasks: autostart
Filename: "{app}\Getting Started.txt"; Description: "Open the getting started guide"; Flags: postinstall shellexec skipifsilent

[UninstallRun]
Filename: "{sys}\WindowsPowerShell\v1.0\powershell.exe"; Parameters: "-NoProfile -ExecutionPolicy Bypass -WindowStyle Hidden -File ""{app}\stop-installed-gojo.ps1"" -InstallDir ""{app}"""; Flags: runhidden waituntilterminated; RunOnceId: stop-installed-gojo

[Code]
var
  McpPage: TWizardPage;
  McpPath: TNewEdit;

procedure CopyMcpPath(Sender: TObject);
begin
  McpPath.SelectAll;
  SendMessage(McpPath.Handle, $0301, 0, 0);
end;

procedure InitializeWizard;
var
  Intro: TNewStaticText;
  Detail: TNewStaticText;
  CopyButton: TNewButton;
begin
  McpPage := CreateCustomPage(wpSelectTasks,
    'Connect your MCP client',
    'Use this executable as the command for a Gojo MCP server.');

  Intro := TNewStaticText.Create(McpPage);
  Intro.Parent := McpPage.Surface;
  Intro.Left := 0;
  Intro.Top := ScaleY(8);
  Intro.Width := McpPage.SurfaceWidth;
  Intro.Height := ScaleY(52);
  Intro.WordWrap := True;
  Intro.Caption := 'After installation, create a server named Gojo in your MCP client and paste this path into its command field:';

  McpPath := TNewEdit.Create(McpPage);
  McpPath.Parent := McpPage.Surface;
  McpPath.Left := 0;
  McpPath.Top := ScaleY(68);
  McpPath.Width := McpPage.SurfaceWidth - ScaleX(98);
  McpPath.ReadOnly := True;

  CopyButton := TNewButton.Create(McpPage);
  CopyButton.Parent := McpPage.Surface;
  CopyButton.Caption := 'Copy path';
  CopyButton.Left := McpPath.Left + McpPath.Width + ScaleX(8);
  CopyButton.Top := McpPath.Top - ScaleY(1);
  CopyButton.Width := ScaleX(90);
  CopyButton.OnClick := @CopyMcpPath;

  Detail := TNewStaticText.Create(McpPage);
  Detail.Parent := McpPage.Surface;
  Detail.Left := 0;
  Detail.Top := ScaleY(114);
  Detail.Width := McpPage.SurfaceWidth;
  Detail.Height := ScaleY(75);
  Detail.WordWrap := True;
  Detail.Caption := 'Gojo runs quietly in the background. The optional watcher connects when Roblox Studio opens. The Getting Started guide will also be available from the Start menu.';
end;

procedure CurPageChanged(CurPageID: Integer);
begin
  if CurPageID = McpPage.ID then
    McpPath.Text := AddBackslash(WizardDirValue) + '{#MyAppExeName}';
end;

procedure CurStepChanged(CurStep: TSetupStep);
begin
  if (CurStep = ssPostInstall) and (not WizardIsTaskSelected('autostart')) then
    DeleteFile(ExpandConstant('{userstartup}\{#MyStartupName}.lnk'));
end;
