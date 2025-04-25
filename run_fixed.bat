@echo off
:: Run this script as administrator to start the Go server
echo Starting Go File Processor Server with admin privileges...

:: Verify Go modules
echo Verifying Go modules...
go mod tidy

:: Run the Go server
echo Running Go server...
go run cmd/server/main.go

pause