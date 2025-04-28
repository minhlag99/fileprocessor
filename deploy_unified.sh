#!/bin/bash

# Enhanced Unified Deployment Script for Go File Processor
# This script handles local, VPS, and remote desktop deployments

# Exit on any error
set -e

# Text colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Detect operating system
OS_TYPE="linux"
if [[ "$OSTYPE" == "msys" || "$OSTYPE" == "cygwin" || "$OSTYPE" == "win32" ]]; then
    OS_TYPE="windows"
    echo -e "${YELLOW}Windows environment detected.${NC}"
    echo -e "For Windows deployment, please use the ${GREEN}deploy_windows.bat${NC} script instead."
    echo -e "Running this script on Windows may cause errors due to Linux commands."
    read -p "Do you want to continue anyway? (y/n): " continue_anyway
    
    if [[ ! $continue_anyway == [Yy]* ]]; then
        echo -e "${BLUE}Exiting. Please use deploy_windows.bat for Windows deployments.${NC}"
        exit 0
    fi
    
    echo -e "${RED}Warning: Continuing on Windows without sudo. Some commands may fail.${NC}"
    
    # Create a dummy sudo function that just executes the command directly
    sudo() {
        "$@"
    }
fi

# Default configuration (can be overridden with environment variables)
APP_NAME=${APP_NAME:-"go-fileprocessor"}
APP_DIR=${APP_DIR:-"/opt/$APP_NAME"}
if [[ "$OS_TYPE" == "windows" ]]; then
    APP_DIR=${APP_DIR:-"$HOME/$APP_NAME"}
fi
SERVICE_NAME=${SERVICE_NAME:-"$APP_NAME.service"}
USER=${USER:-"$(whoami)"}
PORT=${PORT:-8080}
ALTERNATE_PORT=${ALTERNATE_PORT:-9090}
LATEST_GO_VERSION="1.24.2"

# Remote deployment variables (only used in remote mode)
VPS_USER=""
VPS_HOST=""
SSH_KEY_PATH="~/.ssh/id_rsa"

# Banner
echo -e "${BLUE}============================================${NC}"
echo -e "${BLUE}      Go File Processor Deployment Tool     ${NC}"
echo -e "${BLUE}============================================${NC}"

# Function to prompt for deployment mode
select_deployment_mode() {
    echo -e "\n${YELLOW}Select deployment mode:${NC}"
    echo "1) Local deployment (deploy on this machine)"
    echo "2) Remote deployment via SSH (deploy to VPS)"
    echo "3) Direct server deployment (for Remote Desktop/Console access)"
    echo "4) Run network diagnostics only"
    echo "q) Quit"
    
    read -p "Enter your choice (1, 2, 3, 4, q): " DEPLOYMENT_MODE
    
    case $DEPLOYMENT_MODE in
        1)
            echo -e "\n${GREEN}Selected: Local deployment${NC}"
            deploy_local
            ;;
        2)
            echo -e "\n${GREEN}Selected: Remote deployment via SSH${NC}"
            setup_remote_config
            deploy_remote
            ;;
        3)
            echo -e "\n${GREEN}Selected: Direct server deployment${NC}"
            deploy_direct_server
            ;;
        4)
            echo -e "\n${GREEN}Selected: Network diagnostics${NC}"
            run_network_diagnostics
            ;;
        q|Q)
            echo -e "\n${BLUE}Exiting deployment tool.${NC}"
            exit 0
            ;;
        *)
            echo -e "\n${RED}Invalid option. Please try again.${NC}"
            select_deployment_mode
            ;;
    esac
}

# Function to prompt for port configuration
configure_ports() {
    echo -e "\n${YELLOW}Port Configuration${NC}"
    echo "Default application port is $PORT"
    read -p "Do you want to use a different port? (y/n): " change_port
    
    if [[ $change_port == [Yy]* ]]; then
        read -p "Enter new port number (1024-65535) [default: $ALTERNATE_PORT]: " new_port
        if [[ ! -z "$new_port" ]]; then
            if [[ "$new_port" =~ ^[0-9]+$ ]] && [ "$new_port" -ge 1024 -a "$new_port" -le 65535 ]; then
                PORT=$new_port
                echo -e "${GREEN}Port set to: $PORT${NC}"
            else
                echo -e "${RED}Invalid port. Using default port: $PORT${NC}"
            fi
        else
            PORT=$ALTERNATE_PORT
            echo -e "${GREEN}Port set to alternate: $PORT${NC}"
        fi
    fi
}

# Function to set up remote deployment configuration
setup_remote_config() {
    echo -e "\n${YELLOW}Remote Deployment Configuration${NC}"
    read -p "Enter VPS username (e.g., ubuntu): " VPS_USER
    read -p "Enter VPS IP address: " VPS_HOST
    read -p "Enter SSH key path [default: ~/.ssh/id_rsa]: " SSH_KEY_INPUT
    
    if [ ! -z "$SSH_KEY_INPUT" ]; then
        SSH_KEY_PATH=$SSH_KEY_INPUT
    fi
    
    # Verify connection
    echo -e "\n${YELLOW}Verifying SSH connection...${NC}"
    if ssh -i $SSH_KEY_PATH -o BatchMode=yes -o ConnectTimeout=5 $VPS_USER@$VPS_HOST echo "Connection successful" &> /dev/null; then
        echo -e "${GREEN}✓ SSH connection successful${NC}"
    else
        echo -e "${RED}× SSH connection failed${NC}"
        echo -e "Please check your SSH settings and try again.\n"
        echo -e "If you haven't set up SSH key authentication yet, run:"
        echo -e "  ssh-keygen -t rsa -b 4096  # Generate SSH key if needed"
        echo -e "  ssh-copy-id $VPS_USER@$VPS_HOST  # Copy your key to the VPS"
        exit 1
    fi
    
    # Configure port for remote deployment
    configure_ports
}

# Function for all local deployment steps
deploy_local() {
    # Configure port
    configure_ports
    
    # Get public IP if available (for information purposes)
    PUBLIC_IP=$(curl -s https://api.ipify.org || curl -s http://checkip.amazonaws.com || hostname -I | awk '{print $1}')
    echo -e "Detected IP: ${GREEN}$PUBLIC_IP${NC} (You'll use this to access the application)"
    
    if [[ "$OS_TYPE" == "windows" ]]; then
        echo -e "${YELLOW}Windows environment detected for local deployment.${NC}"
        echo -e "For best results, please use ${GREEN}deploy_windows.bat${NC} instead."
        echo -e "Continuing with limited functionality..."
    fi
    
    create_directories
    copy_files
    update_config_file
    install_go
    build_application
    
    if [[ "$OS_TYPE" != "windows" ]]; then
        create_service
        ensure_ufw_installed
        configure_firewall
        
        # Ask if user wants to install Nginx
        read -p "Do you want to install and configure Nginx? (y/n): " install_nginx_input
        if [[ $install_nginx_input == [Yy]* ]]; then
            install_nginx
        else
            echo -e "${YELLOW}Skipping Nginx installation.${NC}"
        fi
        
        start_application
    else
        # Windows-specific deployment steps
        echo -e "${YELLOW}[5] Starting the application directly...${NC}"
        echo -e "   Starting the application in a new terminal window"
        cd $APP_DIR
        nohup ./$APP_NAME > $APP_DIR/logs/output.log 2> $APP_DIR/logs/error.log &
        echo -e "   ${GREEN}✓${NC} Application started"
    fi
    
    run_network_diagnostics
    display_connection_info
}

# Function for direct server deployment (when SSH is not used)
deploy_direct_server() {
    # Configure port
    configure_ports
    
    # Get public IP if available (for information purposes)
    PUBLIC_IP=$(curl -s https://api.ipify.org || curl -s http://checkip.amazonaws.com || hostname -I | awk '{print $1}')
    echo -e "Detected IP: ${GREEN}$PUBLIC_IP${NC} (You'll use this to access the application)"
    
    # For direct server deployment, we're already on the target machine
    create_directories
    copy_files
    update_config_file
    install_go
    build_application
    create_service
    ensure_ufw_installed
    configure_firewall
    
    # Ask if user wants to install Nginx
    read -p "Do you want to install and configure Nginx? (y/n): " install_nginx_input
    if [[ $install_nginx_input == [Yy]* ]]; then
        install_nginx
    else
        echo -e "${YELLOW}Skipping Nginx installation.${NC}"
    fi
    
    start_application
    run_network_diagnostics
    display_connection_info
}

# Function to create required directories
create_directories() {
    echo -e "${YELLOW}[1] Creating application directories...${NC}"
    sudo mkdir -p $APP_DIR
    sudo mkdir -p $APP_DIR/uploads
    sudo mkdir -p $APP_DIR/ui
    sudo mkdir -p $APP_DIR/logs
    sudo mkdir -p $APP_DIR/config
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
    
    # Copy both INI and JSON configs for flexibility
    if [ -f $CURRENT_DIR/fileprocessor.ini ]; then
        sudo cp $CURRENT_DIR/fileprocessor.ini $APP_DIR/ 2>/dev/null || true
    fi
    
    # Copy the JSON config that we know works
    if [ -f $CURRENT_DIR/config/fileprocessor.json ]; then
        sudo cp $CURRENT_DIR/config/fileprocessor.json $APP_DIR/ 2>/dev/null || true
    elif [ -f $CURRENT_DIR/fileprocessor.json ]; then
        sudo cp $CURRENT_DIR/fileprocessor.json $APP_DIR/ 2>/dev/null || true
    else
        # Create a JSON config if none exists
        create_json_config
    fi
    
    # Set proper permissions
    sudo chown -R $USER:$USER $APP_DIR
    sudo chmod -R 755 $APP_DIR
    echo -e "   ${GREEN}✓${NC} Files copied and permissions set"
}

# Function to create a default JSON configuration file
create_json_config() {
    echo -e "   Creating default JSON configuration file..."
    cat > /tmp/fileprocessor.json << EOF
{
    "server": {
        "port": $PORT,
        "uiDir": "./ui",
        "uploadsDir": "./uploads",
        "workerCount": 4,
        "enableLan": true,
        "shutdownTimeout": 30,
        "host": "0.0.0.0"
    },
    "storage": {
        "defaultProvider": "local",
        "local": {
            "basePath": "./uploads"
        },
        "s3": {
            "region": "",
            "bucket": "",
            "accessKey": "",
            "secretKey": "",
            "prefix": ""
        },
        "google": {
            "bucket": "",
            "credentialFile": "",
            "prefix": ""
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
        "enableCloudStorage": false,
        "enableProgressUpdates": true
    },
    "ssl": {
        "enable": false,
        "certFile": "",
        "keyFile": ""
    }
}
EOF
    sudo mv /tmp/fileprocessor.json $APP_DIR/
    echo -e "   ${GREEN}✓${NC} Created default JSON configuration file with port $PORT"
}

# Function to update configuration file with correct port
update_config_file() {
    echo -e "${YELLOW}[3] Updating configuration files...${NC}"
    
    # Update INI file if it exists (for backward compatibility)
    if [ -f $APP_DIR/fileprocessor.ini ]; then
        # Update port in the INI file
        if grep -q "\[server\]" $APP_DIR/fileprocessor.ini; then
            # This is an INI format
            sed -i "s/port = [0-9]*/port = $PORT/" $APP_DIR/fileprocessor.ini
            # Make sure the enable_lan setting is true
            sed -i "s/enable_lan = false/enable_lan = true/" $APP_DIR/fileprocessor.ini
            echo -e "   ${GREEN}✓${NC} Updated INI configuration with port $PORT"
        else
            echo -e "   ${YELLOW}!${NC} Could not update port in existing INI file"
        fi
        
        # Ensure config allows network access
        if grep -q "\[server\]" $APP_DIR/fileprocessor.ini; then
            # Make sure host is set to 0.0.0.0 to allow external connections
            if grep -q "host" $APP_DIR/fileprocessor.ini; then
                sed -i "s/host = .*/host = 0.0.0.0/" $APP_DIR/fileprocessor.ini
            else
                # Add host setting if not present
                sed -i "/\[server\]/a host = 0.0.0.0" $APP_DIR/fileprocessor.ini
            fi
        fi
    else
        echo -e "   ${YELLOW}!${NC} No INI configuration file found"
    fi
    
    # Update JSON file (the one we will actually use)
    if [ -f $APP_DIR/fileprocessor.json ]; then
        # Update port in JSON
        sed -i 's/"port": [0-9]*/"port": '$PORT'/' $APP_DIR/fileprocessor.json
        # Ensure network access settings
        sed -i 's/"host": "[^"]*"/"host": "0.0.0.0"/' $APP_DIR/fileprocessor.json
        sed -i 's/"enableLan": false/"enableLan": true/' $APP_DIR/fileprocessor.json
        echo -e "   ${GREEN}✓${NC} Updated JSON configuration with port $PORT"
    else
        # Create JSON config if it doesn't exist
        create_json_config
    fi
    
    echo -e "   ${GREEN}✓${NC} Configuration updated to allow external connections"
}

# Function to ensure UFW is installed
ensure_ufw_installed() {
    echo -e "${YELLOW}[*] Ensuring UFW firewall is installed...${NC}"
    if ! command -v ufw &> /dev/null; then
        echo -e "   UFW not found. Installing..."
        sudo apt-get update && sudo apt-get install -y ufw
        echo -e "   ${GREEN}✓${NC} UFW installed"
    else
        echo -e "   ${GREEN}✓${NC} UFW already installed"
    fi
}

# Function to install Go
install_go() {
    echo -e "${YELLOW}[3] Checking Go installation...${NC}"
    
    if ! command -v go &> /dev/null; then
        echo -e "   Go is not installed. Installing Go $LATEST_GO_VERSION..."
        
        # Download and install Go
        wget https://go.dev/dl/go$LATEST_GO_VERSION.linux-amd64.tar.gz
        sudo rm -rf /usr/local/go
        sudo tar -C /usr/local -xzf go$LATEST_GO_VERSION.linux-amd64.tar.gz
        
        # Add Go to PATH in both .bashrc and .profile for broader compatibility
        if ! grep -q "export PATH=\$PATH:/usr/local/go/bin" ~/.bashrc; then
            echo "export PATH=\$PATH:/usr/local/go/bin" >> ~/.bashrc
        fi
        
        if ! grep -q "export PATH=\$PATH:/usr/local/go/bin" ~/.profile; then
            echo "export PATH=\$PATH:/usr/local/go/bin" >> ~/.profile
        fi
        
        export PATH=$PATH:/usr/local/go/bin
        rm go$LATEST_GO_VERSION.linux-amd64.tar.gz
        echo -e "   ${GREEN}✓${NC} Go $LATEST_GO_VERSION installed"
    else
        GO_INSTALLED_VERSION=$(go version | awk '{print $3}' | sed 's/go//')
        echo -e "   Go $GO_INSTALLED_VERSION detected"
        
        # Always check if version needs upgrading to latest
        if [ "$(printf '%s\n' "$LATEST_GO_VERSION" "$GO_INSTALLED_VERSION" | sort -V | head -n1)" != "$LATEST_GO_VERSION" ]; then
            echo -e "   ${YELLOW}⚠${NC} Upgrading Go from $GO_INSTALLED_VERSION to $LATEST_GO_VERSION..."
            
            wget https://go.dev/dl/go$LATEST_GO_VERSION.linux-amd64.tar.gz
            sudo rm -rf /usr/local/go
            sudo tar -C /usr/local -xzf go$LATEST_GO_VERSION.linux-amd64.tar.gz
            
            # Ensure PATH is setup correctly
            if ! grep -q "export PATH=\$PATH:/usr/local/go/bin" ~/.bashrc; then
                echo "export PATH=\$PATH:/usr/local/go/bin" >> ~/.bashrc
            fi
            
            if ! grep -q "export PATH=\$PATH:/usr/local/go/bin" ~/.profile; then
                echo "export PATH=\$PATH:/usr/local/go/bin" >> ~/.profile
            fi
            
            export PATH=$PATH:/usr/local/go/bin
            rm go$LATEST_GO_VERSION.linux-amd64.tar.gz
            echo -e "   ${GREEN}✓${NC} Go upgraded to $LATEST_GO_VERSION"
        else
            echo -e "   ${GREEN}✓${NC} Already using latest Go $GO_INSTALLED_VERSION"
        fi
    fi
    
    # Verify Go installation and version
    GO_INSTALLED_VERSION=$(go version | awk '{print $3}' | sed 's/go//')
    echo -e "   Using Go version: $GO_INSTALLED_VERSION"
    
    # Check go.mod version
    MOD_VERSION=$(grep -m 1 "^go " $APP_DIR/go.mod | awk '{print $2}')
    echo -e "   Go version in go.mod: $MOD_VERSION"
    
    # Explicitly set GOROOT and GOPATH to avoid any issues
    export GOROOT=/usr/local/go
    export GOPATH=$HOME/go
    export PATH=$GOROOT/bin:$GOPATH/bin:$PATH
}

# Function to build the application
build_application() {
    echo -e "${YELLOW}[4] Building the application...${NC}"
    cd $APP_DIR
    
    # Clear module cache in case of version changes
    go clean -modcache
    
    # First try to tidy the modules
    echo -e "   Running go mod tidy to ensure dependencies are correct..."
    go mod tidy
    
    echo -e "   Building application..."
    go build -o $APP_NAME cmd/server/main.go
    
    if [ $? -eq 0 ]; then
        echo -e "   ${GREEN}✓${NC} Application built successfully"
    else
        echo -e "   ${RED}✗${NC} Build failed"
        echo -e "   Checking logs and attempting to diagnose the issue..."
        
        # Check for common build errors and provide guidance
        if [ -f "$APP_DIR/logs/error.log" ]; then
            if grep -q "address already in use" "$APP_DIR/logs/error.log"; then
                echo -e "   ${RED}Error:${NC} Port $PORT is already in use."
                echo -e "   Solution: Either stop the process using port $PORT or change the port in your configuration."
            elif grep -q "permission denied" "$APP_DIR/logs/error.log"; then
                echo -e "   ${RED}Error:${NC} Permission issues detected."
                echo -e "   Solution: Check file permissions in $APP_DIR"
            fi
        fi
        
        echo -e "   You may need to manually fix dependency issues or check service logs for details."
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
ExecStart=$APP_DIR/$APP_NAME --config=$APP_DIR/fileprocessor.json
Restart=always
RestartSec=3
StandardOutput=append:$APP_DIR/logs/output.log
StandardError=append:$APP_DIR/logs/error.log
Environment="PORT=$PORT"
Environment="HOST=0.0.0.0"

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
        sudo ufw allow 22/tcp comment "SSH"
        sudo ufw allow 80/tcp comment "HTTP"
        sudo ufw allow 443/tcp comment "HTTPS"
        sudo ufw allow $PORT/tcp comment "Go File Processor"
        
        if ! sudo ufw status | grep -q "Status: active"; then
            echo "   Enabling firewall..."
            echo "y" | sudo ufw enable
        fi
        
        echo -e "   ${GREEN}✓${NC} Firewall configured"
    else
        echo -e "   ${RED}×${NC} UFW not found even after attempted installation."
        echo -e "   Please manually install UFW: sudo apt-get update && sudo apt-get install -y ufw"
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
    
    # First, check if Nginx is already running on port 80
    if command -v lsof &> /dev/null; then
        PORT_CHECK=$(sudo lsof -i:80 | grep -v nginx)
        if [ ! -z "$PORT_CHECK" ]; then
            echo -e "   ${RED}Warning:${NC} Port 80 is already in use by another process."
            echo -e "   $PORT_CHECK"
            echo -e "   You may need to stop this process before Nginx can use port 80."
        fi
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
    echo -e "   Testing Nginx configuration..."
    if sudo nginx -t; then
        sudo systemctl restart nginx
        echo -e "   ${GREEN}✓${NC} Nginx configured and restarted"
    else
        echo -e "   ${RED}✗${NC} Nginx configuration test failed"
        echo -e "   You may need to manually fix the Nginx configuration"
    fi
}

# Function to start the application with better error reporting
start_application() {
    echo -e "${YELLOW}[8] Starting the application...${NC}"
    
    # Create log directories if not exist
    sudo mkdir -p $APP_DIR/logs
    
    # Try running directly first to catch any immediate errors
    echo -e "   Testing application before starting service..."
    cd $APP_DIR
    DIRECT_OUTPUT=$($APP_DIR/$APP_NAME --config=$APP_DIR/fileprocessor.json --test-config 2>&1 || echo "Failed to start")
    
    if [[ $DIRECT_OUTPUT == *"Failed to start"* ]]; then
        echo -e "   ${RED}✗${NC} Application failed during pre-test"
        echo -e "   Error output: $DIRECT_OUTPUT"
        echo -e "   Attempting to fix common configuration issues..."
        
        # Try to fix common issues - use JSON format
        create_json_config
    else
        echo -e "   ${GREEN}✓${NC} Application configuration test passed"
    fi
    
    # Start the service
    sudo systemctl start $SERVICE_NAME
    
    # Check if application started successfully
    sleep 2
    if systemctl is-active --quiet $SERVICE_NAME; then
        echo -e "   ${GREEN}✓${NC} Application started successfully"
    else
        echo -e "   ${RED}✗${NC} Failed to start application"
        echo -e "   Checking logs for errors..."
        
        # Try to show the most relevant part of the logs to diagnose the issue
        LOGS=$(sudo journalctl -u $SERVICE_NAME -n 20 --no-pager)
        ERROR_LINES=$(echo "$LOGS" | grep -i "error\|failed\|fatal" | tail -n 5)
        
        if [ ! -z "$ERROR_LINES" ]; then
            echo -e "\n${RED}Error details:${NC}"
            echo -e "$ERROR_LINES"
        fi
        
        # Check application logs
        if [ -f "$APP_DIR/logs/error.log" ]; then
            APP_ERROR=$(tail -n 10 "$APP_DIR/logs/error.log")
            echo -e "\n${RED}Application error log:${NC}"
            echo -e "$APP_ERROR"
        fi
        
        echo -e "\nFor complete logs: sudo journalctl -u $SERVICE_NAME -n 50"
        echo -e "Application logs: sudo cat $APP_DIR/logs/error.log"
        
        # Check for common issues
        if netstat -tuln 2>/dev/null | grep -q ":$PORT "; then
            echo -e "\n${RED}Port $PORT is already in use by another process.${NC}"
            echo -e "Try changing the port in the configuration file or stop the conflicting service."
        fi
        
        # Attempt to run the application directly to get better error information
        echo -e "\n${YELLOW}Attempting to run application directly for better error information...${NC}"
        cd $APP_DIR
        $APP_DIR/$APP_NAME --config=$APP_DIR/fileprocessor.json --verbose > $APP_DIR/logs/direct-output.log 2> $APP_DIR/logs/direct-error.log &
        DIRECT_PID=$!
        sleep 3
        kill $DIRECT_PID 2>/dev/null || true
        
        echo -e "Direct run logs saved to: $APP_DIR/logs/direct-error.log"
        echo -e "Check these logs for more detailed error information."
        
        # Try to fix permission issues
        echo -e "\n${YELLOW}Fixing potential permission issues...${NC}"
        sudo chown -R $USER:$USER $APP_DIR
        sudo chmod -R 755 $APP_DIR
        
        exit 1
    fi
}

# Function to run network diagnostics
run_network_diagnostics() {
    echo -e "${YELLOW}[9] Running network diagnostics...${NC}"
    
    # Get private IP (internal network)
    PRIVATE_IP=$(hostname -I 2>/dev/null | awk '{print $1}' || echo "Unknown")
    echo -e "  - Private IP: ${GREEN}$PRIVATE_IP${NC} (accessible from your internal network)"
    
    # Get public IP (internet-facing)
    PUBLIC_IP=$(curl -s https://api.ipify.org || curl -s http://checkip.amazonaws.com || echo "Could not determine")
    if [ "$PUBLIC_IP" != "Could not determine" ]; then
        echo -e "  - Public IP:  ${GREEN}$PUBLIC_IP${NC} (potentially accessible from the internet)"
    else
        echo -e "  - Public IP:  ${RED}Could not determine${NC} (may be behind NAT or firewall)"
    fi
    
    # Check listening ports
    echo -e "\n  Checking listening ports..."
    if command -v ss &> /dev/null; then
        TOOL="ss -tulpn"
    elif command -v netstat &> /dev/null; then
        TOOL="netstat -tulpn"
    else
        echo -e "  ${RED}Cannot check listening ports (ss/netstat not installed)${NC}"
        echo "  To install: sudo apt-get update && sudo apt-get install net-tools"
        TOOL=""
    fi
    
    if [ ! -z "$TOOL" ]; then
        if $TOOL 2>/dev/null | grep -q ":$PORT "; then
            echo -e "  - Port $PORT: ${GREEN}LISTENING${NC} (Application port)"
        else
            echo -e "  - Port $PORT: ${RED}NOT LISTENING${NC} (Application might not be running)"
        fi
        
        if $TOOL 2>/dev/null | grep -q ":80 "; then
            echo -e "  - Port 80:   ${GREEN}LISTENING${NC} (HTTP/Nginx)"
        else
            echo -e "  - Port 80:   ${RED}NOT LISTENING${NC} (HTTP/Nginx might not be running)"
        fi
    fi
    
    # Check firewall status
    echo -e "\n  Checking firewall rules..."
    if command -v ufw &> /dev/null; then
        UFW_STATUS=$(sudo ufw status | grep "Status: " | awk '{print $2}')
        if [ "$UFW_STATUS" = "active" ]; then
            echo -e "  - Firewall:  ${GREEN}ACTIVE${NC}"
            
            # Check if ports are open
            HTTP_ALLOWED=$(sudo ufw status | grep "80/tcp" | grep "ALLOW" | wc -l)
            APP_PORT_ALLOWED=$(sudo ufw status | grep "$PORT/tcp" | grep "ALLOW" | wc -l)
            
            if [ $HTTP_ALLOWED -gt 0 ]; then
                echo -e "    - Port 80:   ${GREEN}OPEN${NC}"
            else
                echo -e "    - Port 80:   ${RED}CLOSED${NC}"
            fi
            
            if [ $APP_PORT_ALLOWED -gt 0 ]; then
                echo -e "    - Port $PORT: ${GREEN}OPEN${NC}"
            else
                echo -e "    - Port $PORT: ${RED}CLOSED${NC}"
            fi
        else
            echo -e "  - Firewall:  ${YELLOW}INACTIVE${NC} (all ports open)"
        fi
    else
        echo -e "  - Firewall:  ${YELLOW}NOT INSTALLED${NC}"
    fi
    
    # Check HTTP connectivity
    echo -e "\n  Testing HTTP connectivity..."
    if command -v curl &> /dev/null; then
        # Test local app connection
        HTTP_RESPONSE=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:$PORT 2>/dev/null || echo "Failed")
        if [ "$HTTP_RESPONSE" = "200" ] || [ "$HTTP_RESPONSE" = "302" ] || [ "$HTTP_RESPONSE" = "301" ]; then
            echo -e "  - Local app: ${GREEN}REACHABLE${NC} (HTTP $HTTP_RESPONSE)"
        else
            echo -e "  - Local app: ${RED}NOT REACHABLE${NC} (HTTP $HTTP_RESPONSE)"
        fi
        
        # Test local nginx if installed
        if command -v nginx &> /dev/null; then
            HTTP_RESPONSE=$(curl -s -o /dev/null -w "%{http_code}" http://localhost 2>/dev/null || echo "Failed")
            if [ "$HTTP_RESPONSE" = "200" ] || [ "$HTTP_RESPONSE" = "302" ] || [ "$HTTP_RESPONSE" = "301" ]; then
                echo -e "  - Local web: ${GREEN}REACHABLE${NC} (HTTP $HTTP_RESPONSE)"
            else
                echo -e "  - Local web: ${RED}NOT REACHABLE${NC} (HTTP $HTTP_RESPONSE)"
            fi
        fi
    else
        echo -e "  - HTTP Test: ${YELLOW}SKIPPED${NC} (curl not installed)"
    fi
}

# Function to display connection information
display_connection_info() {
    echo -e "${YELLOW}[10] Connection summary...${NC}"
    
    echo -e "\n${BLUE}============================================${NC}"
    echo -e "${BLUE}      Deployment Complete!                  ${NC}"
    echo -e "${BLUE}============================================${NC}"
    echo -e "\n${GREEN}Access the application:${NC}"
    echo -e "  - Via HTTP: http://$PUBLIC_IP/"
    echo -e "  - Direct application port: http://$PUBLIC_IP:$PORT/"
    if [ "$(command -v nginx)" ] && [ "$(systemctl is-active nginx)" == "active" ]; then
        echo -e "  - Via Nginx: http://$PUBLIC_IP/"
    fi
    echo -e "\n${BLUE}Application Management:${NC}"
    echo -e "  - Check status: sudo systemctl status $APP_NAME"
    echo -e "  - View logs: sudo journalctl -u $APP_NAME"
    echo -e "  - Log files: $APP_DIR/logs/"
    echo -e "\n${BLUE}Troubleshooting:${NC}"
    echo -e "  - Run this script with option 4 to diagnose network issues"
    echo -e "  - Check firewall settings: sudo ufw status"
    echo -e "  - Verify Nginx config: sudo nginx -t"
    echo -e "  - Reload service: sudo systemctl restart $APP_NAME"
    echo -e "${BLUE}============================================${NC}"
}

# Function for remote deployment
deploy_remote() {
    echo -e "${YELLOW}Preparing remote deployment to ${VPS_USER}@${VPS_HOST}...${NC}"
    
    # Build the application locally for Linux
    echo -e "${YELLOW}[1] Building application for Linux...${NC}"
    GOOS=linux GOARCH=amd64 go build -o fileprocessor ./cmd/server/main.go
    
    # Create JSON config file
    echo -e "${YELLOW}[2] Creating JSON config file...${NC}"
    cat > fileprocessor.json << EOF
{
    "server": {
        "port": $PORT,
        "uiDir": "./ui",
        "uploadsDir": "./uploads",
        "workerCount": 4,
        "enableLan": true,
        "shutdownTimeout": 30,
        "host": "0.0.0.0"
    },
    "storage": {
        "defaultProvider": "local",
        "local": {
            "basePath": "./uploads"
        },
        "s3": {
            "region": "",
            "bucket": "",
            "accessKey": "",
            "secretKey": "",
            "prefix": ""
        },
        "google": {
            "bucket": "",
            "credentialFile": "",
            "prefix": ""
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
        "enableCloudStorage": false,
        "enableProgressUpdates": true
    },
    "ssl": {
        "enable": false,
        "certFile": "",
        "keyFile": ""
    }
}
EOF

    echo -e "${YELLOW}[3] Creating deployment directory on VPS...${NC}"
    ssh -i $SSH_KEY_PATH $VPS_USER@$VPS_HOST "sudo mkdir -p $APP_DIR && sudo mkdir -p $APP_DIR/logs && sudo chown $VPS_USER:$VPS_USER $APP_DIR && mkdir -p $APP_DIR/ui $APP_DIR/uploads"

    echo -e "${YELLOW}[4] Copying application files to VPS...${NC}"
    scp -i $SSH_KEY_PATH fileprocessor fileprocessor.json $VPS_USER@$VPS_HOST:$APP_DIR/
    scp -i $SSH_KEY_PATH -r ui/* $VPS_USER@$VPS_HOST:$APP_DIR/ui/

    # Copy systemd service file with JSON config
    echo -e "${YELLOW}[5] Setting up systemd service on VPS...${NC}"
    cat > /tmp/fileprocessor.service << EOF
[Unit]
Description=Go File Processor Service
After=network.target

[Service]
Type=simple
User=$VPS_USER
WorkingDirectory=$APP_DIR
ExecStart=$APP_DIR/fileprocessor --config=$APP_DIR/fileprocessor.json
Restart=always
RestartSec=3
StandardOutput=append:$APP_DIR/logs/output.log
StandardError=append:$APP_DIR/logs/error.log
Environment="PORT=$PORT"
Environment="HOST=0.0.0.0"

[Install]
WantedBy=multi-user.target
EOF
    scp -i $SSH_KEY_PATH /tmp/fileprocessor.service $VPS_USER@$VPS_HOST:/tmp/
    ssh -i $SSH_KEY_PATH $VPS_USER@$VPS_HOST "sudo mv /tmp/fileprocessor.service /etc/systemd/system/"

    echo -e "${YELLOW}[6] Setting up firewall on VPS...${NC}"
    ssh -i $SSH_KEY_PATH $VPS_USER@$VPS_HOST "sudo -S ufw allow $PORT/tcp && sudo ufw allow 80/tcp && sudo ufw allow 22/tcp"

    echo -e "${YELLOW}[7] Configuring and starting service...${NC}"
    ssh -i $SSH_KEY_PATH $VPS_USER@$VPS_HOST "sudo systemctl daemon-reload && sudo systemctl enable fileprocessor.service && sudo systemctl restart fileprocessor.service"

    # Add a validation step to check if service started correctly
    echo -e "${YELLOW}[8] Validating service status...${NC}"
    IS_ACTIVE=$(ssh -i $SSH_KEY_PATH $VPS_USER@$VPS_HOST "systemctl is-active fileprocessor.service || echo 'inactive'")
    
    if [ "$IS_ACTIVE" == "active" ]; then
        echo -e "   ${GREEN}✓${NC} Service started successfully"
    else
        echo -e "   ${RED}×${NC} Service failed to start"
        echo -e "   Fetching error logs..."
        
        ssh -i $SSH_KEY_PATH $VPS_USER@$VPS_HOST "sudo journalctl -u fileprocessor.service -n 20 --no-pager"
        echo -e "\n   Testing configuration directly..."
        
        # Try to run application directly to get better error information
        ssh -i $SSH_KEY_PATH $VPS_USER@$VPS_HOST "cd $APP_DIR && $APP_DIR/fileprocessor --config=$APP_DIR/fileprocessor.json --test-config"
        
        echo -e "\n   Checking application logs..."
        ssh -i $SSH_KEY_PATH $VPS_USER@$VPS_HOST "cat $APP_DIR/logs/error.log 2>/dev/null || echo 'No error log found'"
        
        echo -e "\n${RED}Service failed to start. Please check the logs above for details.${NC}"
        echo -e "You can manually check logs on the VPS with these commands:"
        echo -e "  ssh -i $SSH_KEY_PATH $VPS_USER@$VPS_HOST"
        echo -e "  sudo journalctl -u fileprocessor.service -n 50"
        echo -e "  cat $APP_DIR/logs/error.log"
    fi

    echo -e "${YELLOW}[8] Setting up Nginx (if available)...${NC}"
    ssh -i $SSH_KEY_PATH $VPS_USER@$VPS_HOST "if command -v nginx &> /dev/null; then 
        sudo bash -c 'cat > /etc/nginx/sites-available/fileprocessor.conf << EOF
server {
    listen 80 default_server;
    listen [::]:80 default_server;
    
    server_name \$(curl -s http://checkip.amazonaws.com || hostname -I | awk \"{print \\\$1}\") _;
    
    location / {
        proxy_pass http://localhost:$PORT;
        proxy_http_version 1.1;
        proxy_set_header Upgrade \\$http_upgrade;
        proxy_set_header Connection \"upgrade\";
        proxy_set_header Host \\$host;
        proxy_cache_bypass \\$http_upgrade;
    }
    
    client_max_body_size 500M;
}
EOF'
        sudo ln -sf /etc/nginx/sites-available/fileprocessor.conf /etc/nginx/sites-enabled/
        sudo rm -f /etc/nginx/sites-enabled/default &> /dev/null
        sudo nginx -t && sudo systemctl restart nginx
        echo 'Nginx configured'
    else
        echo 'Nginx not installed on VPS'
    fi"

    # Get the VPS public IP
    REMOTE_IP=$(ssh -i $SSH_KEY_PATH $VPS_USER@$VPS_HOST "curl -s http://checkip.amazonaws.com || hostname -I | awk '{print \$1}'")
    
    echo -e "\n${BLUE}============================================${NC}"
    echo -e "${BLUE}      Remote Deployment Complete!           ${NC}"
    echo -e "${BLUE}============================================${NC}"
    echo -e "\n${GREEN}Access the application:${NC}"
    echo -e "  - Via HTTP: http://$REMOTE_IP/"
    echo -e "  - Direct port: http://$REMOTE_IP:$PORT"
    echo -e "\n${BLUE}Remote Management:${NC}"
    echo -e "  - SSH into server: ssh -i $SSH_KEY_PATH $VPS_USER@$VPS_HOST"
    echo -e "  - Check status: sudo systemctl status fileprocessor"
    echo -e "  - View logs: sudo journalctl -u fileprocessor"
    echo -e "  - Application logs: cat $APP_DIR/logs/error.log"
    echo -e "  - Restart service: sudo systemctl restart fileprocessor"
    echo -e "${BLUE}============================================${NC}"
}

# Start by selecting deployment mode
select_deployment_mode