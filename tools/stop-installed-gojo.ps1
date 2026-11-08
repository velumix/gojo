param(
    [Parameter(Mandatory = $true)]
    [string]$InstallDir
)

$target = [System.IO.Path]::GetFullPath((Join-Path $InstallDir 'gojo.exe'))
Get-CimInstance Win32_Process -Filter "Name = 'gojo.exe'" -ErrorAction SilentlyContinue |
    Where-Object {
        $_.ExecutablePath -and
        [string]::Equals(
            [System.IO.Path]::GetFullPath($_.ExecutablePath),
            $target,
            [System.StringComparison]::OrdinalIgnoreCase
        )
    } |
    ForEach-Object { Stop-Process -Id $_.ProcessId -Force -ErrorAction SilentlyContinue }
