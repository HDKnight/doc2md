@echo off
setlocal
cd /d "%~dp0"

rem usage: start.bat       -> port 8484
rem        start.bat 9000  -> custom port
set "PORT=%~1"
if "%PORT%"=="" set "PORT=8484"
set "DOC2MD_ADDR=:%PORT%"

if not exist "doc2md.exe" (
  echo [ERROR] doc2md.exe not found in this folder
  echo Run: go build -o doc2md.exe ./cmd/server
  pause
  exit /b 1
)

if exist "%~dp0.portable" (
  echo ========================================
  echo   Portable mode: deps already bundled
  echo ========================================
) else (
  echo ========================================
  echo   Dependency check ^(first run may download^)
  echo   Python  -^> tools\python
  echo   Poppler -^> tools\poppler
  echo   OCR     -^> .venv-fpaddle
  echo   oar     -^> tools\oar.exe
  echo ========================================
  echo.
  powershell -NoProfile -ExecutionPolicy Bypass -File "%~dp0ensure-deps.ps1"
  if errorlevel 1 (
    echo.
    echo [ERROR] Dependency setup failed.
    pause
    exit /b 1
  )
)

set "PATH=%~dp0tools\poppler\Library\bin;%~dp0tools\python;%~dp0tools\python\Scripts;%~dp0tools;%PATH%"
set "DOC2MD_PDFTOPPM=%~dp0tools\poppler\Library\bin\pdftoppm.exe"
set "DOC2MD_PDFTOTEXT=%~dp0tools\poppler\Library\bin\pdftotext.exe"

if exist "%~dp0.portable" (
  set "DOC2MD_OCR_PYTHON=%~dp0tools\python\python.exe"
) else if exist "%~dp0.venv-fpaddle\Scripts\python.exe" (
  set "DOC2MD_OCR_PYTHON=%~dp0.venv-fpaddle\Scripts\python.exe"
) else (
  set "DOC2MD_OCR_PYTHON=%~dp0tools\python\python.exe"
)

if exist "%~dp0tools\oar.exe" set "DOC2MD_OCR_OAR=%~dp0tools\oar.exe"
if exist "%~dp0tools\oar-home\" set "OAR_HOME=%~dp0tools\oar-home"

if "%DOC2MD_OAR_STRUCTURE_DPI%"=="" set "DOC2MD_OAR_STRUCTURE_DPI=96"
if "%DOC2MD_OAR_DEVICE%"=="" set "DOC2MD_OAR_DEVICE=auto"

echo.
echo ========================================
echo   doc2md starting
echo   URL: http://127.0.0.1:%PORT%/
echo   oar: dpi=%DOC2MD_OAR_STRUCTURE_DPI% device=%DOC2MD_OAR_DEVICE%
echo   Stop: close this window, or run stop.bat
echo ========================================
echo.

start "" "http://127.0.0.1:%PORT%/"
doc2md.exe
echo.
echo Server exited.
pause