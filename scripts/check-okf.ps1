<#
.SYNOPSIS
  Validates this repository's OKF bundle.

.DESCRIPTION
  A thin wrapper around the shared `okfcheck` tool that ships with the
  OpenKnowledgeFormat reference repository. This script deliberately
  contains no rules of its own: the rule set lives in one implementation
  shared by every repository, and this bundle's stricter policy (required
  frontmatter fields, bundle-absolute links, strict mode) lives in
  `okf/.okfcheck` beside the knowledge it governs. Two copies of the rules
  would drift.

  Locally, every project shares one canonical reference checkout at the
  workspace root instead of holding its own copy. In CI, where the
  workspace root doesn't exist, the pipeline clones the reference
  repository directly next to this repository's root instead. This script
  checks the workspace-root location first, then falls back to a
  repo-root-adjacent checkout.
#>
$ErrorActionPreference = "Stop"

$RepoRoot = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path
$Bundle = Join-Path $RepoRoot "okf"

$CandidateRoots = @()
$WorkspaceRootPath = Join-Path $RepoRoot "../.."
if (Test-Path -LiteralPath $WorkspaceRootPath) {
    $CandidateRoots += (Resolve-Path $WorkspaceRootPath).Path
}
$CandidateRoots += $RepoRoot

$Tool = $null
foreach ($Root in $CandidateRoots) {
    $Candidate = Join-Path $Root "OpenKnowledgeFormat/tools/okfcheck"
    if (Test-Path -LiteralPath (Join-Path $Candidate "main.go")) {
        $Tool = $Candidate
        break
    }
}

if (-not (Test-Path -LiteralPath $Bundle)) {
    throw "No OKF bundle found at $Bundle"
}
if (-not $Tool) {
    $Searched = ($CandidateRoots | ForEach-Object { Join-Path $_ "OpenKnowledgeFormat/tools/okfcheck" }) -join "`n  "
    throw @"
The shared okfcheck tool was not found in any of:
  $Searched

Clone the reference repository into the workspace root (shared by every
project) or next to this repository's root, then re-run:
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
