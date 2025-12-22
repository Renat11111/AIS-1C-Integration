@echo off
setlocal
title AIS-1C Integration Service [Supervisor]
cd /d "%~dp0"

:check_env
if not exist "..\monitoring\prometheus.exe" (
    color 6F
    echo [WARN] Prometheus not found!
    echo Please run 'setup.bat' first to install monitoring tools.
    pause
    exit /b
)

:build
echo [INFO] Checking integrity and building...
cd ..
go build -o build/server.exe ./cmd/api
cd build
if %ERRORLEVEL% NEQ 0 (
    color 4F
    echo [ERROR] Build failed! Fix code errors.
    echo Waiting 10 seconds to retry...
    timeout /t 10
    goto build
)

:run_monitor
echo [INFO] Starting Prometheus...
start "Prometheus Metrics" /min ..\monitoring\prometheus.exe --config.file=..\monitoring\prometheus.yml

:run_server
color 0A
cls
echo =====================================================
echo   AIS-1C Integration Service (PocketBase + 1C)
echo =====================================================
echo   [API]      http://localhost:8081/api/v1/data
echo   [UI]       http://localhost:8081/_/
echo   [DOCS]     http://localhost:8081/swagger/index.html
echo   [METRICS]  http://localhost:8081/metrics
echo   [PROMETHEUS] http://localhost:9090
echo.
echo   [LOGS]     See app.log for details
echo =====================================================
echo.

:: Run Server
server.exe serve --http="0.0.0.0:8081"

:: Crash Handler
color 6F
echo.
echo [WARN] Server process terminated.
echo [WARN] Restarting in 5 seconds...
timeout /t 5
goto run_server
