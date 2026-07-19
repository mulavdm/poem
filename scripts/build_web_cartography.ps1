param(
    [string]$OutputDirectory = (Join-Path $PSScriptRoot "..\pkg\app\web\vector-assets")
)

$ErrorActionPreference = "Stop"
$projectRoot = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path
New-Item -ItemType Directory -Force -Path $OutputDirectory | Out-Null
$previousGOOS = $env:GOOS
$previousGOARCH = $env:GOARCH
$previousGOCACHE = $env:GOCACHE
try {
    $env:GOOS = "js"
    $env:GOARCH = "wasm"
    $env:GOCACHE = Join-Path $projectRoot "..\.gocache"
    & go build -ldflags "-s -w" -o (Join-Path $OutputDirectory "cartography.wasm") "$projectRoot\cmd\cartography-wasm"
    if ($LASTEXITCODE -ne 0) { throw "Go/WASM cartography build failed" }
    $goRuntime = Join-Path (& go env GOROOT) "lib\wasm\wasm_exec.js"
    Copy-Item -LiteralPath $goRuntime -Destination (Join-Path $OutputDirectory "wasm_exec.js") -Force
} finally {
    $env:GOOS = $previousGOOS
    $env:GOARCH = $previousGOARCH
    $env:GOCACHE = $previousGOCACHE
}

Get-ChildItem -LiteralPath $OutputDirectory | Select-Object Name, Length
