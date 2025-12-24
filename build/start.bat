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

:: Load variables from .env
for /f "usebackq tokens=1,2 delims==" %%a in (".env") do (
    if "%%a"=="SERVER_HOST" set SERVER_HOST=%%b
    if "%%a"=="SERVER_PORT" set SERVER_PORT=%%b
)

:: Set defaults if not found in .env
if "%SERVER_HOST%"=="" set SERVER_HOST=127.0.0.1
if "%SERVER_PORT%"=="" set SERVER_PORT=8081

:: Start Monitoring
...
:run
cls
echo =====================================================
echo   AIS-1C INTEGRATION SERVICE IS RUNNING
echo =====================================================
echo   API:      http://%SERVER_HOST%:%SERVER_PORT%/api/v1/data
echo   Health:   http://%SERVER_HOST%:%SERVER_PORT%/api/v1/health
echo   Admin UI: http://%SERVER_HOST%:%SERVER_PORT%/_/
echo   Swagger:  http://%SERVER_HOST%:%SERVER_PORT%/swagger/index.html
echo.
if exist "monitoring\grafana" (
    echo   MONITORING:
    echo   Grafana:    http://localhost:3000 (admin/admin)
    echo   Prometheus: http://localhost:9090
)
echo =====================================================
echo.

if exist "server.exe" (
    server.exe serve --http="%SERVER_HOST%:%SERVER_PORT%"
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
