#!/bin/bash

# Deployment script for Go File Processor
# This script should be run on the VPS after transferring the codebase

# Exit on any error
set -e

# Configuration
APP_NAME="go-fileprocessor"
APP_DIR="/opt/$APP_NAME"
SERVICE_NAME="$APP_NAME.service"
USER="$(whoami)"
PORT=8080

# Get public IP if available
PUBLIC_IP=$(curl -s https://api.ipify.org || curl -s http://checkip.amazonaws.com || echo "your-server-ip")
echo "Detected public IP: $PUBLIC_IP (if this is incorrect, you'll need to manually update configurations)"

# Create application directories if they don't exist
echo "Creating application directories..."
sudo mkdir -p $APP_DIR
sudo mkdir -p $APP_DIR/uploads
sudo mkdir -p $APP_DIR/ui
sudo mkdir -p $APP_DIR/logs

# Copy files to the application directory
echo "Copying application files..."
sudo cp -r cmd $APP_DIR/
sudo cp -r config $APP_DIR/
sudo cp -r internal $APP_DIR/
sudo cp -r ui $APP_DIR/
sudo cp go.mod go.sum $APP_DIR/
sudo cp fileprocessor.ini $APP_DIR/

# Set proper permissions
echo "Setting proper permissions..."
sudo chown -R $USER:$USER $APP_DIR
sudo chmod -R 755 $APP_DIR

# Check if Go is installed
if ! command -v go &> /dev/null; then
    echo "Go is not installed. Installing Go..."
    # Install Go with the latest version
    wget https://go.dev/dl/go1.24.2.linux-amd64.tar.gz
    sudo rm -rf /usr/local/go
    sudo tar -C /usr/local -xzf go1.24.2.linux-amd64.tar.gz
    echo "export PATH=\$PATH:/usr/local/go/bin" >> ~/.profile
    echo "export PATH=\$PATH:/usr/local/go/bin" >> ~/.bashrc
    export PATH=$PATH:/usr/local/go/bin
    rm go1.24.2.linux-amd64.tar.gz
else
    # Check if Go version needs upgrading
    GO_INSTALLED_VERSION=$(go version | awk '{print $3}' | sed 's/go//')
    echo "Found Go version: $GO_INSTALLED_VERSION"
    
    # Compare versions (this is a simple version check)
    if [ "$(printf '%s\n' "1.24.2" "$GO_INSTALLED_VERSION" | sort -V | head -n1)" != "1.24.2" ]; then
        echo "Upgrading Go to version 1.24.2..."
        wget https://go.dev/dl/go1.24.2.linux-amd64.tar.gz
        sudo rm -rf /usr/local/go
        sudo tar -C /usr/local -xzf go1.24.2.linux-amd64.tar.gz
        rm go1.24.2.linux-amd64.tar.gz
    else
        echo "Go version is up to date."
    fi
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
if ! command -v ufw &> /dev/null; then
    echo "Installing firewall (ufw)..."
    sudo apt-get update && sudo apt-get install -y ufw
fi

echo "Configuring firewall to allow web traffic..."
sudo ufw allow 22/tcp  # SSH
sudo ufw allow 80/tcp  # HTTP
sudo ufw allow 443/tcp # HTTPS
sudo ufw allow $PORT/tcp # Application port

# Enable firewall if it's not active
sudo ufw status | grep -q "Status: active" || sudo ufw --force enable

# Start the service
echo "Starting the service..."
sudo systemctl start $SERVICE_NAME
sudo systemctl status $SERVICE_NAME

# Set up Nginx (if installed)
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
    echo "Nginx not found. Installing Nginx for better performance and security..."
    sudo apt-get update && sudo apt-get install -y nginx
    
    # Run the Nginx setup part again
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
    
    if [ -d "/etc/nginx/sites-enabled" ]; then
        # Remove default Nginx site if it exists
        sudo rm -f /etc/nginx/sites-enabled/default
        sudo ln -sf /etc/nginx/sites-available/$APP_NAME.conf /etc/nginx/sites-enabled/
    else
        sudo mkdir -p /etc/nginx/sites-enabled
        sudo ln -sf /etc/nginx/sites-available/$APP_NAME.conf /etc/nginx/sites-enabled/
        sudo cp /etc/nginx/nginx.conf /etc/nginx/nginx.conf.backup
        sudo sed -i '/http {/a \    include /etc/nginx/sites-enabled/*;' /etc/nginx/nginx.conf
    fi
    
    sudo nginx -t && sudo systemctl restart nginx
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