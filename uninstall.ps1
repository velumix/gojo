$ErrorActionPreference = 'Stop'

$shortcutPath = Join-Path ([Environment]::GetFolderPath('Startup')) 'Gojo.lnk'
if (Test-Path -LiteralPath $shortcutPath) {
	Remove-Item -LiteralPath $shortcutPath
}

Write-Host "Removed Gojo startup shortcut."
