$ErrorActionPreference = "Stop"

$clickerDir = $PSScriptRoot

. (Join-Path $clickerDir "scripts\build-common.ps1")

Write-Host "Building author tools..." -ForegroundColor Cyan

Push-Location $clickerDir

Ensure-LicenseKeys -ClickerDir $clickerDir

Remove-Item (Join-Path $clickerDir "licgen.exe") -Force -ErrorAction SilentlyContinue
Remove-Item (Join-Path $clickerDir "licgen-gui.exe~") -Force -ErrorAction SilentlyContinue

Write-Host "Generating manifest..." -ForegroundColor Cyan
$licgenDir = Join-Path $clickerDir "cmd\licgen-gui"
Remove-Item "$licgenDir\*.syso" -Force -ErrorAction SilentlyContinue
Push-Location $licgenDir
go run github.com/akavel/rsrc@v0.10.2 -manifest ..\..\gui\app.manifest -o rsrc.syso
if ($LASTEXITCODE -ne 0) { Pop-Location; Pop-Location; exit $LASTEXITCODE }
Pop-Location

go build -trimpath -ldflags="-H windowsgui" -o licgen-gui.exe .\cmd\licgen-gui
if ($LASTEXITCODE -ne 0) { Pop-Location; exit $LASTEXITCODE }

Pop-Location

Write-Host ""
Write-Host "Done. Double-click:" -ForegroundColor Green
Write-Host "  $clickerDir\licgen-gui.exe" -ForegroundColor Yellow
Write-Host ""
Write-Host "Use this to create activation codes. No console needed." -ForegroundColor Gray
