[CmdletBinding()]
param(
    [ValidateSet("build", "up")]
    [string]$Action = "build",
    [switch]$NoCache
)

$scriptPath = Join-Path $PSScriptRoot "scripts\compose-fast.ps1"
if (-not (Test-Path $scriptPath)) {
    throw "Missing script: $scriptPath"
}

if ($NoCache) {
    & powershell -NoProfile -File $scriptPath -Action $Action -NoCache
} else {
    & powershell -NoProfile -File $scriptPath -Action $Action
}
