[CmdletBinding()]
param(
    [string]$Version = "1.0.2",
    [switch]$RefreshTools
)

$ErrorActionPreference = "Stop"
$projectRoot = $PSScriptRoot
$toolsDir = Join-Path $projectRoot "tools"
$buildDir = Join-Path $projectRoot ".build"
$distDir = Join-Path $projectRoot "dist"
$ytDlpPath = Join-Path $toolsDir "yt-dlp.exe"
$ffmpegPath = Join-Path $toolsDir "ffmpeg.exe"
$denoPath = Join-Path $toolsDir "deno.exe"

function Get-RemoteFile {
    param([string]$Uri, [string]$Destination)
    Write-Host "Downloading $Uri"
    Invoke-WebRequest -Uri $Uri -OutFile $Destination -UseBasicParsing
}

New-Item -ItemType Directory -Force -Path $toolsDir, $buildDir, $distDir | Out-Null

if ($RefreshTools -or -not (Test-Path -LiteralPath $ytDlpPath)) {
    Get-RemoteFile `
        -Uri "https://github.com/yt-dlp/yt-dlp/releases/latest/download/yt-dlp.exe" `
        -Destination $ytDlpPath
}

if ($RefreshTools -or -not (Test-Path -LiteralPath $ffmpegPath)) {
    $ffmpegZip = Join-Path $buildDir "ffmpeg-win64-lgpl.zip"
    $ffmpegExtract = Join-Path $buildDir "ffmpeg-extracted"
    $resolvedRoot = [IO.Path]::GetFullPath($projectRoot).TrimEnd([IO.Path]::DirectorySeparatorChar) + [IO.Path]::DirectorySeparatorChar
    $resolvedExtract = [IO.Path]::GetFullPath($ffmpegExtract)
    if (-not $resolvedExtract.StartsWith($resolvedRoot, [StringComparison]::OrdinalIgnoreCase)) {
        throw "Unsafe build path: $resolvedExtract"
    }
    if (Test-Path -LiteralPath $ffmpegExtract) {
        Remove-Item -LiteralPath $ffmpegExtract -Recurse -Force
    }
    Get-RemoteFile `
        -Uri "https://github.com/BtbN/FFmpeg-Builds/releases/download/latest/ffmpeg-master-latest-win64-lgpl.zip" `
        -Destination $ffmpegZip
    Expand-Archive -LiteralPath $ffmpegZip -DestinationPath $ffmpegExtract -Force
    $foundFFmpeg = Get-ChildItem -LiteralPath $ffmpegExtract -Recurse -File -Filter "ffmpeg.exe" | Select-Object -First 1
    if (-not $foundFFmpeg) {
        throw "ffmpeg.exe was not found in the downloaded archive."
    }
    Copy-Item -LiteralPath $foundFFmpeg.FullName -Destination $ffmpegPath -Force
}

if ($RefreshTools -or -not (Test-Path -LiteralPath $denoPath)) {
    $denoZip = Join-Path $buildDir "deno-x86_64-pc-windows-msvc.zip"
    $denoExtract = Join-Path $buildDir "deno-extracted"
    $resolvedRoot = [IO.Path]::GetFullPath($projectRoot).TrimEnd([IO.Path]::DirectorySeparatorChar) + [IO.Path]::DirectorySeparatorChar
    $resolvedExtract = [IO.Path]::GetFullPath($denoExtract)
    if (-not $resolvedExtract.StartsWith($resolvedRoot, [StringComparison]::OrdinalIgnoreCase)) {
        throw "Unsafe build path: $resolvedExtract"
    }
    if (Test-Path -LiteralPath $denoExtract) {
        Remove-Item -LiteralPath $denoExtract -Recurse -Force
    }
    Get-RemoteFile `
        -Uri "https://github.com/denoland/deno/releases/latest/download/deno-x86_64-pc-windows-msvc.zip" `
        -Destination $denoZip
    Expand-Archive -LiteralPath $denoZip -DestinationPath $denoExtract -Force
    $foundDeno = Get-ChildItem -LiteralPath $denoExtract -Recurse -File -Filter "deno.exe" | Select-Object -First 1
    if (-not $foundDeno) {
        throw "deno.exe was not found in the downloaded archive."
    }
    Copy-Item -LiteralPath $foundDeno.FullName -Destination $denoPath -Force
}

foreach ($tool in @($ytDlpPath, $ffmpegPath, $denoPath)) {
    $header = [IO.File]::ReadAllBytes($tool)
    if ($header.Length -lt 2 -or $header[0] -ne 0x4D -or $header[1] -ne 0x5A) {
        throw "Invalid Windows executable: $tool"
    }
}

Push-Location $projectRoot
try {
    Write-Host "Downloading Go module dependencies..."
    & go mod download
    if ($LASTEXITCODE -ne 0) { throw "go mod download failed." }

    Write-Host "Generating Windows application manifest..."
    & go run github.com/akavel/rsrc@v0.10.2 -manifest app.manifest -o rsrc.syso
    if ($LASTEXITCODE -ne 0) { throw "Manifest generation failed." }

    Write-Host "Running tests..."
    & go test ./...
    if ($LASTEXITCODE -ne 0) { throw "Tests failed." }

    Write-Host "Building single-file EXE..."
    $ldFlags = "-H windowsgui -s -w -X main.version=$Version"
    $buildSucceeded = $false
    foreach ($attempt in 1..3) {
        & go build -trimpath -ldflags $ldFlags -o (Join-Path $distDir "YouTube2MP3.exe") .
        if ($LASTEXITCODE -eq 0) {
            $buildSucceeded = $true
            break
        }
        if ($attempt -lt 3) {
            Write-Host "Build file was temporarily unavailable; retrying ($attempt/3)..."
            Start-Sleep -Seconds 2
        }
    }
    if (-not $buildSucceeded) { throw "Build failed after 3 attempts." }

    Write-Host "Running packaged application self-test..."
    $selfTest = Start-Process -FilePath (Join-Path $distDir "YouTube2MP3.exe") -ArgumentList "--self-test" -Wait -PassThru -WindowStyle Hidden
    if ($selfTest.ExitCode -ne 0) { throw "Packaged application self-test failed." }
}
finally {
    Pop-Location
}

$result = Get-Item -LiteralPath (Join-Path $distDir "YouTube2MP3.exe")
Write-Host "Build complete: $($result.FullName) ($([Math]::Round($result.Length / 1MB, 1)) MB)"
