# run_android_gallery.ps1
# Convenient helper script to launch POEM Android Component Gallery on emulator or device

$ErrorActionPreference = "Stop"
$POEM_DIR = $PSScriptRoot

$sdk = if ($env:ANDROID_SDK) { $env:ANDROID_SDK } else { "$env:LOCALAPPDATA\Android\Sdk" }
$adb = "$sdk\platform-tools\adb.exe"
$emulator = "$sdk\emulator\emulator.exe"
$apk = "$POEM_DIR\android_engine\gallery.apk"

# 1. Always build latest gallery.apk to reflect code changes
Write-Host "==> Building latest gallery.apk..." -ForegroundColor Cyan
& "C:\Program Files\Git\bin\bash.exe" "$POEM_DIR\android_engine\build_apk.sh" ../cmd/gallery com.poem.gallery Gallery "$apk" x86_64

# 2. Start emulator if no device is connected
$devices = & $adb devices | Select-String "\bdevice\b"
if (-not $devices) {
    Write-Host "==> Launching Android Emulator (Medium_Phone)..." -ForegroundColor Cyan
    Start-Process -FilePath $emulator -ArgumentList "-avd", "Medium_Phone"
    Write-Host "==> Waiting for emulator to boot..." -ForegroundColor Yellow
    & $adb wait-for-device
    $booted = $false
    for ($i = 0; $i -lt 30; $i++) {
        $status = (& $adb shell getprop sys.boot_completed 2>$null).Trim()
        if ($status -eq "1") {
            $booted = $true
            break
        }
        Start-Sleep -Seconds 2
    }
}

# 3. Force stop old app, uninstall/reinstall and launch
Write-Host "==> Installing updated gallery.apk..." -ForegroundColor Cyan
& $adb shell am force-stop com.poem.gallery 2>$null
& $adb install -r "$apk"

Write-Host "==> Launching POEM Gallery Activity..." -ForegroundColor Green
& $adb shell am start -n com.poem.gallery/android.app.NativeActivity

Write-Host "==> Android Gallery launched successfully!" -ForegroundColor Green
