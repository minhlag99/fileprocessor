#!/bin/bash
# Network configuration helper for Go File Processor on Ubuntu VPS
# This script helps verify and configure network settings to make your application accessible

# Text colors for better readability
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo -e "${BLUE}========================================================${NC}"
echo -e "${BLUE}Network Configuration Helper for Go File Processor${NC}"
echo -e "${BLUE}========================================================${NC}"

# Define the application port
APP_PORT=8080

# Detect IP addresses
echo -e "\n${YELLOW}[1] Detecting IP addresses...${NC}"
echo "These are the IP addresses on your server:"

# Get loopback IP
LOOPBACK_IP="127.0.0.1"
echo -e "  - Loopback IP:     ${GREEN}$LOOPBACK_IP${NC} (accessible only from this machine)"

# Get private IP (internal network)
PRIVATE_IP=$(hostname -I | awk '{print $1}')
echo -e "  - Private IP:      ${GREEN}$PRIVATE_IP${NC} (accessible from your internal network)"

# Get public IP (internet-facing)
PUBLIC_IP=$(curl -s https://api.ipify.org || curl -s http://checkip.amazonaws.com || echo "Could not determine")
if [ "$PUBLIC_IP" != "Could not determine" ]; then
    echo -e "  - Public IP:       ${GREEN}$PUBLIC_IP${NC} (potentially accessible from the internet)"
else
    echo -e "  - Public IP:       ${RED}Could not determine${NC} (may be behind NAT or firewall)"
fi

# Check if the application is running
echo -e "\n${YELLOW}[2] Checking application status...${NC}"
if systemctl is-active --quiet go-fileprocessor; then
    echo -e "  - Application:     ${GREEN}RUNNING${NC}"
else
    echo -e "  - Application:     ${RED}NOT RUNNING${NC}"
    echo "    To start: sudo systemctl start go-fileprocessor"
fi

# Check if Nginx is installed and running
echo -e "\n${YELLOW}[3] Checking Nginx status...${NC}"
if command -v nginx &> /dev/null; then
    if systemctl is-active --quiet nginx; then
        echo -e "  - Nginx:           ${GREEN}RUNNING${NC}"
    else
        echo -e "  - Nginx:           ${RED}INSTALLED BUT NOT RUNNING${NC}"
        echo "    To start: sudo systemctl start nginx"
    fi
else
    echo -e "  - Nginx:           ${RED}NOT INSTALLED${NC}"
    echo "    To install: sudo apt-get update && sudo apt-get install -y nginx"
fi

# Check firewall status
echo -e "\n${YELLOW}[4] Checking firewall status...${NC}"
if command -v ufw &> /dev/null; then
    UFW_STATUS=$(sudo ufw status | grep "Status: " | awk '{print $2}')
    if [ "$UFW_STATUS" = "active" ]; then
        echo -e "  - Firewall:        ${GREEN}ACTIVE${NC}"
        
        # Check if ports are open
        HTTP_ALLOWED=$(sudo ufw status | grep "80/tcp" | grep "ALLOW" | wc -l)
        APP_PORT_ALLOWED=$(sudo ufw status | grep "$APP_PORT/tcp" | grep "ALLOW" | wc -l)
        
        if [ $HTTP_ALLOWED -gt 0 ]; then
            echo -e "    - Port 80:       ${GREEN}OPEN${NC}"
        else
            echo -e "    - Port 80:       ${RED}CLOSED${NC}"
            echo "      To open: sudo ufw allow 80/tcp"
        fi
        
        if [ $APP_PORT_ALLOWED -gt 0 ]; then
            echo -e "    - Port $APP_PORT:     ${GREEN}OPEN${NC}"
        else
            echo -e "    - Port $APP_PORT:     ${RED}CLOSED${NC}"
            echo "      To open: sudo ufw allow $APP_PORT/tcp"
        fi
    else
        echo -e "  - Firewall:        ${YELLOW}INACTIVE${NC}"
        echo "    This means all ports are potentially open, which may be a security risk."
        echo "    To enable: sudo ufw enable"
        echo "    First, make sure to allow SSH: sudo ufw allow 22/tcp"
    fi
else
    echo -e "  - Firewall:        ${YELLOW}NOT INSTALLED${NC}"
    echo "    To install: sudo apt-get update && sudo apt-get install -y ufw"
fi

# Check listening ports
echo -e "\n${YELLOW}[5] Checking listening ports...${NC}"
echo "  Testing if your application is listening on ports..."

if command -v netstat &> /dev/null; then
    if netstat -tuln | grep ":80 " &> /dev/null; then
        echo -e "  - Port 80:        ${GREEN}LISTENING${NC} (Nginx or web server)"
    else
        echo -e "  - Port 80:        ${RED}NOT LISTENING${NC} (Nginx might not be running)"
    fi
    
    if netstat -tuln | grep ":$APP_PORT " &> /dev/null; then
        echo -e "  - Port $APP_PORT:      ${GREEN}LISTENING${NC} (Your application)"
    else
        echo -e "  - Port $APP_PORT:      ${RED}NOT LISTENING${NC} (Your application might not be running)"
    fi
else
    echo -e "  ${RED}Cannot check listening ports (netstat not installed)${NC}"
    echo "    To install: sudo apt-get update && sudo apt-get install net-tools"
fi

# NAT/Port forwarding guidance
echo -e "\n${YELLOW}[6] VPS Provider Configuration${NC}"
echo "  Since you're using a VPS, you normally don't need to set up port forwarding"
echo "  as VPS providers typically assign public IPs directly to your server."
echo "  However, make sure your VPS provider doesn't have additional firewall rules"
echo "  blocking your ports. Check their control panel or dashboard."

# Would you like to open ports? (if closed)
echo -e "\n${YELLOW}[7] Would you like to open necessary ports now?${NC} (y/n)"
read -r OPEN_PORTS

if [[ $OPEN_PORTS == [Yy]* ]]; then
    if command -v ufw &> /dev/null; then
        echo "Opening ports 80 and $APP_PORT..."
        sudo ufw allow 80/tcp
        sudo ufw allow $APP_PORT/tcp
        echo -e "${GREEN}Ports opened successfully.${NC}"
    else
        echo -e "${RED}UFW firewall is not installed. Please install it first:${NC}"
        echo "sudo apt-get update && sudo apt-get install -y ufw"
    fi
fi

# Test accessibility
echo -e "\n${YELLOW}[8] Testing application accessibility...${NC}"

# Try to connect to the application
if command -v curl &> /dev/null; then
    echo "  Testing connection to your application..."
    HTTP_STATUS=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:$APP_PORT || echo "Connection failed")
    
    if [ "$HTTP_STATUS" = "200" ] || [ "$HTTP_STATUS" = "302" ] || [ "$HTTP_STATUS" = "301" ]; then
        echo -e "  - Local access:    ${GREEN}SUCCESS${NC} (HTTP Status: $HTTP_STATUS)"
    else
        echo -e "  - Local access:    ${RED}FAILED${NC} (HTTP Status: $HTTP_STATUS)"
        echo "    Make sure your application is running properly."
    fi
else
    echo -e "  ${RED}Cannot test connectivity (curl not installed)${NC}"
    echo "    To install: sudo apt-get update && sudo apt-get install curl"
fi

# Summary of access URLs
echo -e "\n${YELLOW}[9] Application access URLs${NC}"
echo -e "  - Local access:     ${GREEN}http://localhost:$APP_PORT${NC}"
echo -e "  - Internal network: ${GREEN}http://$PRIVATE_IP:$APP_PORT${NC}"

if [ "$PUBLIC_IP" != "Could not determine" ]; then
    echo -e "  - Internet access:  ${GREEN}http://$PUBLIC_IP:$APP_PORT${NC}"
fi

# If Nginx is running, also show port 80 URLs
if command -v nginx &> /dev/null && systemctl is-active --quiet nginx; then
    echo -e "  - Local access with Nginx:     ${GREEN}http://localhost${NC}"
    echo -e "  - Internal with Nginx:         ${GREEN}http://$PRIVATE_IP${NC}"
    
    if [ "$PUBLIC_IP" != "Could not determine" ]; then
        echo -e "  - Internet with Nginx:         ${GREEN}http://$PUBLIC_IP${NC}"
    fi
fi

echo -e "\n${BLUE}========================================================${NC}"
echo -e "${BLUE}Network configuration check complete.${NC}"
echo -e "${BLUE}If you still can't access your application from other machines,${NC}"
echo -e "${BLUE}please check the VPS provider's firewall settings.${NC}"
echo -e "${BLUE}========================================================${NC}"

# Offer to deploy or restart the application if it's not running
if ! systemctl is-active --quiet go-fileprocessor; then
    echo -e "\n${YELLOW}Your application doesn't seem to be running.${NC}"
    echo -e "Would you like to deploy/restart it now? (y/n)"
    read -r DEPLOY_APP

    if [[ $DEPLOY_APP == [Yy]* ]]; then
        if [ -f "./deploy_local.sh" ]; then
            echo "Running deployment script..."
            chmod +x ./deploy_local.sh
            sudo ./deploy_local.sh
        else
            echo -e "${RED}Deployment script (deploy_local.sh) not found in the current directory.${NC}"
            echo "Please run the deployment script manually."
        fi
    fi
fi