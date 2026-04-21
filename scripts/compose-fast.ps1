[CmdletBinding()]
param(
    [ValidateSet("build", "up")]
    [string]$Action = "build",
    [switch]$NoCache
)

$ErrorActionPreference = "Stop"

# Force Docker Compose to use BuildKit when podman delegates to docker-compose.
$env:DOCKER_BUILDKIT = "1"
$env:COMPOSE_DOCKER_CLI_BUILD = "1"

$baseArgs = @("compose", "-f", "docker_compose.yaml")

if ($Action -eq "up") {
    $args = $baseArgs + @("up", "-d", "--build")
} else {
    $args = $baseArgs + @("build", "bot")
}

if ($NoCache) {
    $args += "--no-cache"
}

Write-Host "Running: podman $($args -join ' ')"
& podman @args
