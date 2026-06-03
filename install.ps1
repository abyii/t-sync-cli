# install.ps1
# Build and install script for tsync CLI on Windows

$ErrorActionPreference = 'Stop'

Write-Host '==========================================' -ForegroundColor Cyan
Write-Host '  T-Sync CLI Installer                    ' -ForegroundColor Cyan
Write-Host '==========================================' -ForegroundColor Cyan

# 1. Check if Go is installed
Write-Host 'Checking for Go compiler...' -ForegroundColor Gray
$goVersion = & go version 2>&1
if ($LastExitCode -ne 0) {
    Write-Error 'Go is not installed or not in PATH. Please install Go from https://golang.org/'
    exit 1
}
Write-Host ('Found: ' + $goVersion) -ForegroundColor Green

# 2. Get GOPATH using go env
Write-Host 'Resolving target Go bin directory...' -ForegroundColor Gray
$goPath = & go env GOPATH
if (-not $goPath) {
    $goPath = ($env:USERPROFILE + '\go')
}
$binDir = Join-Path $goPath 'bin'
$targetExe = Join-Path $binDir 'tsync.exe'

Write-Host ('Target path: ' + $targetExe) -ForegroundColor Gray

# 3. Compile the Go CLI
Write-Host 'Building tsync.exe...' -ForegroundColor Gray
$startTime = [System.DateTime]::Now
& go build -o tsync.exe main.go
if ($LastExitCode -ne 0) {
    Write-Error 'Failed to compile the tsync CLI.'
    exit 1
}
$buildDuration = ([System.DateTime]::Now - $startTime).TotalSeconds
Write-Host ('Build completed successfully in ' + $buildDuration.ToString('F2') + ' seconds.') -ForegroundColor Green

# 4. Copy to target directory
if (-not (Test-Path $binDir)) {
    Write-Host ('Creating directory: ' + $binDir) -ForegroundColor Gray
    New-Item -ItemType Directory -Force -Path $binDir | Out-Null
}

Write-Host ('Installing tsync.exe to ' + $binDir + ' ...') -ForegroundColor Gray
Copy-Item -Path 'tsync.exe' -Destination $targetExe -Force
Remove-Item -Path 'tsync.exe' -ErrorAction SilentlyContinue

Write-Host '==========================================' -ForegroundColor Green
Write-Host '✔ T-Sync CLI successfully installed!' -ForegroundColor Green
Write-Host ('Location: ' + $targetExe) -ForegroundColor Green
Write-Host '==========================================' -ForegroundColor Green

# Check if PATH contains the directory
$isInPath = $false
$paths = $env:PATH -split ';'
foreach ($p in $paths) {
    if ($p.TrimEnd('\') -eq $binDir.TrimEnd('\')) {
        $isInPath = $true
        break
    }
}

if (-not $isInPath) {
    Write-Host ''
    Write-Host ('Warning: ' + $binDir + ' is not in your environment PATH.') -ForegroundColor Yellow
    Write-Host 'To run tsync from anywhere, add it to your PATH environment variable.' -ForegroundColor Yellow
} else {
    Write-Host 'You can now run tsync from any terminal window.' -ForegroundColor Green
}
