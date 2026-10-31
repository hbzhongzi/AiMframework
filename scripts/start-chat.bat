@echo off
setlocal
set "ROOT=%~dp0..\"
if not exist "%ROOT%dist\gemma-chat.exe" (
  echo ERROR: chat launcher not found: %ROOT%dist\gemma-chat.exe
  echo Run scripts\build-desktop.ps1 first.
  pause
  exit /b 1
)
start "Gemma-Chat" /D "%ROOT%" "%ROOT%dist\gemma-chat.exe"
