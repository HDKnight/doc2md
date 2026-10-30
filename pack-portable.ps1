#Requires -Version 5.1
# Build a click-to-run portable folder (and optional zip).
# Usage:
#   powershell -File pack-portable.ps1
#   powershell -File pack-portable.ps1 -Zip
param(
    [switch]$Zip,
    [string]$OutDir = ""
)
$ErrorActionPreference = "Stop"

$Root = Split-Path -Parent $MyInvocation.MyCommand.Path
Set-Location $Root

if (-not $OutDir) {
    $OutDir = Join-Path $Root "dist\doc2md-portable"
}

function Write-Step([string]$msg) { Write-Host ("[pack] " + $msg) }

if (-not (Test-Path (Join-Path $Root "doc2md.exe"))) {
    Write-Step "Building doc2md.exe ..."
    & go build -o doc2md.exe ./cmd/server
    if ($LASTEXITCODE -ne 0) { throw "go build failed" }
}

Write-Step "ensure-deps (python/poppler/fpaddle/oar) ..."
& powershell -NoProfile -ExecutionPolicy Bypass -File (Join-Path $Root "ensure-deps.ps1")
if ($LASTEXITCODE -ne 0) { throw "ensure-deps failed" }

$oarSrc = Join-Path $Root "tools\oar.exe"
if (-not (Test-Path $oarSrc)) { throw "tools\oar.exe missing after ensure-deps" }

$userOar = Join-Path $env:USERPROFILE ".oar"
if (-not (Test-Path $userOar)) {
    throw "No $userOar — run one 表征/structure convert once to download models, then re-pack."
}

if (Test-Path $OutDir) {
    Write-Step "Cleaning $OutDir ..."
    Remove-Item $OutDir -Recurse -Force
}
New-Item -ItemType Directory -Force -Path $OutDir | Out-Null
New-Item -ItemType Directory -Force -Path (Join-Path $OutDir "tools") | Out-Null

function Copy-Tree([string]$Src, [string]$Dst) {
    if (-not (Test-Path $Src)) { throw "missing: $Src" }
    Write-Step ("Copy " + $Src + " -> " + $Dst)
    New-Item -ItemType Directory -Force -Path (Split-Path $Dst -Parent) | Out-Null
    & robocopy $Src $Dst /E /NFL /NDL /NJH /NJS /nc /ns /np | Out-Null
    # robocopy codes 0-7 are OK
    if ($LASTEXITCODE -ge 8) { throw "robocopy failed $Src ($LASTEXITCODE)" }
}

Write-Step "Copy runtime files ..."
Copy-Item (Join-Path $Root "doc2md.exe") (Join-Path $OutDir "doc2md.exe") -Force
Copy-Item (Join-Path $Root "start.bat") (Join-Path $OutDir "start.bat") -Force
Copy-Item (Join-Path $Root "stop.bat") (Join-Path $OutDir "stop.bat") -Force
Copy-Item (Join-Path $Root "ensure-deps.ps1") (Join-Path $OutDir "ensure-deps.ps1") -Force
Copy-Item $oarSrc (Join-Path $OutDir "tools\oar.exe") -Force
$structWorker = Join-Path $Root "tools\oar_structure_worker.exe"
if (Test-Path $structWorker) {
    Copy-Item $structWorker (Join-Path $OutDir "tools\oar_structure_worker.exe") -Force
} else {
    Write-Step "WARN: tools\oar_structure_worker.exe missing (portable will use cold oar CLI)"
}

Copy-Tree (Join-Path $Root "tools\poppler") (Join-Path $OutDir "tools\poppler")
Copy-Tree (Join-Path $Root "tools\python") (Join-Path $OutDir "tools\python")
Copy-Tree $userOar (Join-Path $OutDir "tools\oar-home")

# Install OCR into bundled portable Python (path-independent; no fragile venv relocate)
$py = Join-Path $OutDir "tools\python\python.exe"
if (-not (Test-Path $py)) { throw "tools\python\python.exe missing in pack" }
Write-Step "pip install faster-paddle into tools\python (portable OCR) ..."
$mirror = "https://pypi.tuna.tsinghua.edu.cn/simple"
& $py -m pip install -U pip -q
& $py -m pip install faster-paddle -i $mirror --trusted-host pypi.tuna.tsinghua.edu.cn
if ($LASTEXITCODE -ne 0) {
    & $py -m pip install faster-paddle
}
if ($LASTEXITCODE -ne 0) { throw "pip install faster-paddle failed" }
& $py -c "import faster_paddle"
if ($LASTEXITCODE -ne 0) { throw "import faster_paddle failed in portable python" }

# Marker: start.bat / ensure-deps treat as fully bundled
Set-Content -Path (Join-Path $OutDir ".portable") -Value "doc2md-portable`n" -Encoding ASCII

$readme = @"
doc2md portable (click to run)

1. Unzip / copy this folder anywhere
2. Double-click start.bat
3. Upload PDF/DOCX in the browser; feat pipeline works (models bundled)

Stop: close the console window, or run stop.bat
Port: start.bat 9000

Note: package is large (~1GB with oar models). Do not copy only doc2md.exe.
"@
$utf8Bom = New-Object System.Text.UTF8Encoding $true
[System.IO.File]::WriteAllText((Join-Path $OutDir "README.txt"), $readme, $utf8Bom)
# remove garbled leftover if re-packing over old tree (fresh OutDir usually)
Remove-Item (Join-Path $OutDir "使用说明.txt") -Force -EA SilentlyContinue
Remove-Item (Join-Path $OutDir "浣跨敤璇存槑.txt") -Force -EA SilentlyContinue

Write-Step "Portable folder ready: $OutDir"
$mb = [math]::Round(((Get-ChildItem $OutDir -Recurse -File -EA SilentlyContinue | Measure-Object Length -Sum).Sum) / 1MB, 1)
Write-Step ("Size ~ {0} MB" -f $mb)

if ($Zip) {
    $zipPath = Join-Path $Root ("dist\doc2md-portable-{0}.zip" -f (Get-Date -Format "yyyyMMdd"))
    if (Test-Path $zipPath) { Remove-Item $zipPath -Force }
    Write-Step "Zipping $zipPath ..."
    Compress-Archive -Path $OutDir -DestinationPath $zipPath -CompressionLevel Optimal
    Write-Step "Zip ready: $zipPath"
}

Write-Step "Done. Copy/unzip the folder on another PC and double-click start.bat."
exit 0
