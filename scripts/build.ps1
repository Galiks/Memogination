# Builds the Memomarium Windows executable (dist\Memomarium.exe).
# Requires Node.js, npm, and Go on PATH.
$ErrorActionPreference = 'Stop'
$root = Split-Path -Parent $PSScriptRoot

# 1. Build the frontend (web/dist is embedded into the binary).
Push-Location (Join-Path $root 'web')
try {
    npm ci
    npm run build
}
finally {
    Pop-Location
}

# 2. Cross-compile the Windows amd64 binary.
Push-Location $root
try {
    $env:CGO_ENABLED = '0'
    $env:GOOS = 'windows'
    $env:GOARCH = 'amd64'
    New-Item -ItemType Directory -Force -Path (Join-Path $root 'dist') | Out-Null
    go build -o (Join-Path $root 'dist\Memomarium.exe') ./cmd/memomarium
}
finally {
    Pop-Location
}

Write-Host "Built dist\Memomarium.exe"