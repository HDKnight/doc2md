# Quick oar structure wall-clock + keyword gate.
param(
  [string]$Pdf = "$env:TEMP\doc2md-1531639136.pdf",
  [string]$OutRoot = ""
)
$ErrorActionPreference = "Stop"
if (-not (Test-Path $Pdf)) { throw "PDF missing: $Pdf" }
if (-not $OutRoot) {
  $OutRoot = Join-Path (Split-Path $PSScriptRoot -Parent) "testdata\out\oar_speed_bench"
}
New-Item -ItemType Directory -Force -Path $OutRoot | Out-Null

$oar = (Get-Command oar -ErrorAction Stop).Source
$dpi = 96
$keyword = [char]0x674E + [char]0x5927 + [char]0x9489  # 李大钊

function Invoke-Oar {
  param([string]$Name, [string]$Device, [int]$OrtThreads)
  $dir = Join-Path $OutRoot $Name
  if (Test-Path $dir) { Remove-Item -Recurse -Force $dir }
  New-Item -ItemType Directory -Force -Path $dir | Out-Null
  $env:OMP_NUM_THREADS = [string]$OrtThreads
  $env:ORT_INTRA_OP_NUM_THREADS = [string]$OrtThreads
  $env:ORT_INTER_OP_NUM_THREADS = "1"
  $sw = [Diagnostics.Stopwatch]::StartNew()
  & $oar structure $Pdf --dpi $dpi --format json -o $dir --device $Device 2>&1 | Out-Null
  if ($LASTEXITCODE -ne 0) { throw "oar failed ($Name) exit=$LASTEXITCODE" }
  $sw.Stop()
  $ms = [int]$sw.Elapsed.TotalMilliseconds
  $text = (Get-ChildItem $dir -Filter *.json | ForEach-Object { Get-Content $_.FullName -Raw -Encoding UTF8 }) -join "`n"
  $hits = ([regex]::Matches($text, [regex]::Escape($keyword))).Count
  [pscustomobject]@{ name = $Name; ms = $ms; keyword = $hits; device = $Device; ort = $OrtThreads }
}

$ncpu = [Environment]::ProcessorCount
$reps = [Math]::Max(1, [Math]::Floor($ncpu / 4))
$cpuDev = (1..$reps | ForEach-Object { "cpu" }) -join ","
$ort = [Math]::Max(1, [Math]::Min(8, [Math]::Floor($ncpu / $reps)))

Write-Host "PDF=$Pdf ncpu=$ncpu reps=$reps ort=$ort"
$results = @()
$results += Invoke-Oar -Name "baseline_cpu" -Device $cpuDev -OrtThreads $ort
$ort2 = [Math]::Max(1, [Math]::Floor($ort / 2))
if ($ort2 -ne $ort) {
  $results += Invoke-Oar -Name "ort_half" -Device $cpuDev -OrtThreads $ort2
}
if ($reps -gt 1) {
  $r2 = $reps - 1
  $dev2 = (1..$r2 | ForEach-Object { "cpu" }) -join ","
  $ortR = [Math]::Max(1, [Math]::Min(8, [Math]::Floor($ncpu / $r2)))
  $results += Invoke-Oar -Name ("reps_" + $r2) -Device $dev2 -OrtThreads $ortR
}

$results | Format-Table -AutoSize
$base = $results | Where-Object { $_.name -eq "baseline_cpu" } | Select-Object -First 1
foreach ($r in $results) {
  $okSp = $r.ms -le 10000
  Write-Host ("{0}: {1}ms kw={2} base_kw={3} under_10s={4}" -f $r.name, $r.ms, $r.keyword, $base.keyword, $okSp)
}
