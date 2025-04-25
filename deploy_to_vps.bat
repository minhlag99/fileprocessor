@echo off
REM Improved deployment script for fileprocessor to Ubuntu VPS from Windows

REM Configuration - change these variables
set VPS_USER=ubuntu
set VPS_HOST=your-vps-ip-address
set DEPLOY_DIR=/opt/fileprocessor
set APP_PORT=8080
set SSH_KEY_PATH=%USERPROFILE%\.ssh\id_rsa
set SSH_OPTIONS=-i "%SSH_KEY_PATH%" -o StrictHostKeyChecking=no

REM Check if Go is installed
where go >nul 2>&1
if %ERRORLEVEL% NEQ 0 (
    echo Error: Go is required but not installed
    exit /b 1
)

REM Check if SSH/SCP are available
where ssh >nul 2>&1
if %ERRORLEVEL% NEQ 0 (
    echo Error: SSH is required but not installed. Install Git Bash or OpenSSH.
    exit /b 1
)

echo Building application for Linux...
set GOOS=linux
set GOARCH=amd64
go build -o fileprocessor ./cmd/server

REM Create config file if it doesn't exist
if not exist fileprocessor.ini (
    echo Creating default config file...
    (
        echo {
        echo     "server": {
        echo         "port": %APP_PORT%,
        echo         "uiDir": "./ui",
        echo         "uploadsDir": "./uploads",
        echo         "host": "0.0.0.0",
        echo         "shutdownTimeout": 30
        echo     },
        echo     "storage": {
        echo         "defaultProvider": "local",
        echo         "local": {
        echo             "basePath": "./uploads"
        echo         }
        echo     },
        echo     "workers": {
        echo         "count": 4,
        echo         "queueSize": 100,
        echo         "maxAttempts": 3
        echo     },
        echo     "features": {
        echo         "enableLAN": true,
        echo         "enableProcessing": true,
        echo         "enableCloudStorage": true,
        echo         "enableProgressUpdates": true
        echo     }
        echo }
    ) > fileprocessor.ini
)

echo Creating deployment directory on VPS...
ssh %SSH_OPTIONS% %VPS_USER%@%VPS_HOST% "sudo mkdir -p %DEPLOY_DIR% && sudo chown %VPS_USER%:%VPS_USER% %DEPLOY_DIR% && mkdir -p %DEPLOY_DIR%/ui %DEPLOY_DIR%/uploads"

echo Copying application files to VPS...
scp %SSH_OPTIONS% fileprocessor fileprocessor.ini %VPS_USER%@%VPS_HOST%:%DEPLOY_DIR%/
scp %SSH_OPTIONS% -r ui/* %VPS_USER%@%VPS_HOST%:%DEPLOY_DIR%/ui/

echo Copying systemd service file and network configuration scripts...
scp %SSH_OPTIONS% deploy/fileprocessor.service deploy/network_setup.sh %VPS_USER%@%VPS_HOST%:/tmp/
ssh %SSH_OPTIONS% %VPS_USER%@%VPS_HOST% "sudo mv /tmp/fileprocessor.service /etc/systemd/system/ && sudo mv /tmp/network_setup.sh %DEPLOY_DIR%/ && sudo chmod +x %DEPLOY_DIR%/network_setup.sh"

echo Setting up firewall on VPS...
ssh %SSH_OPTIONS% %VPS_USER%@%VPS_HOST% "sudo -S ufw allow %APP_PORT%/tcp && sudo ufw status"

echo Configuring and starting service...
ssh %SSH_OPTIONS% %VPS_USER%@%VPS_HOST% "sudo systemctl daemon-reload && sudo systemctl enable fileprocessor.service && sudo systemctl restart fileprocessor.service"

echo Checking service status...
ssh %SSH_OPTIONS% %VPS_USER%@%VPS_HOST% "sudo systemctl status fileprocessor.service"

echo Running network configuration checks...
ssh %SSH_OPTIONS% %VPS_USER%@%VPS_HOST% "sudo %DEPLOY_DIR%/network_setup.sh"

echo Verifying service accessibility...
ssh %SSH_OPTIONS% %VPS_USER%@%VPS_HOST% "curl -s http://localhost:%APP_PORT%/health || echo 'Service not accessible locally'"

echo Deployment complete!
echo Your application should now be accessible at http://%VPS_HOST%:%APP_PORT%
echo.
echo Press any key to continue...
pause > nul