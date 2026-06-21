function Ensure-LicenseKeys {
    param([string]$ClickerDir)

    $publicPem = Join-Path $ClickerDir "license\public.pem"
    if (Test-Path $publicPem) {
        return
    }

    Write-Host "Generating license keys (first time only)..." -ForegroundColor Cyan
    Push-Location $ClickerDir
    go run .\cmd\ensurekeys
    if ($LASTEXITCODE -ne 0) {
        Pop-Location
        exit $LASTEXITCODE
    }
    Pop-Location
}
