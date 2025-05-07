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
PORT=${PORT:-9000}
ALTERNATE_PORT=${ALTERNATE_PORT:-9001}
LATEST_GO_VERSION="1.24.2"

# Remote deployment variables (only used in remote mode)
VPS_USER=""
VPS_HOST=""
SSH_KEY_PATH="~/.ssh/id_rsa"

# Banner
echo -e "${BLUE}============================================${NC}"
echo -e "${BLUE}      Go File Processor Deployment Tool     ${NC}"
echo -e "${BLUE}============================================${NC}"

# Function to print the banner
print_banner() {
    echo -e "${YELLOW}"
    echo "╔════════════════════════════════════════════════════════╗"
    echo "║       Go File Processor Deployment Tool                ║"
    echo "╚════════════════════════════════════════════════════════╝"
    echo -e "${NC}"
}

# Function to set configuration variables
set_config_variables() {
    # Add your configuration variable settings here
    echo "Setting configuration variables..."
}

# Function to check system requirements
check_system_requirements() {
    echo "Checking system requirements..."
    # Add your system requirement checks here
}

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
    ensure_dependencies_installed
    check_port_in_use
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
    public_connectivity_test
    display_connection_info
}

# Function for direct server deployment (when SSH is not used)
deploy_direct_server() {
    ensure_dependencies_installed
    check_port_in_use
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
    public_connectivity_test
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
    if command -v ufw &> /dev/null; then
        echo -e "${YELLOW}[6] Configuring firewall...${NC}"
        
        # Allow SSH first to prevent lockout
        sudo ufw allow ssh
        
        # Allow HTTP (Nginx) access from anywhere
        sudo ufw allow 80/tcp
        
        # Allow the application port
        sudo ufw allow $PORT/tcp
        
        # Check UFW status and only enable if not already enabled
        UFW_STATUS=$(sudo ufw status | grep "Status: " | awk '{print $2}')
        if [ "$UFW_STATUS" != "active" ]; then
            echo -e "   Enabling firewall..."
            echo "y" | sudo ufw enable
            
            # If enabling fails, try again with --force flag
            if [ $? -ne 0 ]; then
                echo -e "   ${YELLOW}!${NC} UFW enable failed, trying with force option..."
                echo "y" | sudo ufw --force enable
            fi
        else
            echo -e "   Firewall already active, ensuring rules are applied..."
            # Ensure critical rules exist even if already enabled
            sudo ufw allow ssh
            sudo ufw allow 80/tcp
            sudo ufw allow $PORT/tcp
        fi
        
        echo -e "   ${GREEN}✓${NC} Firewall configured"
    else
        echo -e "   ${RED}×${NC} UFW not found. Installing..."
        sudo apt-get update && sudo apt-get install -y ufw
        
        # Try again after installation
        if command -v ufw &> /dev/null; then
            configure_firewall
        else
            echo -e "   ${RED}×${NC} UFW not found even after attempted installation."
            echo -e "   Please manually install UFW: sudo apt-get update && sudo apt-get install -y ufw"
        fi
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
        proxy_pass http://127.0.0.1:$PORT;
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
        proxy_pass http://127.0.0.1:$PORT;
        proxy_http_version 1.1;
        proxy_set_header Upgrade \$http_upgrade;
        proxy_set_header Connection "upgrade";
        proxy_set_header Host \$host;
        proxy_cache_bypass \$http_upgrade;
    }
    
    # Health check endpoint for connectivity testing
    location /health {
        proxy_pass http://127.0.0.1:$PORT/health;
        proxy_http_version 1.1;
        proxy_set_header Host \$host;
        proxy_cache_bypass \$http_upgrade;
        access_log off;
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
    echo -e "${YELLOW}[*] Running comprehensive network diagnostics...${NC}"
    
    # Get public and private IPs
    echo -e "   ${GREEN}Network interfaces:${NC}"
    ip addr show | grep -E "inet |inet6 " | grep -v "127.0.0.1" || echo "   ${YELLOW}No IP address found${NC}"
    
    # Get routing table
    echo -e "\n   ${GREEN}Routing table:${NC}"
    ip route || netstat -rn || echo "   ${YELLOW}Cannot display routing table${NC}"
    
    # Check the application port status
    echo -e "\n   ${GREEN}Checking if application port $PORT is open:${NC}"
    if command -v lsof &> /dev/null; then
        sudo lsof -i:$PORT || echo "   ${YELLOW}No process listening on port $PORT${NC}"
    elif command -v netstat &> /dev/null; then
        sudo netstat -tuln | grep ":$PORT " || echo "   ${YELLOW}No process listening on port $PORT${NC}"
    else 
        echo "   ${YELLOW}Cannot check port status (lsof/netstat not installed)${NC}"
    fi
    
    # Check if port 80 (HTTP) is open
    echo -e "\n   ${GREEN}Checking if HTTP port 80 is open:${NC}"
    if command -v lsof &> /dev/null; then
        sudo lsof -i:80 || echo "   ${YELLOW}No process listening on port 80${NC}"
    elif command -v netstat &> /dev/null; then
        sudo netstat -tuln | grep ":80 " || echo "   ${YELLOW}No process listening on port 80${NC}"
    else
        echo "   ${YELLOW}Cannot check port status (lsof/netstat not installed)${NC}"
    fi
    
    # Check DNS resolution
    echo -e "\n   ${GREEN}Testing DNS resolution:${NC}"
    if command -v dig &> /dev/null; then
        echo -e "   Testing with dig:"
        dig +short google.com
    elif command -v nslookup &> /dev/null; then
        echo -e "   Testing with nslookup:"
        nslookup google.com | grep -i "address" | tail -n 2
    else
        echo -e "   Testing with basic hostname resolution:"
        ping -c 1 google.com | head -n 1
    fi
    
    # Check internet connectivity
    echo -e "\n   ${GREEN}Testing internet connectivity:${NC}"
    if ping -c 3 -W 2 8.8.8.8 &> /dev/null; then
        echo -e "   ${GREEN}✓${NC} Internet connectivity: GOOD (Can reach 8.8.8.8)"
    else
        echo -e "   ${RED}×${NC} Internet connectivity: FAILED (Cannot reach 8.8.8.8)"
    fi
    
    if ping -c 3 -W 2 google.com &> /dev/null; then
        echo -e "   ${GREEN}✓${NC} DNS resolution: GOOD (Can reach google.com)"
    else
        echo -e "   ${RED}×${NC} DNS resolution: FAILED (Cannot reach google.com)"
    fi
    
    # Check firewall status
    echo -e "\n   ${GREEN}Checking firewall status:${NC}"
    if command -v ufw &> /dev/null; then
        sudo ufw status verbose
    else
        echo "   ${YELLOW}UFW firewall not installed${NC}"
        # Check iptables as alternative
        if command -v iptables &> /dev/null; then
            echo "   Checking iptables rules:"
            sudo iptables -L -n | grep -E "Chain|ACCEPT|DROP|REJECT"
        fi
    fi

    # Check if Nginx is running
    echo -e "\n   ${GREEN}Checking Nginx status:${NC}"
    if command -v nginx &> /dev/null; then
        sudo systemctl status nginx --no-pager || echo "   ${YELLOW}Nginx not running${NC}"
        # Check Nginx config
        echo -e "\n   ${GREEN}Validating Nginx configuration:${NC}"
        sudo nginx -t
    else
        echo "   ${YELLOW}Nginx not installed${NC}"
    fi
    
    # Check if the application service is running
    echo -e "\n   ${GREEN}Checking application service status:${NC}"
    sudo systemctl status $SERVICE_NAME --no-pager || echo "   ${YELLOW}Service $SERVICE_NAME not running${NC}"
    
    # Test local application connectivity
    echo -e "\n   ${GREEN}Testing local application connectivity:${NC}"
    if command -v curl &> /dev/null; then
        echo "   Testing localhost:"
        curl -s -o /dev/null -w "   Response Code: %{http_code}\n" http://localhost:$PORT || echo "   ${RED}Failed to connect to application locally${NC}"
        
        echo "   Testing loopback address:"
        curl -s -o /dev/null -w "   Response Code: %{http_code}\n" http://127.0.0.1:$PORT || echo "   ${RED}Failed to connect to application via loopback${NC}"
        
        # Try via hostname
        LOCAL_IP=$(hostname -I | awk '{print $1}')
        if [ ! -z "$LOCAL_IP" ]; then
            echo "   Testing via local IP ($LOCAL_IP):"
            curl -s -o /dev/null -w "   Response Code: %{http_code}\n" http://$LOCAL_IP:$PORT || echo "   ${RED}Failed to connect to application via local IP${NC}"
        fi
    else
        echo "   ${YELLOW}Cannot test connectivity (curl not installed)${NC}"
    fi
    
    # Get public IP
    PUBLIC_IP=$(curl -s https://api.ipify.org || curl -s http://checkip.amazonaws.com || echo "unknown")
    echo -e "\n   ${GREEN}Public IP address: $PUBLIC_IP${NC}"
    
    # Check open ports (using nmap if available)
    if command -v nmap &> /dev/null; then
        echo -e "\n   ${GREEN}Scanning for open ports:${NC}"
        echo -e "   (This may take a few seconds...)"
        NMAP_RESULT=$(nmap -T4 -p80,$PORT localhost)
        echo "$NMAP_RESULT" | grep -E "^[0-9]+\/tcp"
        
        # Scan externally visible ports if public IP is available
        if [ "$PUBLIC_IP" != "unknown" ]; then
            echo -e "\n   ${GREEN}Testing externally visible ports (from this machine):${NC}"
            echo -e "   (This may take a few seconds...)"
            EXTERNAL_SCAN=$(nmap -T4 -p80,$PORT $PUBLIC_IP)
            echo "$EXTERNAL_SCAN" | grep -E "^[0-9]+\/tcp"
        fi
    fi
    
    echo -e "\n${GREEN}Network diagnostics completed.${NC}"
}

# Function to test public internet connectivity
public_connectivity_test() {
    echo -e "${YELLOW}[*] Testing public internet connectivity...${NC}"
    
    PUBLIC_IP=$(curl -s https://api.ipify.org || curl -s http://checkip.amazonaws.com || echo "unknown")
    
    if [ "$PUBLIC_IP" == "unknown" ]; then
        echo -e "   ${RED}Could not determine public IP address.${NC}"
        echo -e "   Make sure this server has internet connectivity."
        return
    fi
    
    echo -e "   Public IP: $PUBLIC_IP"
    
    # Wait for services to be fully initialized
    echo -e "   Waiting for services to fully initialize (15 seconds)..."
    sleep 15
    
    # Test via different methods
    echo -e "   Testing multiple connectivity methods..."
    
    # Test via Nginx (port 80)
    echo -e "   1. Testing via HTTP (port 80):"
    if command -v curl &> /dev/null; then
        CURL_RESULT=$(curl -s -I "http://$PUBLIC_IP" 2>&1)
        HTTP_CODE=$(echo "$CURL_RESULT" | head -n 1 | cut -d' ' -f2)
        
        if [ -z "$HTTP_CODE" ]; then
            echo -e "   ${RED}Could not connect to http://$PUBLIC_IP${NC}"
            echo -e "   This could be due to firewall restrictions or Nginx not properly configured."
        elif [ "$HTTP_CODE" -ge 200 ] && [ "$HTTP_CODE" -lt 400 ]; then
            echo -e "   ${GREEN}✓${NC} Successfully connected to http://$PUBLIC_IP (HTTP $HTTP_CODE)"
        else
            echo -e "   ${RED}Received HTTP $HTTP_CODE response from http://$PUBLIC_IP${NC}"
        fi
    else
        echo -e "   ${YELLOW}Cannot test HTTP connectivity (curl not installed)${NC}"
    fi
    
    # Test direct application port
    echo -e "   2. Testing direct application port $PORT:"
    if command -v curl &> /dev/null; then
        CURL_RESULT=$(curl -s -I "http://$PUBLIC_IP:$PORT" 2>&1)
        if [[ "$CURL_RESULT" == *"Connection refused"* || "$CURL_RESULT" == *"Failed to connect"* ]]; then
            echo -e "   ${RED}Could not connect to http://$PUBLIC_IP:$PORT${NC}"
            echo -e "   This could be due to firewall restrictions or the application not binding correctly to external interfaces."
        else
            HTTP_CODE=$(echo "$CURL_RESULT" | head -n 1 | cut -d' ' -f2)
            if [ -z "$HTTP_CODE" ]; then
                echo -e "   ${RED}No HTTP response from http://$PUBLIC_IP:$PORT${NC}"
            elif [ "$HTTP_CODE" -ge 200 ] && [ "$HTTP_CODE" -lt 400 ]; then
                echo -e "   ${GREEN}✓${NC} Successfully connected to http://$PUBLIC_IP:$PORT (HTTP $HTTP_CODE)"
            else
                echo -e "   ${RED}Received HTTP $HTTP_CODE response from http://$PUBLIC_IP:$PORT${NC}"
            fi
        fi
    else
        echo -e "   ${YELLOW}Cannot test direct port connectivity (curl not installed)${NC}"
    fi
    
    # Test from external resources if possible (using external service)
    echo -e "   3. Testing external connectivity (Internet perspective):"
    if command -v curl &> /dev/null; then
        echo -e "   Checking port $PORT from external perspective..."
        # Use a service like portchecker.co to check if port is accessible from internet
        EXTERNAL_CHECK=$(curl -s "https://portchecker.co/check" --data "port=$PORT&ip=$PUBLIC_IP" | grep -o "Port $PORT is.*")
        
        if [[ "$EXTERNAL_CHECK" == *"open"* ]]; then
            echo -e "   ${GREEN}✓${NC} Port $PORT appears to be accessible from the internet"
        elif [[ "$EXTERNAL_CHECK" == *"closed"* ]]; then
            echo -e "   ${RED}×${NC} Port $PORT appears to be closed from the internet"
            echo -e "   This might be due to firewall restrictions or NAT configuration."
        else
            echo -e "   ${YELLOW}?${NC} Could not determine if port $PORT is accessible from the internet"
        fi
    fi
    
    echo -e "\n   Testing LAN connectivity..."
    # Get local IP
    LOCAL_IP=$(hostname -I | awk '{print $1}')
    if [ ! -z "$LOCAL_IP" ]; then
        echo -e "   Local IP: $LOCAL_IP"
        echo -e "   For LAN access, other devices on your network can use: http://$LOCAL_IP:$PORT"
    fi
    
    echo -e "\n${GREEN}Connectivity test completed.${NC}"
    
    # Provide summary
    echo -e "\n${YELLOW}CONNECTIVITY SUMMARY:${NC}"
    if [ "$PUBLIC_IP" != "unknown" ]; then
        echo -e "   • Public URL: http://$PUBLIC_IP"
        if command -v curl &> /dev/null; then
            if curl -s -I "http://$PUBLIC_IP" &>/dev/null; then
                echo -e "   ${GREEN}✓${NC} Nginx connection: WORKING"
            else
                echo -e "   ${RED}×${NC} Nginx connection: NOT WORKING"
            fi
            
            if curl -s -I "http://$PUBLIC_IP:$PORT" &>/dev/null; then
                echo -e "   ${GREEN}✓${NC} Direct port connection: WORKING"
            else
                echo -e "   ${RED}×${NC} Direct port connection: NOT WORKING"
            fi
        fi
    fi
    
    if [ ! -z "$LOCAL_IP" ]; then
        echo -e "   • Local URL: http://$LOCAL_IP:$PORT (for LAN access)"
    fi
    
    echo -e "\n   If you're having connectivity issues, check:"
    echo -e "   1. Firewall settings (both server and cloud provider)"
    echo -e "   2. Configuration binding (ensure server listens on 0.0.0.0)"
    echo -e "   3. Router/NAT configuration if behind a home network"
    echo -e "   4. Run network diagnostics again to troubleshoot further"
}

# Function to display connection information
display_connection_info() {
    echo -e "${YELLOW}[*] Your Go File Processor application is ready!${NC}"
    
    PUBLIC_IP=$(curl -s https://api.ipify.org || curl -s http://checkip.amazonaws.com || hostname -I | awk '{print $1}')
    
    echo -e "\n${GREEN}Connection Information:${NC}"
    echo -e "   Main application: http://$PUBLIC_IP"
    echo -e "   Direct port access: http://$PUBLIC_IP:$PORT"
    
    echo -e "\n${GREEN}Troubleshooting Tips:${NC}"
    echo -e "   • If you cannot access the application, check your firewall settings"
    echo -e "   • Ensure your cloud provider's security groups/firewall allow ports 80 and $PORT"
    echo -e "   • For AWS/GCP/Azure VMs, check your network ACLs and security groups"
    echo -e "   • Run 'sudo systemctl status $SERVICE_NAME' to check application status"
    echo -e "   • Run 'sudo systemctl status nginx' to check web server status"
    echo -e "   • View application logs: sudo cat $APP_DIR/logs/error.log"
    echo -e "   • View service logs: sudo journalctl -u $SERVICE_NAME -n 50"
    
    echo -e "\n${BLUE}============================================${NC}"
    echo -e "${BLUE}      Deployment Complete!     ${NC}"
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

# Function to ensure all required dependencies are installed
ensure_dependencies_installed() {
    echo -e "${YELLOW}[*] Ensuring all required dependencies are installed...${NC}"
    sudo apt-get update
    for pkg in curl wget ufw nginx lsof net-tools; do
        if ! dpkg -s $pkg &> /dev/null; then
            echo -e "   Installing $pkg..."
            sudo apt-get install -y $pkg
        else
            echo -e "   ${GREEN}✓${NC} $pkg already installed"
        fi
    done
}

# Function to check if port is in use and prompt for alternative
check_port_in_use() {
    # Try multiple methods to check for port usage
    PORT_IN_USE=false
    
    if command -v lsof &> /dev/null; then
        if lsof -i :$PORT | grep LISTEN &> /dev/null; then
            PORT_IN_USE=true
        fi
    elif command -v ss &> /dev/null; then
        if ss -tulpn | grep ":$PORT " &> /dev/null; then
            PORT_IN_USE=true
        fi
    elif command -v netstat &> /dev/null; then
        if netstat -tulpn | grep ":$PORT " &> /dev/null; then
            PORT_IN_USE=true
        fi
    fi
    
    if [ "$PORT_IN_USE" = true ]; then
        echo -e "${RED}Port $PORT is already in use.${NC}"
        read -p "Enter a different port (1024-65535) [default: $ALTERNATE_PORT]: " new_port
        if [[ ! -z "$new_port" && "$new_port" =~ ^[0-9]+$ && "$new_port" -ge 1024 && "$new_port" -le 65535 ]]; then
            PORT=$new_port
            echo -e "${GREEN}Port set to: $PORT${NC}"
        else
            PORT=$ALTERNATE_PORT
            echo -e "${GREEN}Port set to alternate: $PORT${NC}"
        fi
    else
        echo -e "${GREEN}Port $PORT is available.${NC}"
    fi
}

# Function to test public internet connectivity to the app
public_connectivity_test() {
    echo -e "${YELLOW}[11] Testing public internet connectivity...${NC}"
    PUBLIC_IP=$(curl -s https://api.ipify.org || curl -s http://checkip.amazonaws.com || echo "Could not determine")
    if [ "$PUBLIC_IP" = "Could not determine" ]; then
        echo -e "${RED}Could not determine public IP. Skipping public connectivity test.${NC}"
        return
    fi
    sleep 3
    HTTP_STATUS=$(curl -s -o /dev/null -w "%{http_code}" http://$PUBLIC_IP:$PORT/health || echo "Failed")
    if [ "$HTTP_STATUS" = "200" ]; then
        echo -e "${GREEN}✓ Application is reachable from the internet at http://$PUBLIC_IP:$PORT/health${NC}"
    else
        echo -e "${RED}✗ Application is NOT reachable from the internet at http://$PUBLIC_IP:$PORT/health (HTTP $HTTP_STATUS)${NC}"
        echo -e "Check firewall, VPS provider rules, and Nginx config."
    fi
}

# Main deployment process
main() {
    print_banner
    
    # Set configuration variables
    set_config_variables
    
    # Check system requirements
    check_system_requirements
    
    # Install required packages
    install_required_packages
    
    # Set up the application environment
    setup_app_environment

    # Configure and start Nginx
    configure_nginx
    
    # Configure the application as a service
    configure_service
    
    # Run network diagnostics to verify connectivity
    run_network_diagnostics
    
    # Test public connectivity
    public_connectivity_test
    
    # Display connection information and instructions
    display_connection_info
}

# Run the main function
main "$@"
