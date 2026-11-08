$ErrorActionPreference = 'Stop'
Add-Type -AssemblyName System.Drawing

$projectDir = Split-Path -Parent $PSScriptRoot
$assetsDir = Join-Path $projectDir 'assets'
$source = [System.Drawing.Image]::FromFile((Join-Path $assetsDir 'gojo.png'))
try {
    foreach ($variant in @(
        @{ Name = 'wizard-large.bmp'; Width = 164; Height = 314; LogoSize = 138; LogoTop = 53 },
        @{ Name = 'wizard-small.bmp'; Width = 55; Height = 55; LogoSize = 48; LogoTop = 3 }
    )) {
        $bitmap = [System.Drawing.Bitmap]::new($variant.Width, $variant.Height)
        try {
            $graphics = [System.Drawing.Graphics]::FromImage($bitmap)
            try {
                $background = [System.Drawing.Color]::FromArgb(14, 22, 43)
                $graphics.Clear($background)
                $graphics.InterpolationMode = [System.Drawing.Drawing2D.InterpolationMode]::HighQualityBicubic
                $graphics.PixelOffsetMode = [System.Drawing.Drawing2D.PixelOffsetMode]::HighQuality
                $left = [int](($variant.Width - $variant.LogoSize) / 2)
                $graphics.DrawImage($source, $left, $variant.LogoTop, $variant.LogoSize, $variant.LogoSize)
                if ($variant.Name -eq 'wizard-large.bmp') {
                    $font = [System.Drawing.Font]::new('Segoe UI', 25, [System.Drawing.FontStyle]::Bold)
                    $smallFont = [System.Drawing.Font]::new('Segoe UI', 8, [System.Drawing.FontStyle]::Regular)
                    $white = [System.Drawing.SolidBrush]::new([System.Drawing.Color]::White)
                    $muted = [System.Drawing.SolidBrush]::new([System.Drawing.Color]::FromArgb(176, 193, 217))
                    $center = [System.Drawing.StringFormat]::new()
                    try {
                        $center.Alignment = [System.Drawing.StringAlignment]::Center
                        $graphics.DrawString('Gojo', $font, $white, [System.Drawing.RectangleF]::new(0, 207, 164, 48), $center)
                        $graphics.DrawString('ROBLOX MCP', $smallFont, $muted, [System.Drawing.RectangleF]::new(0, 253, 164, 20), $center)
                    }
                    finally {
                        $center.Dispose()
                        $muted.Dispose()
                        $white.Dispose()
                        $smallFont.Dispose()
                        $font.Dispose()
                    }
                }
            }
            finally {
                $graphics.Dispose()
            }
            $bitmap.Save((Join-Path $assetsDir $variant.Name), [System.Drawing.Imaging.ImageFormat]::Bmp)
        }
        finally {
            $bitmap.Dispose()
        }
    }
}
finally {
    $source.Dispose()
}
