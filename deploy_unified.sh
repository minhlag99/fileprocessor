#!/bin/bash

# Unified deployment script for Go File Processor
# This script handles both VPS and local deployments

# Exit on any error
set -e

# Text colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration (can be overridden with environment variables)
APP_NAME=${APP_NAME:-"go-fileprocessor"}
APP_DIR=${APP_DIR:-"/opt/$APP_NAME"}
SERVICE_NAME=${SERVICE_NAME:-"$APP_NAME.service"}
USER=${USER:-"$(whoami)"}
PORT=${PORT:-8080}
GO_VERSION=${GO_VERSION:-"1.21.5"}  # Target Go version (1.21.5 is more widely compatible)

# Banner
echo -e "${BLUE}============================================${NC}"
echo -e "${BLUE}      Go File Processor Deployment Tool     ${NC}"
echo -e "${BLUE}============================================${NC}"

# Get public IP
PUBLIC_IP=$(curl -s https://api.ipify.org || curl -s http://checkip.amazonaws.com || hostname -I | awk '{print $1}')
echo -e "Detected IP: ${GREEN}$PUBLIC_IP${NC} (You'll use this to access the application)"

# Function to create required directories
create_directories() {
    echo -e "${YELLOW}[1] Creating application directories...${NC}"
    sudo mkdir -p $APP_DIR
    sudo mkdir -p $APP_DIR/uploads
    sudo mkdir -p $APP_DIR/ui
    sudo mkdir -p $APP_DIR/logs
    echo -e "   ${GREEN}✓${NC} Directories created"
}

# Function to copy application files
copy_files() {
    echo -e "${YELLOW}[2] Copying application files...${NC}"
    CURRENT_DIR=$(pwd)
    
    sudo cp -r $CURRENT_DIR/cmd $APP_DIR/
    sudo cp -r $CURRENT_DIR/internal $APP_DIR/
    sudo cp -r $CURRENT_DIR/config $APP_DIR/ 2>/dev/null || true
    sudo cp -r $CURRENT_DIR/ui $APP_DIR/
    sudo cp $CURRENT_DIR/go.mod $CURRENT_DIR/go.sum $APP_DIR/
    sudo cp $CURRENT_DIR/fileprocessor.ini $APP_DIR/ 2>/dev/null || true
    
    # Set proper permissions
    sudo chown -R $USER:$USER $APP_DIR
    sudo chmod -R 755 $APP_DIR
    echo -e "   ${GREEN}✓${NC} Files copied and permissions set"
}

# Function to install Go
install_go() {
    echo -e "${YELLOW}[3] Checking Go installation...${NC}"
    if ! command -v go &> /dev/null; then
        echo -e "   Go is not installed. Installing Go $GO_VERSION..."
        
        # Download and install Go
        wget https://go.dev/dl/go$GO_VERSION.linux-amd64.tar.gz
        sudo rm -rf /usr/local/go
        sudo tar -C /usr/local -xzf go$GO_VERSION.linux-amd64.tar.gz
        
        # Add Go to PATH in both .bashrc and .profile for broader compatibility
        if ! grep -q "export PATH=\$PATH:/usr/local/go/bin" ~/.bashrc; then
            echo "export PATH=\$PATH:/usr/local/go/bin" >> ~/.bashrc
        fi
        
        if ! grep -q "export PATH=\$PATH:/usr/local/go/bin" ~/.profile; then
            echo "export PATH=\$PATH:/usr/local/go/bin" >> ~/.profile
        fi
        
        export PATH=$PATH:/usr/local/go/bin
        rm go$GO_VERSION.linux-amd64.tar.gz
        echo -e "   ${GREEN}✓${NC} Go $GO_VERSION installed"
    else
        GO_INSTALLED_VERSION=$(go version | awk '{print $3}' | sed 's/go//')
        echo -e "   ${GREEN}✓${NC} Go $GO_INSTALLED_VERSION already installed"
    fi
    
    # Get the installed Go version for compatibility checks
    GO_INSTALLED_VERSION=$(go version | awk '{print $3}' | sed 's/go//')
    echo -e "   Using Go version: $GO_INSTALLED_VERSION"
    
    # Check go.mod file for compatibility with installed Go version
    MOD_VERSION=$(grep -m 1 "^go " $APP_DIR/go.mod | awk '{print $2}')
    echo -e "   Go version in go.mod: $MOD_VERSION"
    
    # Fix go.mod version compatibility if needed
    if [[ "$MOD_VERSION" > "$GO_INSTALLED_VERSION" ]]; then
        echo -e "   ${YELLOW}⚠${NC} go.mod requires Go $MOD_VERSION but installed version is $GO_INSTALLED_VERSION"
        echo -e "   Adjusting go.mod to be compatible with installed Go version..."
        
        # Create a backup of the original go.mod
        cp $APP_DIR/go.mod $APP_DIR/go.mod.bak
        
        # Update the go.mod file to match the installed Go version
        MAJOR_MINOR_VERSION=$(echo $GO_INSTALLED_VERSION | grep -o "^[0-9]\+\.[0-9]\+")
        sed -i "s/^go .*/go $MAJOR_MINOR_VERSION/" $APP_DIR/go.mod
        echo -e "   ${GREEN}✓${NC} go.mod updated to use Go $MAJOR_MINOR_VERSION"
    fi
}

# Function to build the application
build_application() {
    echo -e "${YELLOW}[4] Building the application...${NC}"
    cd $APP_DIR
    
    # Clear module cache in case of version changes
    go clean -modcache
    
    # First try to tidy the modules with the adjusted go.mod
    echo -e "   Running go mod tidy to ensure dependencies are correct..."
    go mod tidy
    
    echo -e "   Building application..."
    go build -o $APP_NAME cmd/server/main.go
    
    if [ $? -eq 0 ]; then
        echo -e "   ${GREEN}✓${NC} Application built successfully"
    else
        echo -e "   ${RED}✗${NC} Build failed"
        echo -e "   You may need to manually upgrade Go or fix dependency issues"
        exit 1
    fi
}

# Function to create systemd service
create_service() {
    echo -e "${YELLOW}[5] Creating systemd service...${NC}"
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

    sudo mv /tmp/$SERVICE_NAME /etc/systemd/system/
    sudo systemctl daemon-reload
    sudo systemctl enable $SERVICE_NAME
    echo -e "   ${GREEN}✓${NC} Service created and enabled"
}

# Function to configure firewall
configure_firewall() {
    echo -e "${YELLOW}[6] Configuring firewall...${NC}"
    if command -v ufw &> /dev/null; then
        echo -e "   Configuring UFW firewall..."
        sudo ufw allow 22/tcp
        sudo ufw allow 80/tcp
        sudo ufw allow $PORT/tcp
        
        if ! sudo ufw status | grep -q "Status: active"; then
            echo "   Enabling firewall..."
            echo "y" | sudo ufw enable
        fi
        
        echo -e "   ${GREEN}✓${NC} Firewall configured"
    else
        echo -e "   ${YELLOW}⚠${NC} UFW not found. Skipping firewall configuration."
        echo -e "   To install: sudo apt-get update && sudo apt-get install -y ufw"
    fi
}

# Function to install and configure Nginx
install_nginx() {
    echo -e "${YELLOW}[7] Setting up Nginx...${NC}"
    
    # Check if Nginx is already installed
    if ! command -v nginx &> /dev/null; then
        echo -e "   Installing Nginx..."
        sudo apt-get update && sudo apt-get install -y nginx
    fi
    
    # Create Nginx configuration
    echo -e "   Creating Nginx configuration..."
    cat > /tmp/$APP_NAME.conf << EOF
server {
    listen 80 default_server;
    listen [::]:80 default_server;
    
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
    
    # Enable the site
    if [ -d "/etc/nginx/sites-enabled" ]; then
        # Remove default Nginx site if it exists
        sudo rm -f /etc/nginx/sites-enabled/default
        sudo ln -sf /etc/nginx/sites-available/$APP_NAME.conf /etc/nginx/sites-enabled/
    else
        sudo mkdir -p /etc/nginx/sites-enabled
        sudo ln -sf /etc/nginx/sites-available/$APP_NAME.conf /etc/nginx/sites-enabled/
        sudo sed -i '/http {/a \    include /etc/nginx/sites-enabled/*;' /etc/nginx/nginx.conf
    fi
    
    # Test and reload Nginx
    sudo nginx -t && sudo systemctl restart nginx
    echo -e "   ${GREEN}✓${NC} Nginx configured"
}

# Function to start the application
start_application() {
    echo -e "${YELLOW}[8] Starting the application...${NC}"
    sudo systemctl start $SERVICE_NAME
    
    # Check if application started successfully
    sleep 2
    if systemctl is-active --quiet $SERVICE_NAME; then
        echo -e "   ${GREEN}✓${NC} Application started successfully"
    else
        echo -e "   ${RED}✗${NC} Failed to start application"
        echo -e "   Check logs: sudo journalctl -u $SERVICE_NAME -n 50"
        exit 1
    fi
}

# Function to display connection information
display_connection_info() {
    echo -e "${YELLOW}[9] Testing connectivity...${NC}"
    
    # Check if curl is installed
    if command -v curl &> /dev/null; then
        # Test local connection
        HTTP_STATUS=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:$PORT || echo "Connection failed")
        
        if [ "$HTTP_STATUS" = "200" ] || [ "$HTTP_STATUS" = "302" ] || [ "$HTTP_STATUS" = "301" ]; then
            echo -e "   Local access: ${GREEN}SUCCESS${NC} (HTTP Status: $HTTP_STATUS)"
        else
            echo -e "   Local access: ${RED}FAILED${NC} (HTTP Status: $HTTP_STATUS)"
            echo -e "   Make sure your application is running properly."
        fi
    else
        echo -e "   ${YELLOW}⚠${NC} curl is not installed. Cannot test connectivity."
        echo -e "   To install: sudo apt-get update && sudo apt-get install curl"
    fi
    
    echo -e "\n${BLUE}============================================${NC}"
    echo -e "${BLUE}      Deployment Complete!                  ${NC}"
    echo -e "${BLUE}============================================${NC}"
    echo -e "\n${GREEN}Access the application:${NC}"
    echo -e "  - Via HTTP: http://$PUBLIC_IP/"
    echo -e "  - Direct port: http://$PUBLIC_IP:$PORT"
    echo -e "\n${BLUE}Application Management:${NC}"
    echo -e "  - Check status: sudo systemctl status $APP_NAME"
    echo -e "  - View logs: sudo journalctl -u $APP_NAME"
    echo -e "  - Log files: $APP_DIR/logs/"
    echo -e "${BLUE}============================================${NC}"
}

# Main execution flow
main() {
    create_directories
    copy_files
    install_go
    build_application
    create_service
    configure_firewall
    
    # Ask if user wants to install Nginx
    read -p "Do you want to install and configure Nginx? (y/n): " install_nginx_input
    if [[ $install_nginx_input == [Yy]* ]]; then
        install_nginx
    else
        echo -e "${YELLOW}Skipping Nginx installation.${NC}"
    fi
    
    start_application
    display_connection_info
}

# Execute main function
main