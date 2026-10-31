@echo off
setlocal
rem Run this script from the directory containing llama-server.exe.
set "MODEL=D:\models\gemma-4-12b-Q4_K_M.gguf"
set "CTX=16384"
set "NGL=999"

if not exist ".\llama-server.exe" (
  echo ERROR: llama-server.exe was not found in %CD%
  pause
  exit /b 1
)
if not exist "%MODEL%" (
  echo ERROR: model was not found at %MODEL%
  pause
  exit /b 1
)

echo Starting llama-server...
echo MODEL=%MODEL%
echo CTX=%CTX% NGL=%NGL% PORT=8080
echo.

".\llama-server.exe" ^
  -m "%MODEL%" ^
  -c %CTX% ^
  -ngl %NGL% ^
  --jinja ^
  --host 127.0.0.1 ^
  --port 8080 ^
  --temp 1.0 --top-p 0.95 --top-k 64

pause
