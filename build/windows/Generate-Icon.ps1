$ErrorActionPreference = 'Stop'
Add-Type -AssemblyName System.Drawing
$images = @()
foreach ($size in @(16,32,48,64,128,256)) {
 $bitmap = New-Object Drawing.Bitmap($size,$size)
 $graphics = [Drawing.Graphics]::FromImage($bitmap)
 $graphics.SmoothingMode = [Drawing.Drawing2D.SmoothingMode]::AntiAlias
 $graphics.ScaleTransform($size/256.0,$size/256.0)
 $purple = New-Object Drawing.SolidBrush([Drawing.ColorTranslator]::FromHtml('#7972c9'))
 $white = New-Object Drawing.SolidBrush([Drawing.Color]::White)
 $gold = New-Object Drawing.SolidBrush([Drawing.ColorTranslator]::FromHtml('#f2d695'))
 $path = New-Object Drawing.Drawing2D.GraphicsPath
 $path.AddArc(8,8,108,108,180,90); $path.AddArc(140,8,108,108,270,90); $path.AddArc(140,140,108,108,0,90); $path.AddArc(8,140,108,108,90,90); $path.CloseFigure()
 $graphics.FillPath($purple,$path)
 $graphics.FillRectangle($white,49,60,151,145)
 $spine = New-Object Drawing.Pen([Drawing.ColorTranslator]::FromHtml('#dcd8f3'),8)
 $graphics.DrawLine($spine,75,61,75,205)
 $ink = New-Object Drawing.Pen([Drawing.ColorTranslator]::FromHtml('#7972c9'),10)
 $ink.StartCap = [Drawing.Drawing2D.LineCap]::Round; $ink.EndCap = [Drawing.Drawing2D.LineCap]::Round
 foreach($y in @(99,128,157)) { $end=171; if($y -eq 157){$end=146}; $graphics.DrawLine($ink,101,$y,$end,$y) }
 [Drawing.PointF[]]$star = @([Drawing.PointF]::new(186,40),[Drawing.PointF]::new(193,58),[Drawing.PointF]::new(212,65),[Drawing.PointF]::new(193,72),[Drawing.PointF]::new(186,91),[Drawing.PointF]::new(179,72),[Drawing.PointF]::new(161,65),[Drawing.PointF]::new(179,58))
 $graphics.FillPolygon($gold,$star)
 $stream=New-Object IO.MemoryStream
 $bitmap.Save($stream,[Drawing.Imaging.ImageFormat]::Png)
 $images += ,@{Size=$size;Bytes=$stream.ToArray()}
 if($size -eq 256){$bitmap.Save((Join-Path $PSScriptRoot 'icon.png'),[Drawing.Imaging.ImageFormat]::Png)}
 $stream.Dispose();$ink.Dispose();$spine.Dispose();$path.Dispose();$gold.Dispose();$white.Dispose();$purple.Dispose();$graphics.Dispose();$bitmap.Dispose()
}
$file=[IO.File]::Create((Join-Path $PSScriptRoot 'icon.ico'))
$writer=New-Object IO.BinaryWriter($file)
try {
 $writer.Write([uint16]0);$writer.Write([uint16]1);$writer.Write([uint16]$images.Count)
 $offset=6+16*$images.Count
 foreach($item in $images){$dimension=$item.Size; if($dimension -eq 256){$dimension=0}; $writer.Write([byte]$dimension);$writer.Write([byte]$dimension);$writer.Write([byte]0);$writer.Write([byte]0);$writer.Write([uint16]1);$writer.Write([uint16]32);$writer.Write([uint32]$item.Bytes.Length);$writer.Write([uint32]$offset);$offset+=$item.Bytes.Length}
 foreach($item in $images){$writer.Write([byte[]]$item.Bytes)}
} finally {$writer.Dispose();$file.Dispose()}
