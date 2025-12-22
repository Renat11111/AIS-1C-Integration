@echo off
setlocal
cd /d "%~dp0"

echo [SETUP] AIS-1C Integration Infrastructure...
echo.

:: 1. Создаем локальную папку мониторинга
if not exist "monitoring" mkdir monitoring

:: --- PROMETHEUS ---
if exist "monitoring\prometheus.exe" (
    echo [OK] Prometheus already installed.
) else (
    echo [DOWNLOADING] Prometheus...
    powershell -Command "Invoke-WebRequest -Uri 'https://github.com/prometheus/prometheus/releases/download/v2.45.0/prometheus-2.45.0.windows-amd64.zip' -OutFile 'monitoring\prometheus.zip'"
    echo [EXTRACTING] Prometheus...
    powershell -Command "Expand-Archive -Path 'monitoring\prometheus.zip' -DestinationPath 'monitoring\temp' -Force"
    copy "monitoring\temp\prometheus-2.45.0.windows-amd64\prometheus.exe" "monitoring\prometheus.exe" >nul
    del "monitoring\prometheus.zip"
    rmdir /s /q "monitoring\temp"
)

:: Конфиг Prometheus
(
echo global:
echo   scrape_interval: 2s
echo scrape_configs:
echo   - job_name: 'ais_1c_proxy'
echo     static_configs:
echo       - targets: ['127.0.0.1:8081']
) > monitoring\prometheus.yml

:: --- GRAFANA ---
if exist "monitoring\grafana\bin\grafana-server.exe" (
    echo [OK] Grafana already installed.
) else (
    echo [DOWNLOADING] Grafana (Portable)...
    powershell -Command "Invoke-WebRequest -Uri 'https://dl.grafana.com/oss/release/grafana-10.0.3.windows-amd64.zip' -OutFile 'monitoring\grafana.zip'"
    echo [EXTRACTING] Grafana...
    powershell -Command "Expand-Archive -Path 'monitoring\grafana.zip' -DestinationPath 'monitoring\temp_grafana' -Force"
    mkdir monitoring\grafana
    xcopy /E /I /Q "monitoring\temp_grafana\grafana-10.0.3" "monitoring\grafana" >nul
    del "monitoring\grafana.zip"
    rmdir /s /q "monitoring\temp_grafana"
)

:: --- НАСТРОЙКИ GRAFANA (Provisioning) ---
mkdir monitoring\grafana_provisioning\datasources -Force
mkdir monitoring\grafana_provisioning\dashboards -Force
mkdir monitoring\dashboards -Force

:: Datasource
(
echo apiVersion: 1
echo datasources:
echo   - name: Prometheus
echo     type: prometheus
echo     url: http://localhost:9090
echo     isDefault: true
) > monitoring\grafana_provisioning\datasources\all.yml

:: Dashboard Provider
(
echo apiVersion: 1
echo providers:
echo   - name: 'AIS Dashboards'
echo     type: file
echo     options:
echo       path: ..\..\dashboards
) > monitoring\grafana_provisioning\dashboards\all.yml

:: Dashboard JSON
(
echo {
echo   "panels": [
echo     { "title": "Очередь", "type": "stat", "gridPos": {"h":8,"w":8}, "targets": [{"expr": "ais_queue_depth"}] },
echo     { "title": "Запросы/сек", "type": "timeseries", "gridPos": {"h":8,"w":16,"x":8}, "targets": [{"expr": "rate(ais_processed_total[1m])"}] }
echo   ],
echo   "title": "AIS Monitor",
echo   "uid": "ais_main"
echo }
) > monitoring\dashboards\dashboard.json

echo.
echo [SUCCESS] All systems ready! Run start.bat now.
pause
