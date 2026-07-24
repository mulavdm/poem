# run_gallery.ps1
# Convenient helper script to launch POEM Desktop Component Gallery

$ErrorActionPreference = "Stop"
$POEM_DIR = Get-Location
$exe = "$POEM_DIR\dist\portable\POEMGallery.exe"

if (-not (Test-Path $exe)) {
    Write-Host "==> Building POEMGallery.exe..." -ForegroundColor Cyan
    powershell -ExecutionPolicy Bypass -File "$POEM_DIR\windows_host\build.ps1" -ProjectDirectory . -GoPackage ./cmd/gallery -Identity POEM.Gallery -DisplayName "POEM Gallery" -Version 0.7.0.0 -OutputDirectory ./dist
}

Write-Host "==> Launching POEM Desktop Gallery..." -ForegroundColor Green
Start-Process -FilePath $exe
