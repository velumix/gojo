param(
    [ValidateNotNullOrEmpty()]
    [string]$Version = '0.1.0'
)

$ErrorActionPreference = 'Stop'
$projectDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$buildDir = Join-Path $projectDir 'build'
$installerScript = Join-Path $projectDir 'installer.iss'
$stageDir = Join-Path $env:TEMP ('gojo-installer-' + [guid]::NewGuid().ToString('N'))
$tempRoot = [System.IO.Path]::GetFullPath($env:TEMP).TrimEnd('\') + '\'
$stagePath = [System.IO.Path]::GetFullPath($stageDir)
if (-not $stagePath.StartsWith($tempRoot, [System.StringComparison]::OrdinalIgnoreCase)) {
    throw 'The installer staging directory must be inside the temporary directory.'
}

$go = Get-Command go -ErrorAction SilentlyContinue | Select-Object -ExpandProperty Source -First 1
if (-not $go) {
    throw 'Go was not found. Install Go 1.22 or newer and add it to PATH.'
}

$iscc = Get-Command ISCC.exe -ErrorAction SilentlyContinue | Select-Object -ExpandProperty Source -First 1
if (-not $iscc) {
    foreach ($candidate in @(
        (Join-Path ${env:ProgramFiles(x86)} 'Inno Setup 6\ISCC.exe'),
        (Join-Path $env:ProgramFiles 'Inno Setup 6\ISCC.exe'),
        (Join-Path $env:LOCALAPPDATA 'Programs\Inno Setup 6\ISCC.exe')
    )) {
        if (Test-Path -LiteralPath $candidate) {
            $iscc = $candidate
            break
        }
    }
}
if (-not $iscc) {
    throw 'Inno Setup 6 was not found. Install it or add ISCC.exe to PATH.'
}

New-Item -ItemType Directory -Path $buildDir -Force | Out-Null
New-Item -ItemType Directory -Path $stageDir -Force | Out-Null
$binaryPath = Join-Path $buildDir 'gojo.exe'
$installerName = "Gojo-Setup-$Version-win64.exe"
$installerPath = Join-Path $projectDir "dist\$installerName"
Push-Location $projectDir
try {
    $env:GOOS = 'windows'
    $env:GOARCH = 'amd64'
    & $go test ./...
    if ($LASTEXITCODE -ne 0) { throw 'Go tests failed.' }
    & $go build -buildvcs=false -trimpath -ldflags '-H=windowsgui' -o $binaryPath .
    if ($LASTEXITCODE -ne 0) { throw 'Go build failed.' }
    & $iscc "/DMyAppVersion=$Version" "/O$stageDir" $installerScript
    if ($LASTEXITCODE -ne 0) { throw 'Inno Setup build failed.' }
    $stagedInstaller = Join-Path $stageDir $installerName
    if (-not (Test-Path -LiteralPath $stagedInstaller)) {
        throw "Installer was not created: $stagedInstaller"
    }
    New-Item -ItemType Directory -Path (Split-Path -Parent $installerPath) -Force | Out-Null
    Copy-Item -LiteralPath $stagedInstaller -Destination $installerPath -Force
}
finally {
    Pop-Location
    Remove-Item -LiteralPath $stagePath -Recurse -Force
}

if (-not (Test-Path -LiteralPath $installerPath)) {
    throw "Installer was not created: $installerPath"
}
Write-Host "Installer: $installerPath"
