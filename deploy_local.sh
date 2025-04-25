#!/bin/bash

# Local deployment script for Go File Processor
# Use this script when you have direct access to the server via Remote Desktop

# Exit on any error
set -e

# Configuration
APP_NAME="go-fileprocessor"
APP_DIR="/opt/$APP_NAME"
SERVICE_NAME="$APP_NAME.service"
USER="$(whoami)"
PORT=8080

echo "Starting local deployment of Go File Processor..."

# Get public IP if available (for information purposes)
PUBLIC_IP=$(curl -s https://api.ipify.org || curl -s http://checkip.amazonaws.com || hostname -I | awk '{print $1}')
echo "Detected IP: $PUBLIC_IP (You'll use this to access the application)"

# Create application directories if they don't exist
echo "Creating application directories..."
sudo mkdir -p $APP_DIR
sudo mkdir -p $APP_DIR/uploads
sudo mkdir -p $APP_DIR/ui
sudo mkdir -p $APP_DIR/logs

# Copy files from current directory to the application directory
echo "Copying application files..."
CURRENT_DIR=$(pwd)
sudo cp -r $CURRENT_DIR/cmd $APP_DIR/
sudo cp -r $CURRENT_DIR/config $APP_DIR/ 2>/dev/null || true
sudo cp -r $CURRENT_DIR/internal $APP_DIR/
sudo cp -r $CURRENT_DIR/ui $APP_DIR/
sudo cp $CURRENT_DIR/go.mod $CURRENT_DIR/go.sum $APP_DIR/
sudo cp $CURRENT_DIR/fileprocessor.ini $APP_DIR/ 2>/dev/null || true

# Set proper permissions
echo "Setting proper permissions..."
sudo chown -R $USER:$USER $APP_DIR
sudo chmod -R 755 $APP_DIR

# Check if Go is installed
if ! command -v go &> /dev/null; then
    echo "Go is not installed. Installing Go..."
    # Install Go (adjust version as needed)
    wget https://go.dev/dl/go1.20.2.linux-amd64.tar.gz
    sudo tar -C /usr/local -xzf go1.20.2.linux-amd64.tar.gz
    echo "export PATH=\$PATH:/usr/local/go/bin" >> ~/.bashrc
    export PATH=$PATH:/usr/local/go/bin
    rm go1.20.2.linux-amd64.tar.gz
fi

# Build the application
echo "Building the application..."
cd $APP_DIR
go build -o $APP_NAME cmd/server/main.go

# Create systemd service file
echo "Creating systemd service..."
cat > /tmp/$SERVICE_NAME << EOF
[Unit]
Description=Go File Processor Service
After=network.target

[Service]
Type=simple
User=$USER
WorkingDirectory=$APP_DIR
ExecStart=$APP_DIR/$APP_NAME
Restart=always
RestartSec=3
StandardOutput=append:$APP_DIR/logs/output.log
StandardError=append:$APP_DIR/logs/error.log
Environment="PORT=$PORT"

[Install]
WantedBy=multi-user.target
EOF

# Install the service
sudo mv /tmp/$SERVICE_NAME /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable $SERVICE_NAME

# Install firewall if it's not installed and open necessary ports
if command -v ufw &> /dev/null; then
    echo "Configuring firewall to allow web traffic..."
    sudo ufw allow 22/tcp  # SSH
    sudo ufw allow 80/tcp  # HTTP
    sudo ufw allow 443/tcp # HTTPS
    sudo ufw allow $PORT/tcp # Application port
    
    # Enable firewall if it's not active
    sudo ufw status | grep -q "Status: active" || sudo ufw --force enable
fi

# Start the service
echo "Starting the service..."
sudo systemctl start $SERVICE_NAME
sudo systemctl status $SERVICE_NAME

# Set up Nginx if available
if command -v nginx > /dev/null; then
    echo "Setting up Nginx reverse proxy..."
    cat > /tmp/$APP_NAME.conf << EOF
server {
    listen 80 default_server;
    listen [::]:80 default_server;
    
    # Use IP directly since there's no domain
    server_name $PUBLIC_IP _;

    # Add security headers
    add_header X-Frame-Options "SAMEORIGIN";
    add_header X-XSS-Protection "1; mode=block";
    add_header X-Content-Type-Options "nosniff";

    # Proxy all requests to the Go application
    location / {
        proxy_pass http://localhost:$PORT;
        proxy_http_version 1.1;
        proxy_set_header Upgrade \$http_upgrade;
        proxy_set_header Connection 'upgrade';
        proxy_set_header Host \$host;
        proxy_cache_bypass \$http_upgrade;
        proxy_set_header X-Real-IP \$remote_addr;
        proxy_set_header X-Forwarded-For \$proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto \$scheme;
    }

    # For WebSocket connections
    location /ws {
        proxy_pass http://localhost:$PORT;
        proxy_http_version 1.1;
        proxy_set_header Upgrade \$http_upgrade;
        proxy_set_header Connection "upgrade";
        proxy_set_header Host \$host;
        proxy_cache_bypass \$http_upgrade;
    }

    # For large file uploads
    client_max_body_size 500M;
}
EOF

    sudo mv /tmp/$APP_NAME.conf /etc/nginx/sites-available/

    # Enable the site if sites-enabled directory exists
    if [ -d "/etc/nginx/sites-enabled" ]; then
        # Remove default Nginx site if it exists
        sudo rm -f /etc/nginx/sites-enabled/default
        sudo ln -sf /etc/nginx/sites-available/$APP_NAME.conf /etc/nginx/sites-enabled/
        
        # Test and reload Nginx
        sudo nginx -t && sudo systemctl restart nginx
    else
        # If sites-enabled doesn't exist, just include the config in nginx.conf
        echo "Nginx sites-enabled directory not found. Creating basic configuration."
        sudo cp /etc/nginx/nginx.conf /etc/nginx/nginx.conf.backup
        sudo sed -i '/http {/a \    include /etc/nginx/sites-available/*;' /etc/nginx/nginx.conf
        sudo nginx -t && sudo systemctl restart nginx
    fi
else
    echo "Nginx not found. You can install it for better performance with:"
    echo "  sudo apt-get update && sudo apt-get install -y nginx"
    echo "Then run this deployment script again."
fi

echo "======================================================"
echo "Deployment complete! The application is now running."
echo ""
echo "You can access it directly via: http://$PUBLIC_IP/"
echo "Or using the application port: http://$PUBLIC_IP:$PORT"
echo ""
echo "To check application status: sudo systemctl status $APP_NAME"
echo "To view logs: sudo journalctl -u $APP_NAME"
echo "Log files are also available at: $APP_DIR/logs/"
echo "======================================================"