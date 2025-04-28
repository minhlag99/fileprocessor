#!/bin/bash
# Network setup script for fileprocessor VPS deployment
# This script helps ensure your application is accessible from the internet

# Configuration
APP_PORT=8080
APP_USER=$(whoami)

echo "=== File Processor Network Configuration ==="
echo "Performing network setup checks and configuration..."

# Check if we're running as root
if [ "$EUID" -ne 0 ]; then 
  echo "This script needs to be run with sudo privileges."
  exit 1
fi

# 1. Check if the port is properly open in the firewall
echo "Checking and configuring firewall..."
if command -v ufw &> /dev/null; then
  # Ubuntu's Uncomplicated Firewall
  ufw allow $APP_PORT/tcp
  echo "UFW: Port $APP_PORT opened for TCP connections"
elif command -v firewalld &> /dev/null; then
  # Firewalld (Red Hat, CentOS, Fedora)
  firewall-cmd --permanent --add-port=$APP_PORT/tcp
  firewall-cmd --reload
  echo "FirewallD: Port $APP_PORT opened for TCP connections"
else
  echo "No recognized firewall system found. Please manually verify firewall settings."
fi

# 2. Check if the application is listening on all interfaces (0.0.0.0)
echo "Checking listening interfaces..."
if command -v netstat &> /dev/null; then
  LISTENING=$(netstat -tulpn | grep -E ":$APP_PORT\s")
elif command -v ss &> /dev/null; then
  LISTENING=$(ss -tulpn | grep -E ":$APP_PORT\s")
else
  echo "Neither netstat nor ss found. Cannot check listening interfaces."
  LISTENING=""
fi

if [ ! -z "$LISTENING" ]; then
  if echo "$LISTENING" | grep -q "0.0.0.0"; then
    echo "Application is correctly listening on all interfaces (0.0.0.0:$APP_PORT)"
  else
    echo "WARNING: Application appears to be listening only on specific interfaces:"
    echo "$LISTENING"
    echo "Make sure your application has 'host': '0.0.0.0' in its configuration."
  fi
else
  echo "WARNING: Application does not appear to be listening on port $APP_PORT."
  echo "Check if the service is running."
fi

# 3. Check for cloud provider specific network settings
CLOUD_PROVIDER="unknown"
if [ -f /sys/hypervisor/uuid ] && grep -q -i amazon /sys/hypervisor/uuid; then
  CLOUD_PROVIDER="aws"
elif curl -s metadata.google.internal > /dev/null 2>&1; then
  CLOUD_PROVIDER="gcp"
elif curl -s -H Metadata:true "http://169.254.169.254/metadata/instance?api-version=2021-02-01" > /dev/null 2>&1; then
  CLOUD_PROVIDER="azure" 
fi

echo "Detected cloud provider: $CLOUD_PROVIDER"

case $CLOUD_PROVIDER in
  "aws")
    echo "AWS detected. Please ensure Security Group allows inbound traffic on port $APP_PORT."
    echo "You can check this through the AWS Console under EC2 > Security Groups"
    ;;
  "gcp") 
    echo "GCP detected. Please ensure Firewall Rules allow inbound traffic on port $APP_PORT."
    echo "You can check this through the GCP Console under VPC Network > Firewall Rules"
    ;;
  "azure")
    echo "Azure detected. Please ensure Network Security Group allows inbound traffic on port $APP_PORT."
    echo "You can check this through the Azure Portal under Virtual Machine > Networking"
    ;;
  *)
    echo "Standard VPS detected. No additional cloud provider configuration needed."
    ;;
esac

# 4. Verify that nothing else is blocking the port
echo "Testing local port accessibility..."
if command -v nc &> /dev/null; then
  if nc -z localhost $APP_PORT; then
    echo "Port $APP_PORT is accessible locally."
  else
    echo "WARNING: Port $APP_PORT is not accessible locally. Check if the service is running."
  fi
else
  echo "Netcat (nc) not found. Cannot test local port accessibility."
fi

# 5. Print helpful information
echo "=== Complete Network Setup Information ==="
echo "Application port: $APP_PORT"
echo "External IP addresses:"
ip -4 addr show | grep -oP '(?<=inet\s)\d+(\.\d+){3}' | grep -v "127.0.0.1"

echo "Network setup complete!"
echo ""
echo "Verify connectivity with: curl http://YOUR-SERVER-IP:$APP_PORT/health"
echo "If you still can't connect from outside, check the following:"
echo "1. Cloud provider firewall/security group settings"
echo "2. Application configuration to ensure it binds to 0.0.0.0"
echo "3. Proper forwarding if behind a load balancer or proxy"