@echo off
cd /d "%~dp0"

echo Stopping doc2md and OCR workers...

taskkill /F /IM doc2md.exe >nul 2>&1

powershell -NoProfile -Command ^
  "Get-CimInstance Win32_Process -Filter \"Name='python.exe'\" | Where-Object { $_.CommandLine -match 'ocr_cli|faster_paddle' } | ForEach-Object { Stop-Process -Id $_.ProcessId -Force -ErrorAction SilentlyContinue }"

echo Done.
timeout /t 2 >nul