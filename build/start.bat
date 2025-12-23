@echo off
setlocal
cd /d "%~dp0"
title AIS-1C Integration Service

:: Check .env
if not exist ".env" (
    echo [ERROR] .env file not found! 
    echo Please configure .env based on .env.example
    pause
    exit /b
)

:: Start Monitoring
echo [INFO] Starting Infrastructure...

if exist "monitoring\prometheus.exe" (
    start "Prometheus" /min monitoring\prometheus.exe --config.file=monitoring\prometheus.yml
)

if exist "monitoring\grafana\bin\grafana-server.exe" (
    pushd monitoring\grafana\bin
    start "Grafana" /min grafana-server.exe
    popd
)

:run
cls
echo =====================================================
echo   AIS-1C INTEGRATION SERVICE IS RUNNING
echo =====================================================
echo   API:      http://127.0.0.1:8081/api/v1/data
echo   Health:   http://127.0.0.1:8081/api/v1/health
echo   Admin UI: http://127.0.0.1:8081/_/
echo   Swagger:  http://127.0.0.1:8081/swagger/index.html
echo.
if exist "monitoring\grafana" (
    echo   MONITORING:
    echo   Grafana:    http://localhost:3000 (admin/admin)
    echo   Prometheus: http://localhost:9090
)
echo =====================================================
echo.

if exist "server.exe" (
    server.exe serve --http="127.0.0.1:8081"
) else (
    echo [ERROR] server.exe not found! Please build the project.
    echo Use: go build -o build/server.exe ./cmd/api/main.go
    pause
    exit /b
)

echo.
echo [WARN] Server process terminated. Restarting in 5s...
timeout /t 5
goto run
