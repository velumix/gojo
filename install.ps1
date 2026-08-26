$ErrorActionPreference = 'Stop'

$projectDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$binaryPath = Join-Path $projectDir 'gojo.exe'

go build -ldflags '-H=windowsgui' -o $binaryPath $projectDir

$startupDir = [Environment]::GetFolderPath('Startup')
$shortcutPath = Join-Path $startupDir 'Gojo.lnk'

$shell = New-Object -ComObject WScript.Shell
$shortcut = $shell.CreateShortcut($shortcutPath)
$shortcut.TargetPath = $binaryPath
$shortcut.WorkingDirectory = $projectDir
$shortcut.WindowStyle = 7
$shortcut.Save()

Start-Process -FilePath $binaryPath -WorkingDirectory $projectDir -WindowStyle Hidden

Write-Host "Installed Gojo startup shortcut: $shortcutPath"
Write-Host "Gojo is running now and will also start when you sign in."
Write-Host "Log file: $env:LOCALAPPDATA\Gojo\gojo.log"
