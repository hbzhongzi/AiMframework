@echo off
setlocal
set "ROOT=%~dp0..\"
if not defined LLAMA_CPP_DIR set "LLAMA_CPP_DIR=D:\llama-cpp"

echo ==== Gemma 4 12B desktop startup ====
if not exist "%ROOT%dist\gemma-desktop.exe" (
  echo ERROR: desktop launcher not found: %ROOT%dist\gemma-desktop.exe
  pause
  exit /b 1
)
if not exist "%LLAMA_CPP_DIR%\llama-server.exe" (
  echo ERROR: llama-server.exe not found: %LLAMA_CPP_DIR%\llama-server.exe
  pause
  exit /b 1
)
if not exist "D:\models\gemma-4-12b-Q4_K_M.gguf" (
  echo ERROR: model not found: D:\models\gemma-4-12b-Q4_K_M.gguf
  pause
  exit /b 1
)

start "Gemma-Desktop" /D "%ROOT%" "%ROOT%dist\gemma-desktop.exe"
echo Desktop launcher started.
echo The launcher owns backend + model lifecycle and opens the admin window.
pause
