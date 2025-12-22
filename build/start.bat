@echo off
setlocal
title AIS-1C Integration Service
cd /d "%~dp0"

:: 1. Проверка .env
if not exist ".env" (
    echo [ERROR] .env file not found! 
    echo Please configure .env based on .env.example
    pause
    exit /b
)

:: 2. Запуск мониторинга (если установлен)
echo [INFO] Starting Infrastructure...

:: Prometheus
if exist "monitoring\prometheus.exe" (
    start "Prometheus" /min monitoring\prometheus.exe --config.file=monitoring\prometheus.yml
)

:: Grafana
if exist "monitoring\grafana\bin\grafana-server.exe" (
    pushd monitoring\grafana\bin
    start "Grafana" /min grafana-server.exe
    popd
)

:run
cls
echo =====================================================
echo   🚀 AIS-1C INTEGRATION SERVICE IS RUNNING
echo =====================================================
echo   ➜ API:      http://127.0.0.1:8081/api/v1/data
echo   ➜ Admin UI: http://127.0.0.1:8081/_/
echo   ➜ Swagger:  http://127.0.0.1:8081/swagger/index.html
echo.
if exist "monitoring\grafana" (
    echo   📊 MONITORING:
    echo   ➜ Grafana:    http://localhost:3000 (admin/admin)
)
echo =====================================================
echo.

if exist "server.exe" (
    server.exe serve --http="0.0.0.0:8081"
) else (
    echo [ERROR] server.exe not found! Please build the project.
    echo Use: go build -o build/server.exe ./cmd/api/main.go
    pause
    exit /b
)

color 4F
echo.
echo [WARN] Server process terminated. Restarting in 5s...
timeout /t 5
color 07
goto run