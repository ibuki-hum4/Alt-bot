[CmdletBinding()]
param(
    [ValidateSet("build", "up", "db")]
    [string]$Action = "build",
    [switch]$NoCache,
    [switch]$Recreate
)

$scriptPath = Join-Path $PSScriptRoot "scripts\compose-fast.ps1"
if (-not (Test-Path $scriptPath)) {
    throw "Missing script: $scriptPath"
}

$scriptArgs = @("-Action", $Action)
if ($NoCache) {
    $scriptArgs += "-NoCache"
}
if ($Recreate) {
    $scriptArgs += "-Recreate"
}

& powershell -NoProfile -File $scriptPath @scriptArgs
