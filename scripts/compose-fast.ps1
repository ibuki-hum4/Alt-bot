[CmdletBinding()]
param(
    [ValidateSet("build", "up", "db")]
    [string]$Action = "build",
    [switch]$NoCache,
    [switch]$Recreate
)

$ErrorActionPreference = "Stop"

# Force Docker Compose to use BuildKit when podman delegates to docker-compose.
$env:DOCKER_BUILDKIT = "1"
$env:COMPOSE_DOCKER_CLI_BUILD = "1"

$baseArgs = @("compose", "--env-file", ".env", "-f", "docker_compose.yaml")

if ($Action -eq "up") {
    $args = $baseArgs + @("up", "-d", "--build")
    if ($Recreate) {
        $args += "--force-recreate"
    }
} elseif ($Action -eq "db") {
    $args = $baseArgs + @("up", "-d", "postgres")
    if ($Recreate) {
        $args += "--force-recreate"
    }
} else {
    $args = $baseArgs + @("build", "bot")
}

if ($NoCache) {
    $args += "--no-cache"
}

Write-Host "Running: podman $($args -join ' ')"
& podman @args
