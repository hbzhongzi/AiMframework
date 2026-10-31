@echo off
setlocal
echo ==== Gemma 4 12B environment check ====
echo.
echo [1/4] NVIDIA GPU
where nvidia-smi >nul 2>&1
if errorlevel 1 (
  echo ERROR: nvidia-smi was not found.
) else (
  nvidia-smi --query-gpu=name,driver_version,memory.total --format=csv,noheader
)
echo.
echo [2/4] llama.cpp and model
if exist "D:\llama-cpp\llama-server.exe" (
  echo OK: D:\llama-cpp\llama-server.exe
) else (
  echo MISSING: D:\llama-cpp\llama-server.exe
)
if exist "D:\models\gemma-4-12b-Q4_K_M.gguf" (
  echo OK: D:\models\gemma-4-12b-Q4_K_M.gguf
) else (
  echo MISSING: D:\models\gemma-4-12b-Q4_K_M.gguf
)
echo.
echo [3/4] Free space on D:
if exist "D:\" (
  powershell -NoProfile -Command "$d=Get-PSDrive -Name D; $d.Free/1GB"
) else (
  echo D: drive was not found.
)
echo.
echo [4/4] Ports 8080 and 8081
netstat -ano | findstr "LISTENING" | findstr /R /C:":8080 " /C:":8081 " >nul
if errorlevel 1 (
  echo OK: ports are free.
) else (
  echo WARNING: one or both ports are already in use.
  netstat -ano | findstr "LISTENING" | findstr /R /C:":8080 " /C:":8081 "
)
echo.
echo Check complete.
pause
