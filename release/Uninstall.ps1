# Belarus Champ Clicker - uninstall
$ErrorActionPreference = "Stop"

$AppDisplayName = "Belarus Champ Clicker"
$AppExeName = "Belarus Champ Clicker.exe"
$InstallDir = Join-Path $env:LOCALAPPDATA "BelarusChampClicker"
$DesktopShortcut = Join-Path $env:USERPROFILE "Desktop\$AppDisplayName.lnk"
$StartShortcut = Join-Path $env:APPDATA "Microsoft\Windows\Start Menu\Programs\$AppDisplayName.lnk"

Write-Host ""
Write-Host "  $AppDisplayName - Uninstall" -ForegroundColor Cyan
Write-Host ""

Stop-Process -Name "Belarus Champ Clicker" -Force -ErrorAction SilentlyContinue
Stop-Process -Name "clicker" -Force -ErrorAction SilentlyContinue
Start-Sleep -Milliseconds 300

$removedApp = $false
if (Test-Path $InstallDir) {
    Remove-Item $InstallDir -Recurse -Force
    Write-Host "  Removed: $InstallDir" -ForegroundColor Green
    $removedApp = $true
}
else {
    Write-Host "  App folder not found (already removed)." -ForegroundColor Gray
}

if (Test-Path $DesktopShortcut) {
    Remove-Item $DesktopShortcut -Force
    Write-Host "  Removed Desktop shortcut." -ForegroundColor Green
}

if (Test-Path $StartShortcut) {
    Remove-Item $StartShortcut -Force
    Write-Host "  Removed Start menu shortcut." -ForegroundColor Green
}

Write-Host ""
if ($removedApp) {
    Write-Host "Clicker uninstalled." -ForegroundColor Green
}
else {
    Write-Host "Nothing to remove for the clicker app." -ForegroundColor Yellow
}

Write-Host ""
Write-Host "The USBip input driver is shared with the system." -ForegroundColor Gray
Write-Host "To remove it: Settings -> Apps -> search USBip -> Uninstall -> restart PC." -ForegroundColor Gray
Write-Host ""
Write-Host "USBip: Параметры -> Приложения -> USBip -> Удалить -> перезагрузка." -ForegroundColor Gray
Write-Host ""
