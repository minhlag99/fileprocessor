#!/bin/bash

# File Processor Deployment Script with Package Preservation
# This script deploys the application while preserving Go modules cache

# Color codes for better output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Application variables
APP_NAME="fileprocessor"
APP_DIR="$(pwd)"
PORT=8080
CONFIG_FILE="config/fileprocessor.json"

# Print the banner
print_banner() {
    echo -e "${YELLOW}"
    echo "╔════════════════════════════════════════════════════════╗"
    echo "║       File Processor Deployment (Package Saving)       ║"
    echo "╚════════════════════════════════════════════════════════╝"
    echo -e "${NC}"
}

# Function to verify Go installation
check_go_installation() {
    echo -e "${YELLOW}[1] Checking Go installation...${NC}"
    
    if command -v go &> /dev/null; then
        GO_VERSION=$(go version | awk '{print $3}' | sed 's/go//')
        echo -e "   Go $GO_VERSION detected"
        echo -e "   ${GREEN}✓${NC} Go is installed"
        
        # Display GOPATH information
        GOPATH=$(go env GOPATH)
        echo -e "   GOPATH: $GOPATH"
        GOMODCACHE=$(go env GOMODCACHE)
        echo -e "   Go module cache: $GOMODCACHE"
    else
        echo -e "   ${RED}✗${NC} Go is not installed"
        echo -e "   Please install Go before continuing"
        exit 1
    fi
}

# Function to build application without clearing module cache
build_application() {
    echo -e "${YELLOW}[2] Building the application...${NC}"
    cd $APP_DIR
    
    # Save current go.mod and go.sum before build
    echo -e "   Backing up dependency files..."
    cp go.mod go.mod.bak
    cp go.sum go.sum.bak
    
    # Run go mod tidy without clearing cache
    echo -e "   Running go mod tidy to ensure dependencies are correct..."
    go mod tidy
    
    # Compare dependency files, show changes if any
    if ! diff -q go.mod go.mod.bak &>/dev/null; then
        echo -e "   ${YELLOW}!${NC} Dependencies changed:"
        diff --color=auto go.mod go.mod.bak || true
    fi
    
    # Build with preserved module cache
    echo -e "   Building application..."
    go build -o $APP_NAME cmd/server/main.go
    
    if [ $? -eq 0 ]; then
        echo -e "   ${GREEN}✓${NC} Application built successfully"
    else
        echo -e "   ${RED}✗${NC} Build failed"
        echo -e "   Checking for common errors..."
        
        # Check for common errors
        if grep -q "cannot find package" build_output.log 2>/dev/null; then
            echo -e "   ${RED}Error:${NC} Missing packages. Try running: go mod download"
        elif grep -q "ambiguous import" build_output.log 2>/dev/null; then
            echo -e "   ${RED}Error:${NC} Ambiguous imports. Check your import paths."
        elif grep -q "package .* is not in GOROOT" build_output.log 2>/dev/null; then
            echo -e "   ${RED}Error:${NC} Package not found. Check if your module path in go.mod matches your import statements."
        fi
        
        return 1
    fi
    
    # Create necessary directories if they don't exist
    mkdir -p "$APP_DIR/uploads"
    mkdir -p "$APP_DIR/logs"
    mkdir -p "$APP_DIR/data/auth"
    
    echo -e "   ${GREEN}✓${NC} Application built and directories set up"
}

# Function to check import paths
check_import_paths() {
    echo -e "${YELLOW}[3] Checking import paths...${NC}"
    
    # Get the module name from go.mod
    MODULE_NAME=$(grep -m 1 "^module " $APP_DIR/go.mod | awk '{print $2}')
    echo -e "   Module name from go.mod: ${GREEN}$MODULE_NAME${NC}"
    
    # Check import paths in main.go
    MAIN_IMPORTS=$(grep -o "\"$MODULE_NAME/.*\"" "$APP_DIR/cmd/server/main.go" | wc -l)
    
    if [ $MAIN_IMPORTS -gt 0 ]; then
        echo -e "   ${GREEN}✓${NC} Main.go imports match module name"
    else
        echo -e "   ${RED}✗${NC} Import paths in main.go may not match module name"
        echo -e "   Imports in main.go should start with: \"$MODULE_NAME/...\""
    fi
    
    # Additional hint for fixing imports
    echo -e "   ${YELLOW}Hint:${NC} If you're having import issues, ensure imports use:"
    echo -e "   \"$MODULE_NAME/internal/...\" instead of \"fileprocessor/internal/...\""
}

# Function to run the application
run_application() {
    echo -e "${YELLOW}[4] Running the application...${NC}"
    
    # Check if port is available
    if lsof -i:$PORT -t &>/dev/null; then
        echo -e "   ${RED}✗${NC} Port $PORT is already in use"
        echo -e "   Please stop the process using this port first"
        return 1
    fi
    
    # Run the application
    echo -e "   Starting $APP_NAME on port $PORT..."
    ./$APP_NAME &
    PID=$!
    
    # Check if application is running
    sleep 2
    if ps -p $PID > /dev/null; then
        echo -e "   ${GREEN}✓${NC} Application is running with PID $PID"
    else
        echo -e "   ${RED}✗${NC} Failed to start application"
        return 1
    fi
    
    # Save PID to file
    echo $PID > $APP_NAME.pid
}

# Main script execution
print_banner
check_go_installation
check_import_paths
build_application || { echo -e "${RED}Build failed, exiting.${NC}"; exit 1; }
run_application || { echo -e "${RED}Failed to run application, exiting.${NC}"; exit 1; }

# Display success message and URL
echo -e "\n${GREEN}Deployment completed successfully!${NC}"
echo -e "Application running at: ${GREEN}http://localhost:$PORT${NC}"
echo -e "To stop the application, run: ${YELLOW}kill \$(cat $APP_NAME.pid)${NC}\n"