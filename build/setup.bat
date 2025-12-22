@echo off
setlocal
cd /d "%~dp0"
cd ..

echo [SETUP] Setting up Monitoring Infrastructure...

if exist "monitoring\prometheus.exe" (
    echo [OK] Prometheus is already installed.
    goto :end
)

echo [DOWNLOADING] Prometheus...
powershell -Command "Invoke-WebRequest -Uri 'https://github.com/prometheus/prometheus/releases/download/v2.45.0/prometheus-2.45.0.windows-amd64.zip' -OutFile 'monitoring\prometheus.zip'"

echo [EXTRACTING]...
powershell -Command "Expand-Archive -Path 'monitoring\prometheus.zip' -DestinationPath 'monitoring\temp' -Force"

echo [INSTALLING] Moving files...
copy "monitoring\temp\prometheus-2.45.0.windows-amd64\prometheus.exe" "monitoring\prometheus.exe"
del "monitoring\prometheus.zip"
rmdir /s /q "monitoring\temp"

echo [SUCCESS] Prometheus installed to monitoring\prometheus.exe

:end
echo.
echo Setup complete. Now run build\start.bat
pause
