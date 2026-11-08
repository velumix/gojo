$ErrorActionPreference = 'Stop'

$projectDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$binaryPath = Join-Path $projectDir 'gojo.exe'

go build -buildvcs=false -ldflags '-H=windowsgui' -o $binaryPath $projectDir
if ($LASTEXITCODE -ne 0 -or -not (Test-Path -LiteralPath $binaryPath)) {
    throw 'Gojo build failed or the executable was removed. Automatic startup was not installed.'
}

$watcher = Start-Process -FilePath $binaryPath -ArgumentList '--watch' -WorkingDirectory $projectDir -WindowStyle Hidden -PassThru
Start-Sleep -Seconds 2
$watcher.Refresh()
if ($watcher.HasExited) {
    throw 'Gojo stopped immediately. Automatic startup was not installed.'
}

$startupDir = [Environment]::GetFolderPath('Startup')
$shortcutPath = Join-Path $startupDir 'Gojo.lnk'

$shell = New-Object -ComObject WScript.Shell
$shortcut = $shell.CreateShortcut($shortcutPath)
$shortcut.TargetPath = $binaryPath
$shortcut.WorkingDirectory = $projectDir
$shortcut.WindowStyle = 7
$shortcut.Save()

Write-Host "Installed Gojo startup shortcut: $shortcutPath"
Write-Host "Gojo is running now and will also start when you sign in."
Write-Host "Log file: $env:LOCALAPPDATA\Gojo\gojo.log"
