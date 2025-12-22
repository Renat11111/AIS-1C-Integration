@echo off
cd /d "%~dp0\.."

echo [BUILD] Getting git version...
for /f %%i in ('git describe --tags --always --dirty') do set VERSION=%%i
for /f %%i in ('git rev-parse --short HEAD') do set COMMIT=%%i
for /f "tokens=2 delims==" %%I in ('wmic os get localdatetime /value') do set DT=%%I
set BUILD_TIME=%DT:~0,4%-%DT:~4,2%-%DT:~6,2% %DT:~8,2%:%DT:~10,2%

echo [BUILD] Compiling %VERSION% (%COMMIT%)...
go build -ldflags "-X 'ais-1c-proxy/internal/config.Version=%VERSION%' -X 'ais-1c-proxy/internal/config.CommitSHA=%COMMIT%' -X 'ais-1c-proxy/internal/config.BuildTime=%BUILD_TIME%'" -o build/server.exe ./cmd/api/main.go

if %ERRORLEVEL% EQU 0 (
    echo [SUCCESS] Build complete: build/server.exe
) else (
    echo [ERROR] Build failed!
)
pause
