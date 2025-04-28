@echo off
echo ============================================
echo       Go File Processor Deployment Tool     
echo ============================================

:: Default configuration
set APP_NAME=go-fileprocessor
set APP_DIR=%USERPROFILE%\go-fileprocessor
set DEFAULT_PORT=8080
set ALTERNATE_PORT=9090

:: Port configuration
set PORT=%DEFAULT_PORT%
echo Current port configuration: %PORT%
set /p change_port="Do you want to use a different port? (y/n): "
if /i "%change_port%"=="y" (
    set /p new_port="Enter new port number (1024-65535) [default: %ALTERNATE_PORT%]: "
    if not "%new_port%"=="" (
        set PORT=%new_port%
        echo Port set to: %PORT%
    ) else (
        set PORT=%ALTERNATE_PORT%
        echo Port set to alternate: %PORT%
    )
)

:: Detect IP address for display
for /f "tokens=*" %%a in ('powershell -Command "(Invoke-WebRequest -Uri 'https://api.ipify.org').Content"') do set PUBLIC_IP=%%a
echo Detected IP: %PUBLIC_IP% (You'll use this to access the application)

echo [1] Creating application directories...
if not exist %APP_DIR% mkdir %APP_DIR%
if not exist %APP_DIR%\uploads mkdir %APP_DIR%\uploads
if not exist %APP_DIR%\ui mkdir %APP_DIR%\ui
if not exist %APP_DIR%\logs mkdir %APP_DIR%\logs
if not exist %APP_DIR%\config mkdir %APP_DIR%\config
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

echo [3] Updating configuration file...
set CONFIG_FILE=%APP_DIR%\fileprocessor.ini

if exist %CONFIG_FILE% (
    powershell -Command "(Get-Content %CONFIG_FILE%) -replace 'port = \d+', 'port = %PORT%' | Set-Content %CONFIG_FILE%"
    powershell -Command "(Get-Content %CONFIG_FILE%) -replace 'enable_lan = false', 'enable_lan = true' | Set-Content %CONFIG_FILE%"
    powershell -Command "if ((Get-Content %CONFIG_FILE%) -match '\[server\]') { if ((Get-Content %CONFIG_FILE%) -match 'host =') { (Get-Content %CONFIG_FILE%) -replace 'host = .*', 'host = 0.0.0.0' | Set-Content %CONFIG_FILE% } else { (Get-Content %CONFIG_FILE% | ForEach-Object { if ($_ -match '\[server\]') { $_; 'host = 0.0.0.0' } else { $_ } }) | Set-Content %CONFIG_FILE% } }"
    echo    √ Updated configuration with port %PORT% and network access
) else (
    echo    Creating default configuration file...
    echo [server] > %CONFIG_FILE%
    echo port = %PORT% >> %CONFIG_FILE%
    echo ui_dir = ./ui >> %CONFIG_FILE%
    echo uploads_dir = ./uploads >> %CONFIG_FILE%
    echo host = 0.0.0.0 >> %CONFIG_FILE%
    echo worker_count = 4 >> %CONFIG_FILE%
    echo enable_lan = true >> %CONFIG_FILE%
    echo shutdown_timeout = 30 >> %CONFIG_FILE%
    echo. >> %CONFIG_FILE%
    echo [storage] >> %CONFIG_FILE%
    echo default_provider = local >> %CONFIG_FILE%
    echo. >> %CONFIG_FILE%
    echo [storage.local] >> %CONFIG_FILE%
    echo base_path = ./uploads >> %CONFIG_FILE%
    echo    √ Created default configuration file with port %PORT%
)

echo [4] Checking Go installation...
where go >nul 2>&1
if %ERRORLEVEL% NEQ 0 (
    echo    Go is not installed. Please install Go from https://golang.org/dl/
    echo    After installing, restart this script.
    goto :EOF
) else (
    for /f "tokens=3" %%g in ('go version') do set GO_VERSION=%%g
    echo    Using Go version: %GO_VERSION%
)

echo [5] Building the application...
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

echo [6] Configuring Windows Firewall...
set /p configure_firewall="Do you want to configure Windows Firewall? (y/n): "
if /i "%configure_firewall%"=="y" (
    echo    Adding firewall rule for port %PORT%...
    powershell -Command "New-NetFirewallRule -DisplayName '%APP_NAME%' -Direction Inbound -LocalPort %PORT% -Protocol TCP -Action Allow" >nul 2>&1
    if %ERRORLEVEL% NEQ 0 (
        echo    × Failed to create firewall rule. Try running this script as Administrator.
    ) else (
        echo    √ Firewall rule added for port %PORT%
    )
)

echo [7] Creating Windows service...
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

echo [8] Starting the application...
start "Go File Processor" cmd /c "%APP_DIR%\%APP_NAME%.exe > %APP_DIR%\logs\output.log 2> %APP_DIR%\logs\error.log"
echo    √ Application started in a new window

echo [9] Testing connectivity...
timeout /t 5 > nul
powershell -Command "try { $response = Invoke-WebRequest -Uri http://localhost:%PORT% -UseBasicParsing -TimeoutSec 5; Write-Output ('   Local access: SUCCESS (HTTP Status: ' + $response.StatusCode + ')') } catch { Write-Output '   Local access: FAILED. Application might not be running properly.' }"

echo [10] Running network diagnostics...
echo.
echo    Checking network configuration...
powershell -Command "$privateIP = (Get-NetIPAddress -AddressFamily IPv4 -InterfaceAlias Ethernet*,Wi-Fi | Where-Object {$_.IPAddress -notmatch '^169\.254'}).IPAddress; Write-Output ('   Private IP: ' + $privateIP + ' (accessible from your internal network)')"
echo    Public IP: %PUBLIC_IP% (potentially accessible from the internet)

echo    Checking if application port %PORT% is listening...
powershell -Command "$listening = Get-NetTCPConnection -LocalPort %PORT% -ErrorAction SilentlyContinue; if ($listening) { Write-Output ('   Port %PORT%: LISTENING (Application port)') } else { Write-Output ('   Port %PORT%: NOT LISTENING (Application might not be running)') }"

echo    Checking Windows Firewall status...
powershell -Command "$rule = Get-NetFirewallRule -DisplayName '%APP_NAME%' -ErrorAction SilentlyContinue; if ($rule) { Write-Output ('   Firewall rule for %APP_NAME%: ACTIVE') } else { Write-Output ('   Firewall rule for %APP_NAME%: NOT FOUND') }"

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
echo   - To manage as a service: sc query %APP_NAME% (if you installed it as a service)
echo.
echo Troubleshooting:
echo   - Check Windows Firewall settings if external access fails
echo   - Review logs in %APP_DIR%\logs\ directory
echo   - Make sure the port %PORT% is not used by another application
echo ============================================

pause