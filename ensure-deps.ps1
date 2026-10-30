#Requires -Version 5.1
# Portable deps next to start.bat: tools/python + tools/poppler + .venv-fpaddle + tools/oar.exe
$ErrorActionPreference = "Stop"
$Root = Split-Path -Parent $MyInvocation.MyCommand.Path
Set-Location $Root

$PopplerDir = Join-Path $Root "tools\poppler"
$PopplerBin = Join-Path $PopplerDir "Library\bin"
$Pdftoppm = Join-Path $PopplerBin "pdftoppm.exe"
$LocalPyDir = Join-Path $Root "tools\python"
$LocalPy = Join-Path $LocalPyDir "python.exe"
$VenvPy = Join-Path $Root ".venv-fpaddle\Scripts\python.exe"
$OarExe = Join-Path $Root "tools\oar.exe"
$OarZipUrl = $env:DOC2MD_OAR_ZIP_URL
if (-not $OarZipUrl) {
    # Optional: set DOC2MD_OAR_ZIP_URL to a zip that contains oar.exe for air-gapped / no-Rust PCs.
    $OarZipUrl = ""
}

$PopplerZipUrl = "https://github.com/oschwartz10612/poppler-windows/releases/download/v26.09.0-0/Release-26.09.0-0.zip"
$PyVer = "3.12.10"
$PyInstallerName = "python-$PyVer-amd64.exe"
$PyUrls = @(
    "https://mirrors.huaweicloud.com/python/$PyVer/$PyInstallerName",
    "https://www.python.org/ftp/python/$PyVer/$PyInstallerName"
)
$PipMirror = "https://pypi.tuna.tsinghua.edu.cn/simple"

function Write-Step([string]$msg) {
    Write-Host ("[doc2md] " + $msg)
}

function Download-File([string[]]$Urls, [string]$OutFile) {
    [Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12
    $last = $null
    foreach ($u in $Urls) {
        try {
            Write-Step ("Downloading: " + $u)
            Invoke-WebRequest -Uri $u -OutFile $OutFile -UseBasicParsing
            if ((Test-Path -LiteralPath $OutFile) -and ((Get-Item -LiteralPath $OutFile).Length -gt 1MB)) {
                return
            }
        } catch {
            $last = $_
            Write-Step ("download failed, try next mirror...")
        }
    }
    throw ("download failed: " + $last)
}

function Test-PythonOk([string]$Py) {
    if (-not (Test-Path -LiteralPath $Py)) { return $false }
    try {
        $ver = & $Py -c "import sys; print(sys.version_info[:2] >= (3,10))" 2>$null
        return ($ver -match "True")
    } catch {
        return $false
    }
}

function Ensure-Poppler {
    if (Test-Path -LiteralPath $Pdftoppm) {
        Write-Step ("Poppler OK: " + $Pdftoppm)
        return
    }
    Write-Step "Poppler missing; installing into tools\poppler ..."
    New-Item -ItemType Directory -Force -Path (Join-Path $Root "tools") | Out-Null
    $tmpZip = Join-Path $env:TEMP "doc2md-poppler.zip"
    $tmpExtract = Join-Path $env:TEMP "doc2md-poppler-extract"
    if (Test-Path $tmpExtract) { Remove-Item $tmpExtract -Recurse -Force }
    New-Item -ItemType Directory -Force -Path $tmpExtract | Out-Null

    try {
        Download-File @($PopplerZipUrl) $tmpZip
    } catch {
        Write-Host ("ERROR: Poppler download failed: " + $_) -ForegroundColor Red
        Write-Host $PopplerZipUrl
        exit 1
    }

    Expand-Archive -LiteralPath $tmpZip -DestinationPath $tmpExtract -Force
    $binDir = Get-ChildItem -Path $tmpExtract -Recurse -Directory -Filter "bin" -EA SilentlyContinue |
        Where-Object { Test-Path (Join-Path $_.FullName "pdftoppm.exe") } |
        Select-Object -First 1
    if (-not $binDir) {
        Write-Host "ERROR: pdftoppm.exe not found in zip" -ForegroundColor Red
        exit 1
    }
    $libraryDir = Split-Path $binDir.FullName -Parent
    $packRoot = Split-Path $libraryDir -Parent
    if (Test-Path $PopplerDir) { Remove-Item $PopplerDir -Recurse -Force }
    New-Item -ItemType Directory -Force -Path $PopplerDir | Out-Null
    Copy-Item -Path $libraryDir -Destination (Join-Path $PopplerDir "Library") -Recurse -Force
    Get-ChildItem $packRoot -Force | Where-Object { $_.Name -ne "Library" } | ForEach-Object {
        Copy-Item $_.FullName -Destination (Join-Path $PopplerDir $_.Name) -Recurse -Force -EA SilentlyContinue
    }
    Remove-Item $tmpZip -Force -EA SilentlyContinue
    Remove-Item $tmpExtract -Recurse -Force -EA SilentlyContinue

    if (-not (Test-Path -LiteralPath $Pdftoppm)) {
        Write-Host "ERROR: Poppler install incomplete" -ForegroundColor Red
        exit 1
    }
    Write-Step "Poppler installed"
}

function Find-AnyPython {
    if (Test-PythonOk $LocalPy) { return $LocalPy }
    $cands = @()
    foreach ($n in @("python", "python3", "py")) {
        $cmd = Get-Command $n -EA SilentlyContinue
        if ($cmd) { $cands += $cmd.Source }
    }
    foreach ($p in @(
        "D:\Python314\python.exe",
        "C:\Python314\python.exe",
        "C:\Python312\python.exe",
        "C:\Python311\python.exe",
        "$env:LOCALAPPDATA\Programs\Python\Python312\python.exe",
        "$env:LOCALAPPDATA\Programs\Python\Python311\python.exe"
    )) {
        if (Test-Path -LiteralPath $p) { $cands += $p }
    }
    foreach ($py in $cands) {
        if (Test-PythonOk $py) { return $py }
    }
    return $null
}

function Ensure-Python {
    $existing = Find-AnyPython
    if ($existing) {
        Write-Step ("Python OK: " + $existing)
        return $existing
    }

    Write-Step "Python missing; installing portable Python into tools\python ..."
    New-Item -ItemType Directory -Force -Path (Join-Path $Root "tools") | Out-Null
    $installer = Join-Path $env:TEMP $PyInstallerName
    try {
        Download-File $PyUrls $installer
    } catch {
        Write-Host ("ERROR: Python download failed: " + $_) -ForegroundColor Red
        exit 1
    }

    if (Test-Path $LocalPyDir) { Remove-Item $LocalPyDir -Recurse -Force }
    New-Item -ItemType Directory -Force -Path $LocalPyDir | Out-Null

    $args = @(
        "/quiet",
        "InstallAllUsers=0",
        ("TargetDir=" + $LocalPyDir),
        "PrependPath=0",
        "Shortcuts=0",
        "Include_doc=0",
        "Include_test=0",
        "Include_launcher=0",
        "AssociateFiles=0",
        "Include_pip=1"
    )
    Write-Step "Running Python silent installer (may take 1-2 minutes)..."
    $p = Start-Process -FilePath $installer -ArgumentList $args -Wait -PassThru
    if ($p.ExitCode -ne 0 -and $p.ExitCode -ne $null) {
        # some installers return 0 only; continue to verify
        Write-Step ("installer exit code: " + $p.ExitCode)
    }

    if (-not (Test-PythonOk $LocalPy)) {
        Write-Host "ERROR: Python install failed (tools\python\python.exe not usable)" -ForegroundColor Red
        Write-Host "Try running the installer manually:" -ForegroundColor Yellow
        Write-Host $installer
        exit 1
    }
    Write-Step ("Python installed: " + $LocalPy)
    return $LocalPy
}

function Ensure-OCR {
    if ((Test-Path -LiteralPath $VenvPy)) {
        & $VenvPy -c "import faster_paddle" 2>$null
        if ($LASTEXITCODE -eq 0) {
            Write-Step ("OCR OK: " + $VenvPy)
            return
        }
    }

    $basePy = Ensure-Python
    Write-Step "Creating .venv-fpaddle and installing faster-paddle ..."
    $venvDir = Join-Path $Root ".venv-fpaddle"
    if (Test-Path $venvDir) { Remove-Item $venvDir -Recurse -Force }
    & $basePy -m venv $venvDir
    if ($LASTEXITCODE -ne 0) {
        Write-Host "ERROR: venv create failed" -ForegroundColor Red
        exit 1
    }
    $pip = Join-Path $venvDir "Scripts\python.exe"
    & $pip -m pip install -U pip -q
    & $pip -m pip install faster-paddle -i $PipMirror --trusted-host pypi.tuna.tsinghua.edu.cn
    if ($LASTEXITCODE -ne 0) {
        Write-Step "Mirror failed; trying official PyPI ..."
        & $pip -m pip install faster-paddle
    }
    if ($LASTEXITCODE -ne 0) {
        Write-Host "ERROR: pip install faster-paddle failed" -ForegroundColor Red
        exit 1
    }
    & $pip -c "import faster_paddle"
    if ($LASTEXITCODE -ne 0) {
        Write-Host "ERROR: import faster_paddle failed" -ForegroundColor Red
        exit 1
    }
    Write-Step "OCR installed"
}

function Test-OarOk([string]$Bin) {
    if (-not (Test-Path -LiteralPath $Bin)) { return $false }
    try {
        $out = & $Bin --version 2>&1 | Out-String
        return ($LASTEXITCODE -eq 0 -and $out -match "oar")
    } catch {
        return $false
    }
}

function Find-SystemOar {
    $cands = @()
    $cmd = Get-Command oar -EA SilentlyContinue
    if ($cmd) { $cands += $cmd.Source }
    $cargoOar = Join-Path $env:USERPROFILE ".cargo\bin\oar.exe"
    if (Test-Path -LiteralPath $cargoOar) { $cands += $cargoOar }
    foreach ($p in $cands) {
        if (Test-OarOk $p) { return $p }
    }
    return $null
}

function Ensure-Oar {
    if (Test-OarOk $OarExe) {
        Write-Step ("oar OK: " + $OarExe)
        return
    }

    New-Item -ItemType Directory -Force -Path (Join-Path $Root "tools") | Out-Null

    $src = Find-SystemOar
    if ($src) {
        Write-Step ("Copying oar -> tools\oar.exe from " + $src)
        Copy-Item -LiteralPath $src -Destination $OarExe -Force
        if (Test-OarOk $OarExe) {
            Write-Step "oar installed (copied)"
            return
        }
    }

    if ($OarZipUrl) {
        Write-Step ("Downloading oar zip: " + $OarZipUrl)
        $tmpZip = Join-Path $env:TEMP "doc2md-oar.zip"
        $tmpDir = Join-Path $env:TEMP "doc2md-oar-extract"
        if (Test-Path $tmpDir) { Remove-Item $tmpDir -Recurse -Force }
        New-Item -ItemType Directory -Force -Path $tmpDir | Out-Null
        Download-File @($OarZipUrl) $tmpZip
        Expand-Archive -LiteralPath $tmpZip -DestinationPath $tmpDir -Force
        $found = Get-ChildItem -Path $tmpDir -Recurse -Filter "oar.exe" -EA SilentlyContinue |
            Select-Object -First 1
        if (-not $found) {
            Write-Host "ERROR: oar.exe not found in zip" -ForegroundColor Red
            exit 1
        }
        Copy-Item -LiteralPath $found.FullName -Destination $OarExe -Force
        Remove-Item $tmpZip -Force -EA SilentlyContinue
        Remove-Item $tmpDir -Recurse -Force -EA SilentlyContinue
        if (Test-OarOk $OarExe) {
            Write-Step "oar installed (zip)"
            return
        }
    }

    $cargo = Get-Command cargo -EA SilentlyContinue
    if ($cargo) {
        Write-Step "cargo install oar-ocr-cli (first time may take several minutes)..."
        $cargoRoot = Join-Path $Root "tools\oar-cargo"
        if (Test-Path $cargoRoot) { Remove-Item $cargoRoot -Recurse -Force }
        & cargo install oar-ocr-cli --locked --root $cargoRoot
        if ($LASTEXITCODE -ne 0) {
            Write-Step "cargo --locked failed; retry without --locked ..."
            & cargo install oar-ocr-cli --root $cargoRoot
        }
        $built = Join-Path $cargoRoot "bin\oar.exe"
        if (Test-OarOk $built) {
            Copy-Item -LiteralPath $built -Destination $OarExe -Force
            Write-Step "oar installed (cargo)"
            return
        }
    }

    Write-Host "ERROR: oar (表征引擎) missing." -ForegroundColor Red
    Write-Host "Fix one of:" -ForegroundColor Yellow
    Write-Host "  1) Copy oar.exe into tools\oar.exe (from a machine that already has it)"
    Write-Host "  2) Install Rust, then re-run start.bat (will cargo install oar-ocr-cli)"
    Write-Host "  3) Set DOC2MD_OAR_ZIP_URL to a zip containing oar.exe, then re-run"
    exit 1
}

function Ensure-StructureWorker {
    $dst = Join-Path $Root "tools\oar_structure_worker.exe"
    $proj = Join-Path $Root "tools\oar_structure_worker"
    if (-not (Test-Path (Join-Path $proj "Cargo.toml"))) {
        Write-Step "oar_structure_worker sources missing; skip (CLI fallback)"
        return
    }
    if (Test-Path $dst) {
        Write-Step "oar_structure_worker present"
        return
    }
    $cargo = Get-Command cargo -EA SilentlyContinue
    if (-not $cargo) {
        Write-Step "cargo missing; skip oar_structure_worker (CLI fallback)"
        return
    }
    Write-Step "building oar_structure_worker (first time may take several minutes)..."
    Push-Location $proj
    try {
        & cargo build --release
        if ($LASTEXITCODE -ne 0) { throw "cargo build failed" }
    } finally {
        Pop-Location
    }
    $built = Join-Path $proj "target\release\oar_structure_worker.exe"
    if (-not (Test-Path $built)) {
        $built = Join-Path $proj "target\release\oar_structure_worker"
    }
    if (Test-Path $built) {
        Copy-Item $built $dst -Force
        Write-Step "oar_structure_worker installed"
    } else {
        Write-Step "oar_structure_worker build produced no binary; CLI fallback"
    }
}

Ensure-Poppler
Ensure-OCR
Ensure-Oar
Ensure-StructureWorker
Write-Step "deps ready"
exit 0
