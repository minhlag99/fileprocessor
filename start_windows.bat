@echo off
TITLE Go File Processor Startup

echo ----------------------------------------------
echo       Go File Processor Startup Script
echo ----------------------------------------------
echo.

:: Check for administrative privileges
net session >nul 2>&1
if %errorLevel% == 0 (
    echo Running with administrator privileges.
) else (
    echo WARNING: Not running with administrator privileges!
    echo Some features may not work properly.
    echo Right-click this file and select "Run as administrator".
    echo.
    pause
    goto :CheckGo
)

:CheckGo
echo Checking for Go installation...
where go >nul 2>&1
if %errorLevel% == 0 (
    for /f "tokens=3" %%i in ('go version') do set GO_VERSION=%%i
    echo Go %GO_VERSION% is installed.
) else (
    echo Go is not installed.
    echo Please install Go from https://golang.org/dl/
    echo and add it to your PATH.
    pause
    exit /b 1
)

:Build
echo.
echo Building the application...
go build -o fileserver.exe cmd\server\main.go
if %errorLevel% neq 0 (
    echo Error building the application.
    pause
    exit /b 1
)

:Run
echo.
echo Starting the file server...
echo Access the application at http://localhost:8080
echo Press Ctrl+C to stop the server.
echo.
fileserver.exe --port=8080 --ui=./ui --uploads=./uploads

pause