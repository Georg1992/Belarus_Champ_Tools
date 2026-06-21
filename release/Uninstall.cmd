@echo off
setlocal
title Belarus Champ Clicker Uninstall
cd /d "%~dp0"

set "BCC_INSTALL_DIR=%~dp0"
set "BCC_CMD_PATH=%~f0"
set "BCC_TMPPS1=%TEMP%\belarus-champ-clicker-uninstall-%RANDOM%.ps1"
set "BCC_SKIP=0"
for /f "tokens=1 delims=:" %%A in ('findstr /n /b ":PS1" "%~f0"') do set "BCC_SKIP=%%A"

powershell.exe -NoProfile -ExecutionPolicy Bypass -Command ^
  "$skip = [int]$env:BCC_SKIP; $lines = [IO.File]::ReadAllLines($env:BCC_CMD_PATH); $body = ($lines | Select-Object -Skip $skip) -join [Environment]::NewLine; [IO.File]::WriteAllText($env:BCC_TMPPS1, $body, [Text.UTF8Encoding]::new($false))"
if errorlevel 1 goto fail

powershell.exe -NoProfile -ExecutionPolicy Bypass -File "%BCC_TMPPS1%"
set ERR=%ERRORLEVEL%
del "%BCC_TMPPS1%" 2>nul
if %ERR% neq 0 goto fail
goto done

:fail
echo.
echo Uninstall failed. See README.txt
set ERR=1

:done
echo.
pause
exit /b %ERR%

:PS1
$ErrorActionPreference = "Stop"

$AppDisplayName = "Belarus Champ Clicker"

Write-Host ""
Write-Host "  $AppDisplayName - Uninstall" -ForegroundColor Cyan
Write-Host ""

Write-Host "Stopping clicker..." -ForegroundColor Cyan
Stop-Process -Name "Belarus Champ Clicker" -Force -ErrorAction SilentlyContinue
Stop-Process -Name "clicker" -Force -ErrorAction SilentlyContinue
Start-Sleep -Milliseconds 300
Write-Host "  Done." -ForegroundColor Green

Write-Host ""
Write-Host "Removing old shortcuts and app data (if any)..." -ForegroundColor Cyan

$legacyDir = Join-Path $env:LOCALAPPDATA "BelarusChampClicker"
if (Test-Path $legacyDir) {
    Remove-Item $legacyDir -Recurse -Force
    Write-Host "  Removed $legacyDir" -ForegroundColor Green
}

$desktopShortcut = Join-Path $env:USERPROFILE "Desktop\$AppDisplayName.lnk"
if (Test-Path $desktopShortcut) {
    Remove-Item $desktopShortcut -Force
    Write-Host "  Removed Desktop shortcut." -ForegroundColor Green
}

$startShortcut = Join-Path $env:APPDATA "Microsoft\Windows\Start Menu\Programs\$AppDisplayName.lnk"
if (Test-Path $startShortcut) {
    Remove-Item $startShortcut -Force
    Write-Host "  Removed Start menu shortcut." -ForegroundColor Green
}

Write-Host ""
Write-Host "Removing input driver..." -ForegroundColor Cyan

$UsbipEntry = Get-ItemProperty "HKLM:\Software\Microsoft\Windows\CurrentVersion\Uninstall\*" -ErrorAction SilentlyContinue |
    Where-Object { $_.DisplayName -like "USBip version*" } |
    Select-Object -First 1
if (-not $UsbipEntry) {
    $UsbipEntry = Get-ItemProperty "HKLM:\Software\WOW6432Node\Microsoft\Windows\CurrentVersion\Uninstall\*" -ErrorAction SilentlyContinue |
        Where-Object { $_.DisplayName -like "USBip version*" } |
        Select-Object -First 1
}

if ($UsbipEntry -and $UsbipEntry.UninstallString) {
    Write-Host "  Click Yes on the Windows security prompt." -ForegroundColor Yellow
    $cmd = $UsbipEntry.UninstallString.Trim()
    if ($cmd -notmatch '(?i)/S\b') {
        $cmd = "$cmd /S"
    }
    Start-Process -FilePath "cmd.exe" -ArgumentList "/c `"$cmd`"" -Verb RunAs -Wait
    Write-Host "  Input driver removed." -ForegroundColor Green
    Write-Host ""
    Write-Host "Restart your computer to finish removal." -ForegroundColor Yellow
} else {
    Write-Host "  USBip driver not found (already removed)." -ForegroundColor Gray
}

Write-Host ""
Write-Host "Uninstall complete." -ForegroundColor Green
Write-Host ""
Write-Host "Delete this folder to remove the app:" -ForegroundColor Cyan
$folder = $env:BCC_INSTALL_DIR.TrimEnd('\')
Write-Host "  $folder" -ForegroundColor Gray
Write-Host ""
Write-Host "See README.txt for details." -ForegroundColor Gray
Write-Host ""
