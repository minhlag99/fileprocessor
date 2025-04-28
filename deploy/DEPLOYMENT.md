# Go File Processor Deployment Guide

This document provides instructions for deploying the Go File Processor application using the unified deployment script.

## Deployment Options

The `deploy_unified.sh` script supports three main modes of operation:

1. **Local Deployment**: Install and run the application on the current machine
2. **Remote Deployment**: Build locally and deploy to a remote VPS
3. **Network Diagnostics**: Run diagnostics to troubleshoot connectivity issues

## Prerequisites

### For Local Deployment
- A Linux server or VPS (Ubuntu/Debian recommended)
- Sudo privileges
- Internet access for downloading dependencies

### For Remote Deployment
- SSH key authentication set up with your VPS
- Go installed on your local machine for building
- SSH access to the remote server

## Quick Start

1. Clone the repository to your local machine or server
2. Navigate to the project root directory
3. Make the deployment script executable:
   ```
   chmod +x deploy_unified.sh
   ```
4. Run the deployment script:
   ```
   ./deploy_unified.sh
   ```
5. Follow the interactive prompts to select your deployment mode

## Deployment Modes Explained

### 1. Local Deployment

This mode installs and runs the application on the current machine. It:

- Creates all necessary directories
- Installs or updates Go to version 1.24.2
- Builds the application
- Sets up a systemd service
- Configures firewall rules
- Optionally installs and configures Nginx as a reverse proxy
- Runs network diagnostics to verify everything is working

### 2. Remote Deployment

This mode builds the application locally and deploys it to a remote server. It:

- Builds the application for Linux
- Transfers the binary and required files to your VPS
- Sets up directories and permissions on the remote server
- Creates and enables a systemd service
- Configures firewall rules
- Optionally sets up Nginx (if installed)
- Verifies the application is running

### 3. Network Diagnostics

This mode only runs network diagnostics to help troubleshoot issues with an existing deployment:

- Checks public and private IP addresses
- Verifies listening ports
- Examines firewall configurations
- Tests HTTP connectivity

## After Deployment

- Access the application at: `http://<your-server-ip>/`
- If Nginx is not installed, access via: `http://<your-server-ip>:8080/`

## Common Issues and Solutions

### Application Won't Start

Check the application logs:
```
sudo journalctl -u go-fileprocessor -n 50
```

### Port Already in Use

If port 8080 is already in use, either:
1. Change the port in `fileprocessor.ini`
2. Stop the process using port 8080

### Nginx Configuration Errors

Check Nginx configuration validity:
```
sudo nginx -t
```

### Cannot Access from Browser

1. Run the network diagnostics mode to verify connectivity
2. Check firewall rules to ensure ports 80 and 8080 are open
3. If using a cloud provider, verify security group/network rules

## Managing the Service

- Check status: `sudo systemctl status go-fileprocessor`
- Stop service: `sudo systemctl stop go-fileprocessor`
- Start service: `sudo systemctl start go-fileprocessor`
- Restart service: `sudo systemctl restart go-fileprocessor`
- View logs: `sudo journalctl -u go-fileprocessor`