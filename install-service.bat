@echo off
setlocal

net session >nul 2>&1
if not "%errorlevel%"=="0" (
  powershell -NoProfile -ExecutionPolicy Bypass -Command "Start-Process -FilePath '%~f0' -Verb RunAs"
  exit /b
)

set "APP_DIR=%~dp0"
set "APP_EXE=%APP_DIR%printerService.exe"

if not exist "%APP_EXE%" (
  echo printerService.exe not found: "%APP_EXE%"
  pause
  exit /b 1
)

"%APP_EXE%" install
if errorlevel 1 (
  pause
  exit /b 1
)

sc.exe config printerService start= auto
if errorlevel 1 (
  pause
  exit /b 1
)

"%APP_EXE%" start
if errorlevel 1 (
  pause
  exit /b 1
)

echo printerService installed, set to auto start, and started.
pause
