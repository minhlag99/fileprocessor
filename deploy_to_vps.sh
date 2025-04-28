#!/bin/bash
# Improved deployment script for fileprocessor to Ubuntu VPS

# Configuration - change these variables
VPS_USER="ubuntu"
VPS_HOST="your-vps-ip-address"
DEPLOY_DIR="/opt/fileprocessor"
APP_PORT=8080

# Verify required tools
command -v go >/dev/null 2>&1 || { echo "Error: Go is required but not installed"; exit 1; }
command -v ssh >/dev/null 2>&1 || { echo "Error: SSH is required but not installed"; exit 1; }
command -v scp >/dev/null 2>&1 || { echo "Error: SCP is required but not installed"; exit 1; }

# Build the application for Linux
echo "Building application for Linux..."
GOOS=linux GOARCH=amd64 go build -o fileprocessor ./cmd/server

# Create config file if it doesn't exist
if [ ! -f fileprocessor.ini ]; then
    echo "Creating default config file..."
    cat > fileprocessor.ini << EOF
{
    "server": {
        "port": $APP_PORT,
        "uiDir": "./ui",
        "uploadsDir": "./uploads",
        "host": "0.0.0.0",
        "shutdownTimeout": 30
    },
    "storage": {
        "defaultProvider": "local",
        "local": {
            "basePath": "./uploads"
        }
    },
    "workers": {
        "count": 4,
        "queueSize": 100,
        "maxAttempts": 3
    },
    "features": {
        "enableLAN": true,
        "enableProcessing": true,
        "enableCloudStorage": true,
        "enableProgressUpdates": true
    }
}
EOF
fi

echo "Creating deployment directory on VPS..."
ssh $VPS_USER@$VPS_HOST "sudo mkdir -p $DEPLOY_DIR && sudo chown $VPS_USER:$VPS_USER $DEPLOY_DIR && mkdir -p $DEPLOY_DIR/ui $DEPLOY_DIR/uploads"

echo "Copying application files to VPS..."
scp fileprocessor fileprocessor.ini $VPS_USER@$VPS_HOST:$DEPLOY_DIR/
scp -r ui/* $VPS_USER@$VPS_HOST:$DEPLOY_DIR/ui/

# Copy systemd service file and network setup script
echo "Copying systemd service file and network configuration scripts..."
scp deploy/fileprocessor.service deploy/network_setup.sh $VPS_USER@$VPS_HOST:/tmp/
ssh $VPS_USER@$VPS_HOST "sudo mv /tmp/fileprocessor.service /etc/systemd/system/ && \
                         sudo mv /tmp/network_setup.sh $DEPLOY_DIR/ && \
                         sudo chmod +x $DEPLOY_DIR/network_setup.sh"

echo "Setting up firewall on VPS..."
ssh $VPS_USER@$VPS_HOST "sudo -S ufw allow $APP_PORT/tcp && sudo ufw status" 

echo "Configuring and starting service..."
ssh $VPS_USER@$VPS_HOST "sudo systemctl daemon-reload && sudo systemctl enable fileprocessor.service && sudo systemctl restart fileprocessor.service"

echo "Checking service status..."
ssh $VPS_USER@$VPS_HOST "sudo systemctl status fileprocessor.service"

echo "Running network configuration checks..."
ssh $VPS_USER@$VPS_HOST "sudo $DEPLOY_DIR/network_setup.sh"

# Verify the service is running and accessible
echo "Verifying service accessibility..."
ssh $VPS_USER@$VPS_HOST "curl -s http://localhost:$APP_PORT/health || echo 'Service not accessible locally'"

echo "Deployment complete!"
echo "Your application should now be accessible at http://$VPS_HOST:$APP_PORT"