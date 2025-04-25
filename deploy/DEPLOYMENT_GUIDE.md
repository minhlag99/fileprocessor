# File Processor Deployment Guide

This guide explains how to deploy the File Processor application to an Ubuntu VPS (Virtual Private Server) and make it accessible from anywhere on the internet.

## Prerequisites

1. **Ubuntu VPS** with:
   - Ubuntu 20.04 or newer
   - SSH access with sudo privileges
   - At least 1GB RAM, 1 CPU core

2. **Local Development Environment** with:
   - Go 1.24.x installed
   - SSH client
   - Git

## Deployment Steps

### 1. Prepare Your Local Environment

1. Clone or download the repository
2. Open `deploy_to_vps.bat` (Windows) or `deploy_to_vps.sh` (Linux/Mac)
3. Update the configuration variables at the top:

```
VPS_USER=ubuntu                 # Your SSH username
VPS_HOST=your-vps-ip-address    # Your VPS IP address
DEPLOY_DIR=/opt/fileprocessor   # Where to install the app
APP_PORT=8080                   # The port to run on
SSH_KEY_PATH=~/.ssh/id_rsa      # Path to your SSH key
```

### 2. Set Up SSH Key Authentication (If Not Already Done)

This allows passwordless authentication to your VPS:

```bash
# Generate an SSH key if you don't have one
ssh-keygen -t rsa -b 4096

# Copy your key to the VPS
ssh-copy-id username@your-vps-ip-address
```

### 3. Run the Deployment Script

From your local machine:

**Windows:**
```
deploy_to_vps.bat
```

**Linux/Mac:**
```
bash deploy_to_vps.sh
```

The deployment script will:
- Build the application for Linux
- Create a configuration file
- Copy all necessary files to your VPS
- Set up a systemd service
- Configure firewall rules
- Perform network checks
- Start the service

### 4. Verify Deployment

After running the deployment script, your application should be accessible at:

```
http://your-vps-ip-address:8080
```

### 5. Troubleshooting Network Access

If your application is only accessible locally on the VPS but not from other locations:

1. Verify the application is binding to all interfaces:
   ```
   sudo cat /opt/fileprocessor/fileprocessor.ini
   ```
   Ensure the "host" is set to "0.0.0.0", not "127.0.0.1" or "localhost".

2. Check firewall settings:
   ```
   sudo ufw status
   ```
   Ensure port 8080 (or your configured port) is allowed.

3. Check the server is listening on all interfaces:
   ```
   sudo ss -tulpn | grep 8080
   ```
   Look for "0.0.0.0:8080" in the output.

4. For cloud-based VPS (AWS, Azure, GCP, etc.), verify cloud provider firewall rules:
   - **AWS**: Check Security Groups
   - **Azure**: Check Network Security Groups
   - **GCP**: Check VPC Firewall Rules

5. Run the network diagnostics script:
   ```
   sudo /opt/fileprocessor/network_setup.sh
   ```

### 6. Managing the Service

- **Check status**: `sudo systemctl status fileprocessor`
- **Start service**: `sudo systemctl start fileprocessor`
- **Stop service**: `sudo systemctl stop fileprocessor`
- **Restart service**: `sudo systemctl restart fileprocessor`
- **View logs**: `sudo journalctl -u fileprocessor`

### 7. Updating the Application

To update to a new version, simply run the deployment script again. It will:
- Build the latest version
- Copy the new files to your VPS
- Restart the service

## Directory Structure on VPS

```
/opt/fileprocessor/            # Main application directory
├── fileprocessor              # The compiled binary
├── fileprocessor.ini          # Configuration file
├── uploads/                   # Directory for file uploads
├── ui/                        # Web interface files
└── network_setup.sh           # Network configuration script

/etc/systemd/system/
└── fileprocessor.service      # Systemd service definition
```

## Security Considerations

1. **File Permissions**: The application runs with restricted privileges via systemd
2. **Firewall**: Only the necessary port is opened
3. **Updates**: Regularly update your Ubuntu VPS with `sudo apt update && sudo apt upgrade`

For additional security, consider setting up HTTPS with a reverse proxy like Nginx.