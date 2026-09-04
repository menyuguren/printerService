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

"%APP_EXE%" stop

"%APP_EXE%" uninstall
if errorlevel 1 (
  pause
  exit /b 1
)

echo printerService stopped and uninstalled.
pause
