# Belarus Champ Clicker - one-time setup for end users
$ErrorActionPreference = "Stop"

$AppDisplayName = "Belarus Champ Clicker"
$AppExeName = "Belarus Champ Clicker.exe"
$InstallDir = Join-Path $env:LOCALAPPDATA "BelarusChampClicker"
$SourceExe = Join-Path $PSScriptRoot $AppExeName

Write-Host ""
Write-Host "  $AppDisplayName - Setup" -ForegroundColor Cyan
Write-Host ""

if (-not (Test-Path $SourceExe)) {
    Write-Host "Error: Could not find '$AppExeName' in this folder." -ForegroundColor Red
    Write-Host "Extract the full ZIP package and run Install.cmd from inside it." -ForegroundColor Yellow
    exit 1
}

Write-Host "Installing application..." -ForegroundColor Cyan
New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
Copy-Item $SourceExe (Join-Path $InstallDir $AppExeName) -Force
Write-Host "  Installed to: $InstallDir" -ForegroundColor Green

$WshShell = New-Object -ComObject WScript.Shell
$DesktopShortcut = $WshShell.CreateShortcut((Join-Path $env:USERPROFILE "Desktop\$AppDisplayName.lnk"))
$DesktopShortcut.TargetPath = Join-Path $InstallDir $AppExeName
$DesktopShortcut.WorkingDirectory = $InstallDir
$DesktopShortcut.Description = $AppDisplayName
$DesktopShortcut.Save()
Write-Host "  Desktop shortcut created." -ForegroundColor Green

$StartMenuDir = Join-Path $env:APPDATA "Microsoft\Windows\Start Menu\Programs"
$StartShortcut = $WshShell.CreateShortcut((Join-Path $StartMenuDir "$AppDisplayName.lnk"))
$StartShortcut.TargetPath = Join-Path $InstallDir $AppExeName
$StartShortcut.WorkingDirectory = $InstallDir
$StartShortcut.Description = $AppDisplayName
$StartShortcut.Save()
Write-Host "  Start menu shortcut created." -ForegroundColor Green

Write-Host ""
Write-Host "Checking input driver..." -ForegroundColor Cyan

$UsbipTargetVersion = [Version]"0.9.7.7"
$UsbipInstalledVersion = $null

$UsbipEntry = Get-ItemProperty "HKLM:\Software\Microsoft\Windows\CurrentVersion\Uninstall\*" -ErrorAction SilentlyContinue |
    Where-Object { $_.DisplayName -like 'USBip version*' } |
    Select-Object -First 1
if (-not $UsbipEntry) {
    $UsbipEntry = Get-ItemProperty "HKLM:\Software\WOW6432Node\Microsoft\Windows\CurrentVersion\Uninstall\*" -ErrorAction SilentlyContinue |
        Where-Object { $_.DisplayName -like 'USBip version*' } |
        Select-Object -First 1
}
if ($UsbipEntry) {
    try { $UsbipInstalledVersion = [Version]$UsbipEntry.DisplayVersion } catch { }
}

if (-not $UsbipInstalledVersion) {
    $DriverPath = Join-Path $env:SystemRoot "System32\drivers\usbip2_ude.sys"
    if (Test-Path $DriverPath) {
        try { $UsbipInstalledVersion = [Version](Get-Item $DriverPath).VersionInfo.FileVersion } catch { }
    }
}

$NeedsReboot = $false
$NeedsDriverInstall = $true

if ($UsbipInstalledVersion -and $UsbipInstalledVersion -ge $UsbipTargetVersion) {
    Write-Host "  Input driver is already installed." -ForegroundColor Green
    $NeedsDriverInstall = $false
}
elseif ($UsbipInstalledVersion) {
    Write-Host "  Input driver is outdated. Updating..." -ForegroundColor Yellow
}
else {
    Write-Host "  Input driver not found. Installing..." -ForegroundColor Yellow
}

if ($NeedsDriverInstall) {
    Write-Host ""
    Write-Host "  Administrator permission is required for the input driver." -ForegroundColor Yellow
    Write-Host "  Click Yes on the Windows security prompt." -ForegroundColor Yellow
    Write-Host ""

    $TempDir = New-TemporaryFile | ForEach-Object { Remove-Item $_; New-Item -ItemType Directory -Path $_ }
    try {
        $UsbipInstallerUrl = "https://github.com/vadimgrn/usbip-win2/releases/download/v.0.9.7.7/USBip-0.9.7.7-x64.exe"
        $UsbipInstaller = Join-Path $TempDir "USBip-setup.exe"
        Invoke-WebRequest -Uri $UsbipInstallerUrl -OutFile $UsbipInstaller -ErrorAction Stop
        Start-Process -FilePath $UsbipInstaller -ArgumentList "/S" -Verb RunAs -Wait
        Write-Host "  Input driver installed." -ForegroundColor Green
        $NeedsReboot = $true
    }
    catch {
        Write-Host "  Warning: Could not install the input driver automatically." -ForegroundColor Red
        Write-Host "  $($_.Exception.Message)" -ForegroundColor Red
        Write-Host ""
        Write-Host "  Download and install manually from:" -ForegroundColor Yellow
        Write-Host "  https://github.com/vadimgrn/usbip-win2/releases" -ForegroundColor Yellow
        Write-Host "  Then restart your PC and run this setup again." -ForegroundColor Yellow
    }
    finally {
        Remove-Item -Recurse -Force $TempDir -ErrorAction SilentlyContinue
    }
}

Write-Host ""
Write-Host "Setup complete!" -ForegroundColor Green
Write-Host ""
Write-Host "Next steps:" -ForegroundColor Cyan
Write-Host "  1. Open '$AppDisplayName' from your Desktop or Start menu"
Write-Host "  2. Enter your activation code when prompted (see INSTALL.txt)"
Write-Host "  3. Click Start in the app, bind a trigger key, then launch your game"
Write-Host ""

if ($NeedsReboot) {
    Write-Host "IMPORTANT: Restart your computer before using the clicker." -ForegroundColor Yellow
    Write-Host "The input driver needs a reboot to work." -ForegroundColor Yellow
    Write-Host ""
}

Write-Host 'See INSTALL.txt or INSTALL.ru.txt for full instructions.' -ForegroundColor Gray
Write-Host ""
