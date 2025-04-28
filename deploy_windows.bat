@echo off
echo ============================================
echo       Go File Processor Deployment Tool     
echo ============================================

:: Configuration
set APP_NAME=go-fileprocessor
set APP_DIR=%USERPROFILE%\go-fileprocessor
set PORT=8080

:: Detect IP address for display
for /f "tokens=*" %%a in ('powershell -Command "(Invoke-WebRequest -Uri 'https://api.ipify.org').Content"') do set PUBLIC_IP=%%a
echo Detected IP: %PUBLIC_IP% (You'll use this to access the application)

echo [1] Creating application directories...
if not exist %APP_DIR% mkdir %APP_DIR%
if not exist %APP_DIR%\uploads mkdir %APP_DIR%\uploads
if not exist %APP_DIR%\ui mkdir %APP_DIR%\ui
if not exist %APP_DIR%\logs mkdir %APP_DIR%\logs
echo    √ Directories created

echo [2] Copying application files...
xcopy /E /I /Y cmd %APP_DIR%\cmd
xcopy /E /I /Y internal %APP_DIR%\internal
if exist config xcopy /E /I /Y config %APP_DIR%\config
xcopy /E /I /Y ui %APP_DIR%\ui
copy /Y go.mod %APP_DIR%
copy /Y go.sum %APP_DIR%
if exist fileprocessor.ini copy /Y fileprocessor.ini %APP_DIR%
echo    √ Files copied

echo [3] Checking Go installation...
where go >nul 2>&1
if %ERRORLEVEL% NEQ 0 (
    echo    Go is not installed. Please install Go from https://golang.org/dl/
    echo    After installing, restart this script.
    goto :EOF
) else (
    for /f "tokens=3" %%g in ('go version') do set GO_VERSION=%%g
    echo    Using Go version: %GO_VERSION%
)

echo [4] Building the application...
cd %APP_DIR%
go clean -modcache
echo    Running go mod tidy to ensure dependencies are correct...
go mod tidy
echo    Building application...
go build -o %APP_NAME%.exe cmd\server\main.go
if %ERRORLEVEL% NEQ 0 (
    echo    × Build failed
    echo    Please check the error messages above.
    goto :EOF
) else (
    echo    √ Application built successfully
)

echo [5] Creating Windows service...
set /p create_service="Do you want to create a Windows service? (y/n): "
if /i "%create_service%"=="y" (
    echo    Installing Windows service requires NSSM (Non-Sucking Service Manager)
    echo    Please download NSSM from https://nssm.cc/download
    echo    After installing NSSM, run the following commands as administrator:
    echo    nssm install %APP_NAME% "%APP_DIR%\%APP_NAME%.exe"
    echo    nssm set %APP_NAME% AppDirectory "%APP_DIR%"
    echo    nssm set %APP_NAME% AppEnvironment "PORT=%PORT%"
    echo    nssm set %APP_NAME% Start SERVICE_AUTO_START
    echo    nssm set %APP_NAME% AppStdout "%APP_DIR%\logs\output.log"
    echo    nssm set %APP_NAME% AppStderr "%APP_DIR%\logs\error.log"
    echo    nssm start %APP_NAME%
)

echo [6] Starting the application...
start "Go File Processor" cmd /c "%APP_DIR%\%APP_NAME%.exe > %APP_DIR%\logs\output.log 2> %APP_DIR%\logs\error.log"
echo    √ Application started in a new window

echo [7] Testing connectivity...
timeout /t 5 > nul
powershell -Command "try { $response = Invoke-WebRequest -Uri http://localhost:%PORT% -UseBasicParsing -TimeoutSec 5; Write-Output ('   Local access: SUCCESS (HTTP Status: ' + $response.StatusCode + ')') } catch { Write-Output '   Local access: FAILED. Application might not be running properly.' }"

echo.
echo ============================================
echo       Deployment Complete!                  
echo ============================================
echo.
echo Access the application:
echo   - Via HTTP: http://localhost:%PORT%/
echo   - Network: http://%PUBLIC_IP%:%PORT%/
echo.
echo Application Management:
echo   - Logs are located at: %APP_DIR%\logs\
echo   - To stop the application, close the command window or use Task Manager
echo ============================================

pause