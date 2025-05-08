@echo off
setlocal enabledelayedexpansion

echo ============================================
echo      Go File Processor - Windows Setup      
echo ============================================
echo.

:: Set default configuration
set APP_NAME=fileprocessor
set APP_DIR=%USERPROFILE%\%APP_NAME%
set PORT=9000

:: Check if Go is installed
where go >nul 2>&1
if %ERRORLEVEL% neq 0 (
    echo Go is not installed. Please install Go from https://go.dev/dl/ and try again.
    goto :EXIT
)

echo Go detected: 
go version
echo.

:: Check for port conflicts
echo Checking if port %PORT% is in use...
netstat -ano | findstr ":%PORT% " >nul
if %ERRORLEVEL% equ 0 (
    echo Port %PORT% is already in use. You can specify a different port when running the application.
    set PORT=9001
    echo Will use alternative port %PORT%
)

:: Create directories
echo Creating application directories...
if not exist "%APP_DIR%" mkdir "%APP_DIR%"
if not exist "%APP_DIR%\uploads" mkdir "%APP_DIR%\uploads"
if not exist "%APP_DIR%\ui" mkdir "%APP_DIR%\ui"
if not exist "%APP_DIR%\logs" mkdir "%APP_DIR%\logs"
if not exist "%APP_DIR%\config" mkdir "%APP_DIR%\config"
echo Directories created

:: Copy files
echo.
echo Copying application files...
xcopy /E /Y /I "cmd" "%APP_DIR%\cmd"
xcopy /E /Y /I "internal" "%APP_DIR%\internal"
xcopy /E /Y /I "ui" "%APP_DIR%\ui"
if exist "config" xcopy /E /Y /I "config" "%APP_DIR%\config"
copy "go.mod" "%APP_DIR%\"
copy "go.sum" "%APP_DIR%\"
echo Files copied

:: Create or update configuration
echo.
echo Creating configuration file...
echo {^
    "server": {^
        "port": %PORT%,^
        "uiDir": "./ui",^
        "uploadsDir": "./uploads",^
        "workerCount": 4,^
        "enableLan": true,^
        "shutdownTimeout": 30,^
        "host": "0.0.0.0"^
    },^
    "storage": {^
        "defaultProvider": "local",^
        "local": {^
            "basePath": "./uploads"^
        },^
        "s3": {^
            "region": "",^
            "bucket": "",^
            "accessKey": "",^
            "secretKey": "",^
            "prefix": ""^
        },^
        "google": {^
            "bucket": "",^
            "credentialFile": "",^
            "prefix": ""^
        }^
    },^
    "workers": {^
        "count": 4,^
        "queueSize": 100,^
        "maxAttempts": 3^
    },^
    "features": {^
        "enableLAN": true,^
        "enableProcessing": true,^
        "enableCloudStorage": false,^
        "enableProgressUpdates": true,^
        "enableMediaPreview": true^
    },^
    "ssl": {^
        "enable": false,^
        "certFile": "",^
        "keyFile": ""^
    }^
} > "%APP_DIR%\fileprocessor.json"

echo Configuration file created at %APP_DIR%\fileprocessor.json

:: Build the application
echo.
echo Building the application...
cd "%APP_DIR%"
go mod tidy
go build -o %APP_NAME%.exe cmd\server\main.go

if %ERRORLEVEL% neq 0 (
    echo Build failed. Check error messages above.
    goto :EXIT
)

echo Application built successfully!

:: Run the application
echo.
echo ============================================
echo      Application ready to run!      
echo ============================================
echo.
echo To run the application, execute this command:
echo cd "%APP_DIR%" ^&^& .\%APP_NAME%.exe --config=fileprocessor.json
echo.
echo Or run it now by pressing any key...
pause > nul

:: Launch the application
start "File Processor" cmd /k "cd %APP_DIR% && .\%APP_NAME%.exe --config=fileprocessor.json"
echo Application is starting in a new window...
echo.
echo Access the application at http://localhost:%PORT%
echo Access on your network at http://[your-ip]:%PORT%
echo.

:EXIT
pause