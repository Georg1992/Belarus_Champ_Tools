param([string]$ExePath)
$p = Start-Process -FilePath $ExePath -PassThru
Start-Sleep -Seconds 3
$w = Get-Process -Id $p.Id -ErrorAction SilentlyContinue
if ($w) {
    Write-Host "HW: $($w.MainWindowHandle -ne 0)"
    Write-Host "Title: [$($w.MainWindowTitle)]"
} else {
    Write-Host "EXITED"
}
Stop-Process -Id $p.Id -Force -ErrorAction SilentlyContinue
