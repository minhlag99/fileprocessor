#!/bin/bash

# Quick start script for Go File Processor
# This script provides an easy way to start the application

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo -e "${BLUE}========================================${NC}"
echo -e "${BLUE}    Go File Processor Quick Start       ${NC}"
echo -e "${BLUE}========================================${NC}"
echo 

# Check if we're on Linux or WSL
if [[ "$(uname)" == "Linux" ]]; then
    echo -e "${YELLOW}Linux environment detected${NC}"
    PLATFORM="linux"
elif grep -qE "(Microsoft|WSL)" /proc/version &> /dev/null; then
    echo -e "${YELLOW}WSL environment detected${NC}"
    PLATFORM="wsl"
else
    echo -e "${RED}Unsupported platform. This script is designed for Linux or WSL.${NC}"
    echo -e "For Windows without WSL, please use start_windows.bat"
    exit 1
fi

# Check for Docker
if command -v docker &> /dev/null && command -v docker-compose &> /dev/null; then
    echo -e "${GREEN}Docker and Docker Compose detected${NC}"
    HAS_DOCKER=true
else
    echo -e "${YELLOW}Docker and/or Docker Compose not detected${NC}"
    HAS_DOCKER=false
fi

# Present options to the user
echo
echo "How would you like to run the application?"
echo "1) Standard mode (native Go)"
echo "2) Docker mode (using Docker & Docker Compose)"
echo "3) Deploy to VPS (using unified deployment script)"
echo "q) Quit"
echo

read -p "Enter your choice: " CHOICE

case $CHOICE in
    1)
        echo -e "${YELLOW}Starting in standard mode...${NC}"
        
        # Check for Go
        if ! command -v go &> /dev/null; then
            echo -e "${RED}Go is not installed. Please install Go first.${NC}"
            exit 1
        fi
        
        # Build and run
        go build -o fileserver ./cmd/server/main.go
        echo -e "${GREEN}Application built successfully${NC}"
        echo -e "${GREEN}Starting server on port 8080...${NC}"
        ./fileserver --port=8080 --ui=./ui --uploads=./uploads
        ;;
        
    2)
        if [ "$HAS_DOCKER" = false ]; then
            echo -e "${RED}Docker and/or Docker Compose not installed.${NC}"
            echo -e "Please install Docker and Docker Compose first."
            exit 1
        fi
        
        echo -e "${YELLOW}Starting in Docker mode...${NC}"
        docker-compose up --build -d
        echo -e "${GREEN}Application started in Docker containers${NC}"
        echo -e "You can access it at: http://localhost"
        echo -e "To stop it, run: docker-compose down"
        ;;
        
    3)
        echo -e "${YELLOW}Preparing for VPS deployment...${NC}"
        
        # Check if the deployment script exists
        if [ ! -f "./deploy_unified.sh" ]; then
            echo -e "${RED}Deployment script not found.${NC}"
            exit 1
        fi
        
        # Make the script executable
        chmod +x ./deploy_unified.sh
        
        echo -e "${YELLOW}This script should be run on the VPS itself.${NC}"
        echo -e "${YELLOW}Copy the entire project to your VPS and run deploy_unified.sh there.${NC}"
        echo
        echo -e "${BLUE}Instructions:${NC}"
        echo "1. Copy the project to your VPS:"
        echo "   scp -r . username@your-vps-address:/path/to/destination"
        echo
        echo "2. SSH into your VPS:"
        echo "   ssh username@your-vps-address"
        echo
        echo "3. Navigate to the project directory and run the deployment script:"
        echo "   cd /path/to/destination"
        echo "   chmod +x deploy_unified.sh"
        echo "   ./deploy_unified.sh"
        ;;
        
    q|Q)
        echo -e "${BLUE}Exiting...${NC}"
        exit 0
        ;;
        
    *)
        echo -e "${RED}Invalid option${NC}"
        exit 1
        ;;
esac