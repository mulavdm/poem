[CmdletBinding()]
param(
    [Parameter(Mandatory = $true)][string]$ProjectDirectory,
    [Parameter(Mandatory = $true)][string]$GoPackage,
    [Parameter(Mandatory = $true)][ValidatePattern('^[A-Za-z0-9][A-Za-z0-9.-]{2,127}$')][string]$Identity,
    [Parameter(Mandatory = $true)][string]$DisplayName,
    [Parameter(Mandatory = $true)][ValidatePattern('^\d+\.\d+\.\d+\.\d+$')][string]$Version,
    [Parameter(Mandatory = $true)][string]$OutputDirectory,
    [string]$ExecutableName,
    [string]$Icon,
    [string]$Publisher = 'CN=POEM Development',
    [string]$CertificatePath,
    [string]$CertificateThumbprint,
    [string]$CertificatePasswordEnvironment = 'POEM_WINDOWS_CERT_PASSWORD',
    [switch]$SkipMSIX
)

$ErrorActionPreference = 'Stop'
$poemRoot = Split-Path -Parent $PSScriptRoot
$projectRoot = (Resolve-Path -LiteralPath $ProjectDirectory).Path
$outputRoot = [System.IO.Path]::GetFullPath($OutputDirectory)
if (-not $ExecutableName) {
    $ExecutableName = ($DisplayName -replace '[^A-Za-z0-9._-]', '')
}
if (-not $ExecutableName) { throw 'ExecutableName resolves to an empty filename.' }
if ($DisplayName.Length -lt 1 -or $DisplayName.Length -gt 256) { throw 'DisplayName must contain 1-256 characters.' }
$xmlDisplayName = [Security.SecurityElement]::Escape($DisplayName)
$xmlPublisher = [Security.SecurityElement]::Escape($Publisher)

$hostBuild = Join-Path $poemRoot 'cpp_sidecar\build-package'
$portable = Join-Path $outputRoot 'portable'
$msixStage = Join-Path $outputRoot 'msix-stage'
New-Item -ItemType Directory -Force -Path $outputRoot | Out-Null
foreach ($stage in @($portable, $msixStage)) {
    if ([System.IO.Path]::GetDirectoryName([System.IO.Path]::GetFullPath($stage)) -ne $outputRoot) {
        throw "Refusing to clean packaging path outside $outputRoot"
    }
    if (Test-Path -LiteralPath $stage) { Remove-Item -LiteralPath $stage -Recurse -Force }
}
New-Item -ItemType Directory -Force -Path $portable | Out-Null

cmake -S (Join-Path $poemRoot 'cpp_sidecar') -B $hostBuild -A x64
if ($LASTEXITCODE -ne 0) { throw 'CMake configuration failed.' }
cmake --build $hostBuild --config Release --target poem_windows_host
if ($LASTEXITCODE -ne 0) { throw 'Windows host build failed.' }

$gcc = (Get-Command gcc.exe -ErrorAction SilentlyContinue).Source
if (-not $gcc -and (Test-Path -LiteralPath 'C:\msys64\ucrt64\bin\gcc.exe')) {
    $gcc = 'C:\msys64\ucrt64\bin\gcc.exe'
}
if (-not $gcc) { throw 'UCRT64 gcc.exe is required to build the Go shared library.' }

$previousCGO = $env:CGO_ENABLED
$previousCC = $env:CC
try {
    $env:CGO_ENABLED = '1'
    $env:CC = $gcc
    Push-Location $projectRoot
    try {
        go build -buildmode=c-shared -trimpath -o (Join-Path $portable 'poem_app.dll') $GoPackage
        if ($LASTEXITCODE -ne 0) { throw 'Go application DLL build failed.' }
    } finally {
        Pop-Location
    }
} finally {
    $env:CGO_ENABLED = $previousCGO
    $env:CC = $previousCC
}

$hostExe = Join-Path $hostBuild 'Release\poem_windows_host.exe'
Copy-Item -LiteralPath $hostExe -Destination (Join-Path $portable "$ExecutableName.exe") -Force
$notices = Join-Path $poemRoot 'LICENSE'
if (Test-Path -LiteralPath $notices) { Copy-Item -LiteralPath $notices -Destination $portable -Force }
@{
    abi = 1
    identity = $Identity
    displayName = $DisplayName
    version = $Version
    architecture = 'x64'
    contents = @("$ExecutableName.exe", 'poem_app.dll')
} | ConvertTo-Json -Depth 3 | Set-Content -LiteralPath (Join-Path $portable 'bundle.json') -Encoding utf8

$zip = Join-Path $outputRoot "$ExecutableName-$Version-windows-x64.zip"
Compress-Archive -Path (Join-Path $portable '*') -DestinationPath $zip -Force

if (-not $SkipMSIX) {
    New-Item -ItemType Directory -Force -Path $msixStage, (Join-Path $msixStage 'Assets') | Out-Null
    Copy-Item -LiteralPath (Join-Path $portable "$ExecutableName.exe"), (Join-Path $portable 'poem_app.dll') -Destination $msixStage

    Add-Type -AssemblyName System.Drawing
    function Write-AppLogo([string]$Path, [int]$Size) {
        $bitmap = [System.Drawing.Bitmap]::new($Size, $Size)
        try {
            $graphics = [System.Drawing.Graphics]::FromImage($bitmap)
            try {
                $graphics.Clear([System.Drawing.Color]::FromArgb(255, 37, 99, 235))
                if ($Icon) {
                    $source = [System.Drawing.Image]::FromFile((Resolve-Path -LiteralPath $Icon).Path)
                    try { $graphics.DrawImage($source, 0, 0, $Size, $Size) } finally { $source.Dispose() }
                } else {
                    $font = [System.Drawing.Font]::new('Segoe UI', [Math]::Max(12, $Size / 2), [System.Drawing.FontStyle]::Bold)
                    try {
                        $format = [System.Drawing.StringFormat]::new()
                        $format.Alignment = [System.Drawing.StringAlignment]::Center
                        $format.LineAlignment = [System.Drawing.StringAlignment]::Center
                        $graphics.DrawString('P', $font, [System.Drawing.Brushes]::White, [System.Drawing.RectangleF]::new(0, 0, $Size, $Size), $format)
                        $format.Dispose()
                    } finally { $font.Dispose() }
                }
            } finally { $graphics.Dispose() }
            $bitmap.Save($Path, [System.Drawing.Imaging.ImageFormat]::Png)
        } finally { $bitmap.Dispose() }
    }
    Write-AppLogo (Join-Path $msixStage 'Assets\Square44x44Logo.png') 44
    Write-AppLogo (Join-Path $msixStage 'Assets\Square150x150Logo.png') 150

    $manifest = @"
<?xml version="1.0" encoding="utf-8"?>
<Package xmlns="http://schemas.microsoft.com/appx/manifest/foundation/windows10" xmlns:uap="http://schemas.microsoft.com/appx/manifest/uap/windows10" xmlns:rescap="http://schemas.microsoft.com/appx/manifest/foundation/windows10/restrictedcapabilities" IgnorableNamespaces="uap rescap">
  <Identity Name="$Identity" Publisher="$xmlPublisher" Version="$Version" ProcessorArchitecture="x64" />
  <Properties><DisplayName>$xmlDisplayName</DisplayName><PublisherDisplayName>POEM</PublisherDisplayName><Logo>Assets\Square44x44Logo.png</Logo></Properties>
  <Dependencies><TargetDeviceFamily Name="Windows.Desktop" MinVersion="10.0.17763.0" MaxVersionTested="10.0.26100.0" /></Dependencies>
  <Resources><Resource Language="en-us" /></Resources>
  <Applications><Application Id="App" Executable="$ExecutableName.exe" EntryPoint="Windows.FullTrustApplication"><uap:VisualElements DisplayName="$xmlDisplayName" Description="$xmlDisplayName" BackgroundColor="transparent" Square44x44Logo="Assets\Square44x44Logo.png" Square150x150Logo="Assets\Square150x150Logo.png" /></Application></Applications>
  <Capabilities><rescap:Capability Name="runFullTrust" /></Capabilities>
</Package>
"@
    $manifest | Set-Content -LiteralPath (Join-Path $msixStage 'AppxManifest.xml') -Encoding utf8

    $makeAppx = (Get-Command MakeAppx.exe -ErrorAction SilentlyContinue).Source
    if (-not $makeAppx) {
        $makeAppx = Get-ChildItem -Path "${env:ProgramFiles(x86)}\Windows Kits\10\bin" -Recurse -Filter MakeAppx.exe -ErrorAction SilentlyContinue |
            Where-Object { $_.FullName -match '\\x64\\MakeAppx\.exe$' } | Sort-Object FullName -Descending | Select-Object -First 1 -ExpandProperty FullName
    }
    if (-not $makeAppx) { throw 'Windows SDK MakeAppx.exe was not found.' }
    $msix = Join-Path $outputRoot "$ExecutableName-$Version-x64.msix"
    & $makeAppx pack /d $msixStage /p $msix /o
    if ($LASTEXITCODE -ne 0) { throw 'MSIX construction failed.' }

    if ($CertificatePath -or $CertificateThumbprint) {
        $signTool = (Get-Command SignTool.exe -ErrorAction SilentlyContinue).Source
        if (-not $signTool) {
            $signTool = Get-ChildItem -Path "${env:ProgramFiles(x86)}\Windows Kits\10\bin" -Recurse -Filter SignTool.exe -ErrorAction SilentlyContinue |
                Where-Object { $_.FullName -match '\\x64\\SignTool\.exe$' } | Sort-Object FullName -Descending | Select-Object -First 1 -ExpandProperty FullName
        }
        if (-not $signTool) { throw 'Windows SDK SignTool.exe was not found.' }
        if ($CertificatePath) {
            $password = [Environment]::GetEnvironmentVariable($CertificatePasswordEnvironment)
            if (-not $password) { throw "Certificate password environment variable $CertificatePasswordEnvironment is empty." }
            & $signTool sign /fd SHA256 /f $CertificatePath /p $password $msix
        } else {
            & $signTool sign /fd SHA256 /sha1 $CertificateThumbprint $msix
        }
        if ($LASTEXITCODE -ne 0) { throw 'MSIX signing failed.' }
    }
}

Write-Host "Portable bundle: $portable"
Write-Host "Portable ZIP: $zip"
if (-not $SkipMSIX) { Write-Host "MSIX: $msix" }
