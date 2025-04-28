@echo off
TITLE Go File Processor - Enhanced Windows Tool
setlocal enabledelayedexpansion

:: Set colors for better readability
set "BLUE=[94m"
set "GREEN=[92m"
set "RED=[91m"
set "YELLOW=[93m"
set "NC=[0m"

echo %BLUE%============================================%NC%
echo %BLUE%    Go File Processor - Windows Toolkit     %NC%
echo %BLUE%============================================%NC%
echo.

:Menu
echo %YELLOW%Select an option:%NC%
echo 1) Local development (build and run)
echo 2) Build for Linux deployment
echo 3) Run diagnostics
echo q) Quit
echo.

set /p CHOICE=Enter your choice (1, 2, 3, q): 
if "%CHOICE%"=="1" goto LocalDev
if "%CHOICE%"=="2" goto LinuxBuild
if "%CHOICE%"=="3" goto Diagnostics
if /i "%CHOICE%"=="q" goto End
echo %RED%Invalid option. Please try again.%NC%
echo.
goto Menu

:CheckAdmin
echo Checking for administrative privileges...
net session >nul 2>&1
if %errorLevel% == 0 (
    echo %GREEN%√ Running with administrator privileges.%NC%
) else (
    echo %YELLOW%⚠ Not running with administrator privileges!%NC%
    echo Some features may not work properly.
    echo Consider right-clicking this file and selecting "Run as administrator".
    echo.
    pause
)
goto :eof

:CheckGo
echo %YELLOW%[1] Checking for Go installation...%NC%
where go >nul 2>&1
if %errorLevel% == 0 (
    for /f "tokens=3" %%i in ('go version') do set GO_VERSION=%%i
    echo %GREEN%√ Go %GO_VERSION% is installed.%NC%
) else (
    echo %RED%× Go is not installed.%NC%
    echo Please install Go from https://golang.org/dl/
    echo and add it to your PATH.
    pause
    exit /b 1
)
goto :eof

:LocalDev
call :CheckAdmin
call :CheckGo

echo %YELLOW%[2] Building the application...%NC%
go build -o fileprocessor.exe cmd\server\main.go
if %errorLevel% neq 0 (
    echo %RED%× Error building the application.%NC%
    echo Check for any compilation errors above.
    pause
    goto Menu
) else (
    echo %GREEN%√ Application built successfully.%NC%
)

echo %YELLOW%[3] Creating required directories...%NC%
if not exist ".\uploads" mkdir uploads
if not exist ".\logs" mkdir logs
echo %GREEN%√ Directories created.%NC%

echo %YELLOW%[4] Starting the file server...%NC%
echo %GREEN%Application starting - Access at http://localhost:8080%NC%
echo Press Ctrl+C to stop the server.
echo.
start "File Processor" cmd /c "fileprocessor.exe --port=8080 --ui=./ui --uploads=./uploads"

echo %YELLOW%[5] Testing connectivity...%NC%
timeout /t 3 > nul
curl -s -o nul -w "HTTP Status: %%{http_code}" http://localhost:8080
if %errorLevel% neq 0 (
    echo %RED%× Failed to connect to application.%NC%
) else (
    echo %GREEN%√ Application is running and accessible.%NC%
)

echo.
echo %BLUE%============================================%NC%
echo %BLUE%    Application started successfully!       %NC%
echo %BLUE%============================================%NC%
echo.
echo Access the application at: http://localhost:8080
echo.
pause
goto Menu

:LinuxBuild
call :CheckGo

echo %YELLOW%[1] Building application for Linux deployment...%NC%
set GOOS=linux
set GOARCH=amd64
go build -o fileprocessor cmd\server\main.go
if %errorLevel% neq 0 (
    echo %RED%× Error building the application for Linux.%NC%
    echo Check for any compilation errors above.
    pause
    goto Menu
) else (
    echo %GREEN%√ Linux binary built successfully as 'fileprocessor'.%NC%
)

echo.
echo %BLUE%============================================%NC%
echo %BLUE%    Linux Build Complete                    %NC%
echo %BLUE%============================================%NC%
echo.
echo %YELLOW%The Linux binary 'fileprocessor' has been created.%NC%
echo You can now:
echo 1. Copy this binary to your Linux server
echo 2. Create the necessary directories (uploads, logs, etc.)
echo 3. Set up a systemd service for the application
echo.
pause
goto Menu

:Diagnostics
call :CheckAdmin

echo %YELLOW%[1] Running network diagnostics...%NC%
echo.
echo Checking network interfaces:
ipconfig | findstr IPv4
echo.

echo Checking listening ports:
netstat -ano | findstr :8080
if %errorLevel% neq 0 (
    echo %RED%× Port 8080 is not in use.%NC%
) else (
    echo %GREEN%√ Port 8080 is in use.%NC%
)

echo.
echo Checking HTTP connectivity:
curl -s -o nul -w "HTTP Status: %%{http_code}" http://localhost:8080
if %errorLevel% neq 0 (
    echo %RED%× Failed to connect to application on port 8080.%NC%
) else (
    echo %GREEN%√ Application is accessible on port 8080.%NC%
)

echo.
echo %BLUE%============================================%NC%
echo %BLUE%    Diagnostics Complete                    %NC%
echo %BLUE%============================================%NC%
echo.
pause
goto Menu

:End
echo.
echo %BLUE%Exiting Go File Processor toolkit.%NC%
echo.
endlocal
exit /b 0