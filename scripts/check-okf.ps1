<#
.SYNOPSIS
  Validates this repository's OKF bundle.

.DESCRIPTION
  A thin wrapper around the shared `okfcheck` tool that ships with the
  OpenKnowledgeFormat reference folder. This script deliberately contains no
  rules of its own: the rule set lives in one implementation shared by every
  repository, and this bundle's stricter policy (required frontmatter fields,
  bundle-absolute links, strict mode) lives in `okf/.okfcheck` beside the
  knowledge it governs. Two copies of the rules would drift.

  The reference folder is gitignored, so a clean checkout does not have it.
  Clone it next to this script's repository root before running:

      git clone https://github.com/mulavdm/OpenKnowledgeFormat.git
#>
$ErrorActionPreference = "Stop"

$RepoRoot = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path
$Bundle = Join-Path $RepoRoot "okf"
$Tool = Join-Path $RepoRoot "OpenKnowledgeFormat/tools/okfcheck"

if (-not (Test-Path -LiteralPath $Bundle)) {
    throw "No OKF bundle found at $Bundle"
}
if (-not (Test-Path -LiteralPath (Join-Path $Tool "main.go"))) {
    throw @"
The shared okfcheck tool is not present at:
  $Tool

It ships with the OpenKnowledgeFormat reference folder, which is gitignored.
Clone it into the repository root and re-run:
  git clone https://github.com/mulavdm/OpenKnowledgeFormat.git
"@
}

# The reference folder is its own Go module and sits outside any parent
# go.work, which would otherwise refuse to build it.
$PreviousGoWork = $env:GOWORK
Push-Location -LiteralPath $Tool
try {
    $env:GOWORK = "off"
    & go run . $Bundle
    $ExitCode = $LASTEXITCODE
} finally {
    Pop-Location
    if ($null -eq $PreviousGoWork) {
        Remove-Item Env:GOWORK -ErrorAction SilentlyContinue
    } else {
        $env:GOWORK = $PreviousGoWork
    }
}

if ($ExitCode -ne 0) {
    throw "OKF bundle is not conformant (okfcheck exit $ExitCode)."
}
