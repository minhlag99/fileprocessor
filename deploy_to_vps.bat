@echo off
:: Script to deploy Go File Processor to a VPS
:: You'll need to have pscp.exe (PuTTY SCP) or scp installed

echo ======================================
echo Go File Processor - VPS Deployment Tool
echo ======================================

:: Get VPS connection details from the user
set /p VPS_IP="Enter VPS IP address: "
set /p VPS_USER="Enter VPS username (default: root): " || set VPS_USER=root
set /p SSH_PORT="Enter SSH port (default: 22): " || set SSH_PORT=22
set /p SSH_KEY="Enter path to SSH private key (leave empty for password authentication): "

echo.
echo Creating deployment package...

:: Create a temporary directory for the deployment files
if not exist ".\deploy_temp" mkdir ".\deploy_temp"

:: Copy necessary files to the deployment directory
echo Copying files...
xcopy ".\cmd" ".\deploy_temp\cmd\" /E /I /Y
xcopy ".\config" ".\deploy_temp\config\" /E /I /Y
xcopy ".\internal" ".\deploy_temp\internal\" /E /I /Y
xcopy ".\ui" ".\deploy_temp\ui\" /E /I /Y
copy ".\go.mod" ".\deploy_temp\"
copy ".\go.sum" ".\deploy_temp\"
copy ".\fileprocessor.ini" ".\deploy_temp\" 2>nul
copy ".\deploy.sh" ".\deploy_temp\"

:: Make sure the deploy.sh script is using Unix line endings
powershell -Command "(Get-Content .\deploy_temp\deploy.sh) -replace \"`r`n\", \"`n\" | Set-Content -NoNewline .\deploy_temp\deploy.sh"

echo.
echo Compressing files...
powershell -Command "Compress-Archive -Path .\deploy_temp\* -DestinationPath .\deploy_package.zip -Force"

echo.
echo Transferring files to VPS...

:: Use pscp (PuTTY SCP) if available, otherwise try scp
where pscp >nul 2>nul
if %ERRORLEVEL% EQU 0 (
    if not "%SSH_KEY%"=="" (
        pscp -P %SSH_PORT% -i "%SSH_KEY%" ".\deploy_package.zip" "%VPS_USER%@%VPS_IP%:~/"
    ) else (
        pscp -P %SSH_PORT% ".\deploy_package.zip" "%VPS_USER%@%VPS_IP%:~/"
    )
) else (
    where scp >nul 2>nul
    if %ERRORLEVEL% EQU 0 (
        if not "%SSH_KEY%"=="" (
            scp -P %SSH_PORT% -i "%SSH_KEY%" ".\deploy_package.zip" "%VPS_USER%@%VPS_IP%:~/"
        ) else (
            scp -P %SSH_PORT% ".\deploy_package.zip" "%VPS_USER%@%VPS_IP%:~/"
        )
    ) else (
        echo ERROR: Neither pscp nor scp was found. Please install PuTTY or openssh.
        goto cleanup
    )
)

if %ERRORLEVEL% NEQ 0 (
    echo ERROR: Failed to transfer files to VPS.
    goto cleanup
)

echo.
echo Deploying application on VPS...

:: Run the deployment script on the VPS
where plink >nul 2>nul
if %ERRORLEVEL% EQU 0 (
    if not "%SSH_KEY%"=="" (
        echo | plink -P %SSH_PORT% -i "%SSH_KEY%" %VPS_USER%@%VPS_IP% "unzip -o ~/deploy_package.zip -d ~/fileprocessor && cd ~/fileprocessor && chmod +x deploy.sh && ./deploy.sh"
    ) else (
        echo | plink -P %SSH_PORT% %VPS_USER%@%VPS_IP% "unzip -o ~/deploy_package.zip -d ~/fileprocessor && cd ~/fileprocessor && chmod +x deploy.sh && ./deploy.sh"
    )
) else (
    where ssh >nul 2>nul
    if %ERRORLEVEL% EQU 0 (
        if not "%SSH_KEY%"=="" (
            ssh -p %SSH_PORT% -i "%SSH_KEY%" %VPS_USER%@%VPS_IP% "unzip -o ~/deploy_package.zip -d ~/fileprocessor && cd ~/fileprocessor && chmod +x deploy.sh && ./deploy.sh"
        ) else (
            ssh -p %SSH_PORT% %VPS_USER%@%VPS_IP% "unzip -o ~/deploy_package.zip -d ~/fileprocessor && cd ~/fileprocessor && chmod +x deploy.sh && ./deploy.sh"
        )
    ) else (
        echo ERROR: Neither plink nor ssh was found. Please install PuTTY or openssh.
        goto cleanup
    )
)

if %ERRORLEVEL% NEQ 0 (
    echo ERROR: Failed to deploy application on VPS.
    goto cleanup
)

echo.
echo ======================================
echo Deployment completed successfully!
echo.
echo Your application should now be running on your VPS at http://%VPS_IP%:8080
echo If Nginx was set up correctly, it should also be accessible at http://%VPS_IP%/
echo.
echo Remember to configure SSL/TLS for secure connections if needed.
echo ======================================

:cleanup
:: Clean up temporary files
echo.
echo Cleaning up temporary files...
rmdir /S /Q ".\deploy_temp" 2>nul
echo Done.

pause