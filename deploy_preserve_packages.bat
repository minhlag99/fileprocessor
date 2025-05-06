@echo off
setlocal enabledelayedexpansion

:: File Processor Deployment Script with Package Preservation (Windows)
:: This script deploys the application while preserving Go modules cache

:: Color codes for better output
set "GREEN=[92m"
set "YELLOW=[93m"
set "RED=[91m"
set "NC=[0m"

:: Application variables
set "APP_NAME=fileprocessor"
set "APP_DIR=%CD%"
set "PORT=8080"
set "CONFIG_FILE=config\fileprocessor.json"

:: Print the banner
echo %YELLOW%
echo ╔════════════════════════════════════════════════════════╗
echo ║       File Processor Deployment (Package Saving)       ║
echo ╚════════════════════════════════════════════════════════╝
echo %NC%

:: Check Go installation
echo %YELLOW%[1] Checking Go installation...%NC%
where go >nul 2>nul
if %ERRORLEVEL% neq 0 (
    echo    %RED%✗%NC% Go is not installed
    echo    Please install Go before continuing
    exit /b 1
)

for /f "tokens=3" %%i in ('go version') do set "GO_VERSION=%%i"
echo    Go !GO_VERSION! detected
echo    %GREEN%✓%NC% Go is installed

:: Display GOPATH information
for /f "tokens=*" %%i in ('go env GOPATH') do set "GOPATH=%%i"
echo    GOPATH: !GOPATH!
for /f "tokens=*" %%i in ('go env GOMODCACHE') do set "GOMODCACHE=%%i"
echo    Go module cache: !GOMODCACHE!

:: Check import paths
echo %YELLOW%[2] Checking import paths...%NC%
for /f "tokens=2" %%i in ('findstr /R "^module " go.mod') do set "MODULE_NAME=%%i"
echo    Module name from go.mod: %GREEN%!MODULE_NAME!%NC%

:: Check main.go imports
findstr /C:"\"!MODULE_NAME!/internal/" cmd\server\main.go >nul 2>nul
if %ERRORLEVEL% equ 0 (
    echo    %GREEN%✓%NC% Main.go imports match module name
) else (
    echo    %RED%✗%NC% Import paths in main.go may not match module name
    echo    Imports in main.go should start with: "!MODULE_NAME!/..."
)

:: Check file_handlers.go imports
findstr /C:"\"!MODULE_NAME!/internal/" internal\handlers\file_handlers.go >nul 2>nul
if %ERRORLEVEL% equ 0 (
    echo    %GREEN%✓%NC% file_handlers.go imports match module name
) else (
    echo    %RED%✗%NC% Import paths in file_handlers.go may not match module name
    echo    Imports in file_handlers.go should start with: "!MODULE_NAME!/..."
)

:: Build application
echo %YELLOW%[3] Building the application...%NC%
cd %APP_DIR%

:: Save current go.mod and go.sum before build
echo    Backing up dependency files...
copy go.mod go.mod.bak >nul
copy go.sum go.sum.bak >nul

:: Run go mod tidy without clearing cache
echo    Running go mod tidy to ensure dependencies are correct...
go mod tidy

:: Compare dependency files, show changes if any
fc /b go.mod go.mod.bak >nul
if %ERRORLEVEL% neq 0 (
    echo    %YELLOW%!%NC% Dependencies changed:
    fc go.mod go.mod.bak
)

:: Build with preserved module cache
echo    Building application...
go build -o %APP_NAME%.exe cmd\server\main.go

if %ERRORLEVEL% neq 0 (
    echo    %RED%✗%NC% Build failed
    echo    Checking for common errors...
    
    findstr /C:"cannot find package" build_output.log >nul 2>nul
    if %ERRORLEVEL% equ 0 (
        echo    %RED%Error:%NC% Missing packages. Try running: go mod download
    )
    
    findstr /C:"ambiguous import" build_output.log >nul 2>nul
    if %ERRORLEVEL% equ 0 (
        echo    %RED%Error:%NC% Ambiguous imports. Check your import paths.
    )
    
    findstr /C:"package .* is not in GOROOT" build_output.log >nul 2>nul
    if %ERRORLEVEL% equ 0 (
        echo    %RED%Error:%NC% Package not found. Check if your module path in go.mod matches your import statements.
    )
    
    exit /b 1
) else (
    echo    %GREEN%✓%NC% Application built successfully
)

:: Create necessary directories if they don't exist
if not exist "%APP_DIR%\uploads" mkdir "%APP_DIR%\uploads"
if not exist "%APP_DIR%\logs" mkdir "%APP_DIR%\logs"
if not exist "%APP_DIR%\data\auth" mkdir "%APP_DIR%\data\auth"

echo    %GREEN%✓%NC% Application built and directories set up

:: Run application
echo %YELLOW%[4] Running the application...%NC%

:: Check if port is available
netstat -ano | findstr ":%PORT%" >nul
if %ERRORLEVEL% equ 0 (
    echo    %RED%✗%NC% Port %PORT% is already in use
    echo    Please stop the process using this port first
    exit /b 1
)

:: Run the application
echo    Starting %APP_NAME% on port %PORT%...
start "File Processor" %APP_NAME%.exe
timeout /t 2 >nul

:: Check if application is running
tasklist /fi "imagename eq %APP_NAME%.exe" | findstr "%APP_NAME%.exe" >nul
if %ERRORLEVEL% equ 0 (
    for /f "tokens=2" %%i in ('tasklist /fi "imagename eq %APP_NAME%.exe" ^| findstr "%APP_NAME%.exe"') do set "PID=%%i"
    echo    %GREEN%✓%NC% Application is running with PID !PID!
) else (
    echo    %RED%✗%NC% Failed to start application
    exit /b 1
)

:: Save PID to file
echo !PID! > %APP_NAME%.pid

:: Display success message and URL
echo.
echo %GREEN%Deployment completed successfully!%NC%
echo Application running at: %GREEN%http://localhost:%PORT%%NC%
echo To stop the application, run: %YELLOW%taskkill /pid !PID! /f%NC%
echo.

endlocal