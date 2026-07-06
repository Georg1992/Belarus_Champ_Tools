$p = Start-Process -FilePath "C:\Users\georg\Desktop\Belarus_Champ_Tools\app_prephase3.exe" -PassThru
Start-Sleep -Seconds 3
$w = Get-Process -Id $p.Id -ErrorAction SilentlyContinue
if ($w) {
    Write-Host "HasWindow: $($w.MainWindowHandle -ne 0)"
    Write-Host "Title: [$($w.MainWindowTitle)]"
    Write-Host "Responding: $($w.Responding)"
} else {
    Write-Host "Process already exited"
}
Stop-Process -Id $p.Id -Force -ErrorAction SilentlyContinue
Write-Host "DONE"
