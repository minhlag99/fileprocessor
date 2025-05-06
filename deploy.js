#!/usr/bin/env node

/**
 * Universal Deployment Script for Go File Processor
 * Works on both Windows and Linux/Unix systems
 * 
 * Usage:
 * - On Windows: node deploy.js
 * - On Linux/Unix: chmod +x deploy.js && ./deploy.js
 */

const fs = require('fs');
const path = require('path');
const { execSync } = require('child_process');
const os = require('os');
const readline = require('readline');

// ANSI color codes (works in modern Windows terminals and Unix)
const colors = {
  reset: '\x1b[0m',
  red: '\x1b[31m',
  green: '\x1b[32m',
  yellow: '\x1b[33m',
  blue: '\x1b[34m',
  magenta: '\x1b[35m',
  cyan: '\x1b[36m'
};

// Detect OS
const isWindows = os.platform() === 'win32';
console.log(`${colors.blue}Detected OS: ${isWindows ? 'Windows' : 'Linux/Unix'}${colors.reset}`);

// Load environment variables from .env file
function loadEnvVars() {
  try {
    const envPath = path.join(process.cwd(), '.env');
    if (fs.existsSync(envPath)) {
      console.log(`${colors.green}Loading environment variables from .env file${colors.reset}`);
      const envContent = fs.readFileSync(envPath, 'utf8');
      const envLines = envContent.split('\n');
      
      for (const line of envLines) {
        const trimmedLine = line.trim();
        if (trimmedLine && !trimmedLine.startsWith('#')) {
          const parts = trimmedLine.split('=');
          if (parts.length >= 2) {
            const key = parts[0].trim();
            // Join all parts after the first '=' in case value contains '='
            const value = parts.slice(1).join('=').trim();
            // Remove quotes if present
            const cleanValue = value.replace(/^["'](.*)["']$/, '$1');
            
            // Process environment variables in values (${VAR} syntax)
            let processedValue = cleanValue;
            const varRegex = /\${([^}]+)}/g;
            let match;
            while ((match = varRegex.exec(cleanValue)) !== null) {
              const envVar = match[1];
              const envValue = process.env[envVar] || '';
              processedValue = processedValue.replace(`\${${envVar}}`, envValue);
            }
            
            process.env[key] = processedValue;
          }
        }
      }
      console.log(`${colors.green}✓ Loaded environment variables from .env file${colors.reset}`);
    } else {
      console.log(`${colors.yellow}No .env file found, using defaults${colors.reset}`);
      setupDefaultEnv();
    }
    
    // Load environment-specific configuration
    loadEnvironmentConfig();
  } catch (error) {
    console.error(`${colors.red}Error loading .env file: ${error.message}${colors.reset}`);
    setupDefaultEnv();
  }
}

// Load environment-specific configuration
function loadEnvironmentConfig() {
  const environment = process.env.ENVIRONMENT || 'development';
  console.log(`${colors.blue}Loading ${environment} environment configuration${colors.reset}`);
  
  try {
    // Determine which config file to use
    let configFile = '';
    switch (environment.toLowerCase()) {
      case 'production':
      case 'prod':
        configFile = 'env.prod.json';
        break;
      case 'staging':
      case 'stage':
        configFile = 'env.staging.json';
        break;
      case 'development':
      case 'dev':
      default:
        configFile = 'env.dev.json';
        break;
    }
    
    const configPath = path.join(process.cwd(), 'config', configFile);
    if (fs.existsSync(configPath)) {
      const configContent = fs.readFileSync(configPath, 'utf8');
      const config = JSON.parse(configContent);
      
      // Set environment variables from config
      process.env.PORT = config.server.port.toString();
      process.env.WORKER_COUNT = config.workers.count.toString();
      process.env.ENABLE_MEDIA_PREVIEW = config.features.enableMediaPreview.toString();
      
      console.log(`${colors.green}✓ Loaded configuration from ${configFile}${colors.reset}`);
    } else {
      console.log(`${colors.yellow}⚠ Configuration file ${configFile} not found${colors.reset}`);
    }
  } catch (error) {
    console.error(`${colors.red}Error loading environment configuration: ${error.message}${colors.reset}`);
  }
}

// Set up default environment variables if .env file not found
function setupDefaultEnv() {
  process.env.APP_NAME = process.env.APP_NAME || 'go-fileprocessor';
  process.env.PORT = process.env.PORT || '9000';
  process.env.ALTERNATE_PORT = process.env.ALTERNATE_PORT || '9001';
  process.env.LATEST_GO_VERSION = process.env.LATEST_GO_VERSION || '1.24.2';
  
  if (isWindows) {
    process.env.APP_DIR = process.env.APP_DIR || path.join(os.homedir(), 'go-fileprocessor');
  } else {
    process.env.APP_DIR = process.env.APP_DIR || '/opt/go-fileprocessor';
    process.env.SERVICE_NAME = process.env.SERVICE_NAME || 'go-fileprocessor.service';
  }
}

// Create an interactive command line interface
const rl = readline.createInterface({
  input: process.stdin,
  output: process.stdout
});

function question(query) {
  return new Promise(resolve => {
    rl.question(query, resolve);
  });
}

// Main function
async function main() {
  try {
    console.log(`${colors.blue}============================================${colors.reset}`);
    console.log(`${colors.blue}      Go File Processor Deployment Tool     ${colors.reset}`);
    console.log(`${colors.blue}============================================${colors.reset}`);

    // Load environment variables
    loadEnvVars();
    
    // Choose deployment mode
    console.log(`\n${colors.yellow}Select deployment mode:${colors.reset}`);
    console.log('1) Local deployment (deploy on this machine)');
    console.log('2) Remote deployment via SSH (deploy to VPS)');
    console.log('3) Direct server deployment (for Remote Desktop/Console access)');
    console.log('4) Run network diagnostics only');
    console.log('q) Quit');

    const mode = await question('Enter your choice (1, 2, 3, 4, q): ');
    
    switch (mode) {
      case '1':
        console.log(`\n${colors.green}Selected: Local deployment${colors.reset}`);
        await deployLocal();
        break;
      case '2':
        console.log(`\n${colors.green}Selected: Remote deployment via SSH${colors.reset}`);
        await setupRemoteConfig();
        await deployRemote();
        break;
      case '3':
        console.log(`\n${colors.green}Selected: Direct server deployment${colors.reset}`);
        await deployDirectServer();
        break;
      case '4':
        console.log(`\n${colors.green}Selected: Network diagnostics${colors.reset}`);
        runNetworkDiagnostics();
        break;
      case 'q':
      case 'Q':
        console.log(`\n${colors.blue}Exiting deployment tool.${colors.reset}`);
        process.exit(0);
        break;
      default:
        console.log(`\n${colors.red}Invalid option.${colors.reset}`);
        await main();
        return;
    }
  } catch (error) {
    console.error(`${colors.red}Error: ${error.message}${colors.reset}`);
    process.exit(1);
  } finally {
    rl.close();
  }
}

// Configure ports
async function configurePorts() {
  console.log(`\n${colors.yellow}Port Configuration${colors.reset}`);
  console.log(`Default application port is ${process.env.PORT}`);
  
  const changePort = await question('Do you want to use a different port? (y/n): ');
  if (changePort.toLowerCase() === 'y') {
    const newPort = await question(`Enter new port number (1024-65535) [default: ${process.env.ALTERNATE_PORT}]: `);
    if (newPort) {
      const portNum = parseInt(newPort);
      if (!isNaN(portNum) && portNum >= 1024 && portNum <= 65535) {
        process.env.PORT = newPort;
        console.log(`${colors.green}Port set to: ${process.env.PORT}${colors.reset}`);
      } else {
        console.log(`${colors.red}Invalid port. Using default port: ${process.env.PORT}${colors.reset}`);
      }
    } else {
      process.env.PORT = process.env.ALTERNATE_PORT;
      console.log(`${colors.green}Port set to alternate: ${process.env.PORT}${colors.reset}`);
    }
  }
}

// Create directories
function createDirectories() {
  console.log(`${colors.yellow}[1] Creating application directories...${colors.reset}`);
  
  const appDir = process.env.APP_DIR;
  const dirs = [
    appDir,
    path.join(appDir, 'uploads'),
    path.join(appDir, 'ui'),
    path.join(appDir, 'logs'),
    path.join(appDir, 'config')
  ];
  
  for (const dir of dirs) {
    if (!fs.existsSync(dir)) {
      fs.mkdirSync(dir, { recursive: true });
    }
  }
  
  console.log(`   ${colors.green}✓${colors.reset} Directories created`);
}

// Copy files
function copyFiles() {
  console.log(`${colors.yellow}[2] Copying application files...${colors.reset}`);
  
  const currentDir = process.cwd();
  const appDir = process.env.APP_DIR;
  
  // Helper function to copy directory recursively
  function copyDir(src, dest) {
    if (!fs.existsSync(dest)) {
      fs.mkdirSync(dest, { recursive: true });
    }
    
    const entries = fs.readdirSync(src);
    
    for (const entry of entries) {
      const srcPath = path.join(src, entry);
      const destPath = path.join(dest, entry);
      
      const stat = fs.statSync(srcPath);
      
      if (stat.isDirectory()) {
        copyDir(srcPath, destPath);
      } else {
        fs.copyFileSync(srcPath, destPath);
      }
    }
  }
  
  // Copy directories
  if (fs.existsSync(path.join(currentDir, 'cmd'))) {
    copyDir(path.join(currentDir, 'cmd'), path.join(appDir, 'cmd'));
  }
  
  if (fs.existsSync(path.join(currentDir, 'internal'))) {
    copyDir(path.join(currentDir, 'internal'), path.join(appDir, 'internal'));
  }
  
  if (fs.existsSync(path.join(currentDir, 'config'))) {
    copyDir(path.join(currentDir, 'config'), path.join(appDir, 'config'));
  }
  
  if (fs.existsSync(path.join(currentDir, 'ui'))) {
    copyDir(path.join(currentDir, 'ui'), path.join(appDir, 'ui'));
  }
  
  // Copy individual files
  const filesToCopy = ['go.mod', 'go.sum'];
  for (const file of filesToCopy) {
    if (fs.existsSync(path.join(currentDir, file))) {
      fs.copyFileSync(path.join(currentDir, file), path.join(appDir, file));
    }
  }
  
  // Copy config files
  if (fs.existsSync(path.join(currentDir, 'fileprocessor.ini'))) {
    fs.copyFileSync(
      path.join(currentDir, 'fileprocessor.ini'), 
      path.join(appDir, 'fileprocessor.ini')
    );
  }
  
  if (fs.existsSync(path.join(currentDir, 'config/fileprocessor.json'))) {
    fs.copyFileSync(
      path.join(currentDir, 'config/fileprocessor.json'), 
      path.join(appDir, 'fileprocessor.json')
    );
  } else if (fs.existsSync(path.join(currentDir, 'fileprocessor.json'))) {
    fs.copyFileSync(
      path.join(currentDir, 'fileprocessor.json'), 
      path.join(appDir, 'fileprocessor.json')
    );
  } else {
    createJsonConfig();
  }
  
  console.log(`   ${colors.green}✓${colors.reset} Files copied`);
  
  // Set permissions on Unix systems
  if (!isWindows) {
    try {
      execSync(`chmod -R 755 ${appDir}`);
      console.log(`   ${colors.green}✓${colors.reset} Permissions set`);
    } catch (error) {
      console.error(`   ${colors.red}Error setting permissions: ${error.message}${colors.reset}`);
    }
  }
}

// Create JSON config
function createJsonConfig() {
  console.log("   Creating default JSON configuration file...");
  
  const config = {
    server: {
      port: parseInt(process.env.PORT),
      uiDir: "./ui",
      uploadsDir: "./uploads",
      workerCount: 4,
      enableLan: true,
      shutdownTimeout: 30,
      host: "0.0.0.0"
    },
    storage: {
      defaultProvider: "local",
      local: {
        basePath: "./uploads"
      },
      s3: {
        region: "",
        bucket: "",
        accessKey: "",
        secretKey: "",
        prefix: ""
      },
      google: {
        bucket: "",
        credentialFile: "",
        prefix: ""
      }
    },
    workers: {
      count: 4,
      queueSize: 100,
      maxAttempts: 3
    },
    features: {
      enableLAN: true,
      enableProcessing: true,
      enableCloudStorage: false,
      enableProgressUpdates: true,
      enableMediaPreview: process.env.ENABLE_MEDIA_PREVIEW === 'true'
    },
    ssl: {
      enable: false,
      certFile: "",
      keyFile: ""
    }
  };
  
  fs.writeFileSync(
    path.join(process.env.APP_DIR, 'fileprocessor.json'),
    JSON.stringify(config, null, 2)
  );
  
  console.log(`   ${colors.green}✓${colors.reset} Created default JSON configuration file with port ${process.env.PORT}`);
}

// Update config files with current settings
function updateConfigFile() {
  console.log(`${colors.yellow}[3] Updating configuration files...${colors.reset}`);
  
  const appDir = process.env.APP_DIR;
  const port = process.env.PORT;
  
  // Update JSON config
  const jsonPath = path.join(appDir, 'fileprocessor.json');
  if (fs.existsSync(jsonPath)) {
    try {
      const config = JSON.parse(fs.readFileSync(jsonPath, 'utf8'));
      config.server.port = parseInt(port);
      config.server.host = '0.0.0.0';
      config.features.enableLAN = true;
      config.features.enableMediaPreview = process.env.ENABLE_MEDIA_PREVIEW === 'true';
      
      fs.writeFileSync(jsonPath, JSON.stringify(config, null, 2));
      console.log(`   ${colors.green}✓${colors.reset} Updated JSON configuration with port ${port}`);
    } catch (error) {
      console.error(`   ${colors.red}Error updating JSON config: ${error.message}${colors.reset}`);
      createJsonConfig();
    }
  } else {
    createJsonConfig();
  }
  
  // Update INI config if it exists (for backward compatibility)
  const iniPath = path.join(appDir, 'fileprocessor.ini');
  if (fs.existsSync(iniPath)) {
    try {
      let iniContent = fs.readFileSync(iniPath, 'utf8');
      
      // Update port
      iniContent = iniContent.replace(/port = [0-9]*/g, `port = ${port}`);
      
      // Update enable_lan
      iniContent = iniContent.replace(/enable_lan = false/g, 'enable_lan = true');
      
      // Update host
      if (iniContent.includes('host =')) {
        iniContent = iniContent.replace(/host = .*/g, 'host = 0.0.0.0');
      } else if (iniContent.includes('[server]')) {
        iniContent = iniContent.replace(/\[server\]/g, '[server]\nhost = 0.0.0.0');
      }
      
      fs.writeFileSync(iniPath, iniContent);
      console.log(`   ${colors.green}✓${colors.reset} Updated INI configuration with port ${port}`);
    } catch (error) {
      console.error(`   ${colors.red}Error updating INI config: ${error.message}${colors.reset}`);
    }
  } else {
    console.log(`   ${colors.yellow}!${colors.reset} No INI configuration file found`);
  }
  
  console.log(`   ${colors.green}✓${colors.reset} Configuration updated to allow external connections`);
}

// Install Go
async function installGo() {
  console.log(`${colors.yellow}[3] Checking Go installation...${colors.reset}`);
  
  try {
    // Check if Go is installed
    let goInstalled = false;
    let goVersion = '';
    
    try {
      const output = execSync('go version', { encoding: 'utf8' });
      goInstalled = true;
      goVersion = output.match(/go(\d+\.\d+\.\d+)/)[1];
      console.log(`   Go ${goVersion} detected`);
    } catch (error) {
      console.log(`   Go is not installed`);
    }
    
    if (!goInstalled) {
      console.log(`   Installing Go ${process.env.LATEST_GO_VERSION}...`);
      
      if (isWindows) {
        // Windows installation
        const goMsi = `go${process.env.LATEST_GO_VERSION}.windows-amd64.msi`;
        const goUrl = `https://go.dev/dl/${goMsi}`;
        
        console.log(`   Downloading Go from ${goUrl}`);
        execSync(`powershell -Command "Invoke-WebRequest -Uri ${goUrl} -OutFile ${goMsi}"`, { stdio: 'inherit' });
        
        console.log('   Installing Go...');
        execSync(`msiexec /i ${goMsi} /quiet`, { stdio: 'inherit' });
        
        // Update PATH for current session
        const goPath = 'C:\\Program Files\\Go\\bin';
        process.env.PATH = `${goPath};${process.env.PATH}`;
        
        // Clean up
        fs.unlinkSync(goMsi);
      } else {
        // Linux/Unix installation
        const goTar = `go${process.env.LATEST_GO_VERSION}.linux-amd64.tar.gz`;
        const goUrl = `https://go.dev/dl/${goTar}`;
        
        console.log(`   Downloading Go from ${goUrl}`);
        execSync(`wget -q ${goUrl}`, { stdio: 'inherit' });
        
        console.log('   Installing Go...');
        execSync(`sudo rm -rf /usr/local/go && sudo tar -C /usr/local -xzf ${goTar}`, { stdio: 'inherit' });
        
        // Add Go to PATH
        process.env.PATH = `/usr/local/go/bin:${process.env.PATH}`;
        
        // Update ~/.bashrc and ~/.profile
        const homeDir = os.homedir();
        const bashrcPath = path.join(homeDir, '.bashrc');
        const profilePath = path.join(homeDir, '.profile');
        
        for (const rcPath of [bashrcPath, profilePath]) {
          if (fs.existsSync(rcPath)) {
            let rcContent = fs.readFileSync(rcPath, 'utf8');
            if (!rcContent.includes('export PATH=$PATH:/usr/local/go/bin')) {
              fs.appendFileSync(rcPath, '\nexport PATH=$PATH:/usr/local/go/bin\n');
            }
          }
        }
        
        // Clean up
        fs.unlinkSync(goTar);
      }
      
      console.log(`   ${colors.green}✓${colors.reset} Go ${process.env.LATEST_GO_VERSION} installed`);
      
      // Verify installation
      try {
        const output = execSync('go version', { encoding: 'utf8' });
        goVersion = output.match(/go(\d+\.\d+\.\d+)/)[1];
        console.log(`   Using Go version: ${goVersion}`);
      } catch (error) {
        console.error(`   ${colors.red}Go installation verification failed: ${error.message}${colors.reset}`);
      }
    } else {
      // Check if we should upgrade
      const latestVersion = process.env.LATEST_GO_VERSION;
      
      // Simple version comparison
      const versionParts = goVersion.split('.').map(Number);
      const latestParts = latestVersion.split('.').map(Number);
      
      let shouldUpgrade = false;
      for (let i = 0; i < versionParts.length; i++) {
        if (versionParts[i] < latestParts[i]) {
          shouldUpgrade = true;
          break;
        } else if (versionParts[i] > latestParts[i]) {
          break;
        }
      }
      
      if (shouldUpgrade) {
        console.log(`   ${colors.yellow}⚠${colors.reset} Upgrading Go from ${goVersion} to ${latestVersion}...`);
        
        if (isWindows) {
          // Windows upgrade
          const goMsi = `go${latestVersion}.windows-amd64.msi`;
          const goUrl = `https://go.dev/dl/${goMsi}`;
          
          execSync(`powershell -Command "Invoke-WebRequest -Uri ${goUrl} -OutFile ${goMsi}"`, { stdio: 'inherit' });
          execSync(`msiexec /i ${goMsi} /quiet`, { stdio: 'inherit' });
          
          // Clean up
          fs.unlinkSync(goMsi);
        } else {
          // Linux/Unix upgrade
          const goTar = `go${latestVersion}.linux-amd64.tar.gz`;
          const goUrl = `https://go.dev/dl/${goTar}`;
          
          execSync(`wget -q ${goUrl}`, { stdio: 'inherit' });
          execSync(`sudo rm -rf /usr/local/go && sudo tar -C /usr/local -xzf ${goTar}`, { stdio: 'inherit' });
          
          // Clean up
          fs.unlinkSync(goTar);
        }
        
        console.log(`   ${colors.green}✓${colors.reset} Go upgraded to ${latestVersion}`);
      } else {
        console.log(`   ${colors.green}✓${colors.reset} Already using latest Go ${goVersion}`);
      }
    }
    
    // Check go.mod version
    const goModPath = path.join(process.env.APP_DIR, 'go.mod');
    if (fs.existsSync(goModPath)) {
      const goModContent = fs.readFileSync(goModPath, 'utf8');
      const modVersionMatch = goModContent.match(/^go (\d+\.\d+)/m);
      if (modVersionMatch) {
        console.log(`   Go version in go.mod: ${modVersionMatch[1]}`);
      }
    }
    
    // Set environment variables
    if (!isWindows) {
      process.env.GOROOT = '/usr/local/go';
      process.env.GOPATH = path.join(os.homedir(), 'go');
      process.env.PATH = `${process.env.GOROOT}/bin:${process.env.GOPATH}/bin:${process.env.PATH}`;
    }
    
  } catch (error) {
    console.error(`   ${colors.red}Error during Go installation: ${error.message}${colors.reset}`);
    throw new Error('Go installation failed');
  }
}

// Build application
async function buildApplication() {
  console.log(`${colors.yellow}[4] Building the application...${colors.reset}`);
  
  try {
    const appDir = process.env.APP_DIR;
    process.chdir(appDir);
    
    // Clear module cache
    console.log('   Clearing module cache...');
    execSync('go clean -modcache', { stdio: 'inherit' });
    
    // Tidy modules
    console.log('   Running go mod tidy to ensure dependencies are correct...');
    execSync('go mod tidy', { stdio: 'inherit' });
    
    // Build application
    console.log('   Building application...');
    execSync(`go build -o ${process.env.APP_NAME} cmd/server/main.go`, { stdio: 'inherit' });
    
    console.log(`   ${colors.green}✓${colors.reset} Application built successfully`);
  } catch (error) {
    console.error(`   ${colors.red}✗${colors.reset} Build failed`);
    console.error(`   ${colors.red}Error: ${error.message}${colors.reset}`);
    
    // Check for common build errors
    const errorLog = path.join(process.env.APP_DIR, 'logs', 'error.log');
    if (fs.existsSync(errorLog)) {
      const logContent = fs.readFileSync(errorLog, 'utf8');
      
      if (logContent.includes('address already in use')) {
        console.error(`   ${colors.red}Error:${colors.reset} Port ${process.env.PORT} is already in use.`);
        console.error(`   Solution: Either stop the process using port ${process.env.PORT} or change the port in your configuration.`);
      } else if (logContent.includes('permission denied')) {
        console.error(`   ${colors.red}Error:${colors.reset} Permission issues detected.`);
        console.error(`   Solution: Check file permissions in ${process.env.APP_DIR}`);
      }
    }
    
    console.error('   You may need to manually fix dependency issues or check service logs for details.');
    throw new Error('Build failed');
  }
}

// Create systemd service (Unix only)
function createService() {
  if (isWindows) {
    console.log(`${colors.yellow}[5] Skipping systemd service creation on Windows${colors.reset}`);
    return;
  }
  
  console.log(`${colors.yellow}[5] Creating systemd service...${colors.reset}`);
  
  try {
    const serviceName = process.env.SERVICE_NAME;
    const appDir = process.env.APP_DIR;
    const username = os.userInfo().username;
    
    const serviceContent = `[Unit]
Description=Go File Processor Service
After=network.target

[Service]
Type=simple
User=${username}
WorkingDirectory=${appDir}
ExecStart=${appDir}/${process.env.APP_NAME} --config=${appDir}/fileprocessor.json
Restart=always
RestartSec=3
StandardOutput=append:${appDir}/logs/output.log
StandardError=append:${appDir}/logs/error.log
Environment="PORT=${process.env.PORT}"
Environment="HOST=0.0.0.0"

[Install]
WantedBy=multi-user.target
`;
    
    const tempServicePath = path.join(os.tmpdir(), serviceName);
    fs.writeFileSync(tempServicePath, serviceContent);
    
    execSync(`sudo mv ${tempServicePath} /etc/systemd/system/`, { stdio: 'inherit' });
    execSync('sudo systemctl daemon-reload', { stdio: 'inherit' });
    execSync(`sudo systemctl enable ${serviceName}`, { stdio: 'inherit' });
    
    console.log(`   ${colors.green}✓${colors.reset} Service created and enabled`);
  } catch (error) {
    console.error(`   ${colors.red}Error creating service: ${error.message}${colors.reset}`);
  }
}

// Configure firewall (Unix only)
function configureFirewall() {
  if (isWindows) {
    console.log(`${colors.yellow}[6] Windows Firewall configuration...${colors.reset}`);
    try {
      // Add firewall rule for the port
      execSync(`netsh advfirewall firewall add rule name="Go File Processor" dir=in action=allow protocol=TCP localport=${process.env.PORT}`, 
        { stdio: 'inherit' });
      console.log(`   ${colors.green}✓${colors.reset} Windows Firewall rule added for port ${process.env.PORT}`);
    } catch (error) {
      console.error(`   ${colors.red}Error configuring Windows Firewall: ${error.message}${colors.reset}`);
      console.error(`   You may need to run as Administrator to configure the firewall`);
    }
    return;
  }
  
  console.log(`${colors.yellow}[6] Configuring firewall...${colors.reset}`);
  
  try {
    // Check if ufw is installed
    let ufwInstalled = false;
    try {
      execSync('command -v ufw', { stdio: 'ignore' });
      ufwInstalled = true;
    } catch (error) {
      console.log('   UFW not installed. Installing...');
      execSync('sudo apt-get update && sudo apt-get install -y ufw', { stdio: 'inherit' });
      ufwInstalled = true;
    }
    
    if (ufwInstalled) {
      // Allow SSH first to prevent lockout
      execSync('sudo ufw allow ssh', { stdio: 'inherit' });
      
      // Allow HTTP
      execSync('sudo ufw allow 80/tcp', { stdio: 'inherit' });
      
      // Allow application port
      execSync(`sudo ufw allow ${process.env.PORT}/tcp`, { stdio: 'inherit' });
      
      // Check UFW status and enable if not already
      const ufwStatus = execSync('sudo ufw status | grep "Status: "', { encoding: 'utf8' });
      if (!ufwStatus.includes('active')) {
        console.log('   Enabling firewall...');
        execSync('echo "y" | sudo ufw enable', { stdio: 'inherit' });
      } else {
        console.log('   Firewall already active, ensuring rules are applied...');
        execSync('sudo ufw allow ssh', { stdio: 'ignore' });
        execSync('sudo ufw allow 80/tcp', { stdio: 'ignore' });
        execSync(`sudo ufw allow ${process.env.PORT}/tcp`, { stdio: 'ignore' });
      }
      
      console.log(`   ${colors.green}✓${colors.reset} Firewall configured`);
    } else {
      console.error(`   ${colors.red}×${colors.reset} UFW not found even after attempted installation.`);
      console.error('   Please manually install UFW: sudo apt-get update && sudo apt-get install -y ufw');
    }
  } catch (error) {
    console.error(`   ${colors.red}Error configuring firewall: ${error.message}${colors.reset}`);
  }
}

// Network diagnostics
function runNetworkDiagnostics() {
  console.log(`${colors.yellow}[*] Running network diagnostics...${colors.reset}`);
  
  try {
    // Get network interfaces
    console.log(`   ${colors.green}Network interfaces:${colors.reset}`);
    if (isWindows) {
      const interfaces = execSync('ipconfig', { encoding: 'utf8' });
      console.log(interfaces.split('\n').filter(line => line.includes('IPv4') || line.includes('IPv6')).join('\n'));
    } else {
      const interfaces = execSync("ip addr show | grep -E 'inet |inet6 ' | grep -v '127.0.0.1'", { encoding: 'utf8' });
      console.log(interfaces);
    }
    
    // Check port status
    console.log(`\n   ${colors.green}Checking if application port ${process.env.PORT} is open:${colors.reset}`);
    if (isWindows) {
      try {
        const netstat = execSync(`netstat -ano | findstr :${process.env.PORT}`, { encoding: 'utf8' });
        console.log(netstat || `   No process listening on port ${process.env.PORT}`);
      } catch (error) {
        console.log(`   ${colors.yellow}No process listening on port ${process.env.PORT}${colors.reset}`);
      }
    } else {
      try {
        const lsof = execSync(`sudo lsof -i:${process.env.PORT}`, { encoding: 'utf8' });
        console.log(lsof);
      } catch (error) {
        try {
          const netstat = execSync(`sudo netstat -tuln | grep :${process.env.PORT}`, { encoding: 'utf8' });
          console.log(netstat || `   ${colors.yellow}No process listening on port ${process.env.PORT}${colors.reset}`);
        } catch (error) {
          console.log(`   ${colors.yellow}No process listening on port ${process.env.PORT}${colors.reset}`);
        }
      }
    }
    
    // Test internet connectivity
    console.log(`\n   ${colors.green}Testing internet connectivity:${colors.reset}`);
    try {
      if (isWindows) {
        execSync('ping -n 3 8.8.8.8', { stdio: 'inherit' });
        console.log(`   ${colors.green}✓${colors.reset} Internet connectivity: GOOD (Can reach 8.8.8.8)`);
      } else {
        execSync('ping -c 3 -W 2 8.8.8.8', { stdio: 'inherit' });
        console.log(`   ${colors.green}✓${colors.reset} Internet connectivity: GOOD (Can reach 8.8.8.8)`);
      }
    } catch (error) {
      console.log(`   ${colors.red}×${colors.reset} Internet connectivity: FAILED (Cannot reach 8.8.8.8)`);
    }
    
    // Check DNS resolution
    try {
      if (isWindows) {
        execSync('ping -n 3 google.com', { stdio: 'inherit' });
        console.log(`   ${colors.green}✓${colors.reset} DNS resolution: GOOD (Can reach google.com)`);
      } else {
        execSync('ping -c 3 -W 2 google.com', { stdio: 'inherit' });
        console.log(`   ${colors.green}✓${colors.reset} DNS resolution: GOOD (Can reach google.com)`);
      }
    } catch (error) {
      console.log(`   ${colors.red}×${colors.reset} DNS resolution: FAILED (Cannot reach google.com)`);
    }
    
    // Get public IP
    console.log(`\n   ${colors.green}Getting public IP address:${colors.reset}`);
    try {
      const publicIP = execSync('curl -s https://api.ipify.org || curl -s http://checkip.amazonaws.com', { encoding: 'utf8' }).trim();
      console.log(`   Public IP: ${publicIP}`);
    } catch (error) {
      console.log(`   ${colors.yellow}Could not determine public IP address${colors.reset}`);
    }
    
    console.log(`\n${colors.green}Network diagnostics completed.${colors.reset}`);
  } catch (error) {
    console.error(`   ${colors.red}Error during network diagnostics: ${error.message}${colors.reset}`);
  }
}

// Start application
async function startApplication() {
  console.log(`${colors.yellow}[8] Starting the application...${colors.reset}`);
  
  try {
    const appDir = process.env.APP_DIR;
    
    // Create log directories if not exist
    if (!fs.existsSync(path.join(appDir, 'logs'))) {
      fs.mkdirSync(path.join(appDir, 'logs'), { recursive: true });
    }
    
    // Test configuration first
    console.log('   Testing application before starting...');
    process.chdir(appDir);
    
    let testOutput = '';
    try {
      testOutput = execSync(`${appDir}/${process.env.APP_NAME} --config=${appDir}/fileprocessor.json --test-config`, { encoding: 'utf8' });
      console.log(`   ${colors.green}✓${colors.reset} Application configuration test passed`);
    } catch (error) {
      console.log(`   ${colors.red}✗${colors.reset} Application failed during pre-test`);
      console.log(`   Error output: ${error.message}`);
      console.log('   Attempting to fix common configuration issues...');
      createJsonConfig();
    }
    
    if (isWindows) {
      // Windows: Start application directly
      console.log('   Starting application in background...');
      
      const logDir = path.join(appDir, 'logs');
      const startScript = path.join(os.tmpdir(), 'start_fileprocessor.bat');
      
      const scriptContent = `@echo off
cd ${appDir}
start "Go File Processor" /B ${appDir}\\${process.env.APP_NAME} --config=${appDir}\\fileprocessor.json > ${logDir}\\output.log 2> ${logDir}\\error.log
`;
      
      fs.writeFileSync(startScript, scriptContent);
      execSync(`cmd /c "${startScript}"`, { stdio: 'inherit' });
      
      console.log(`   ${colors.green}✓${colors.reset} Application started`);
      
      // Create startup shortcut
      console.log('   Creating startup shortcut...');
      
      const startupDir = path.join(os.homedir(), 'AppData', 'Roaming', 'Microsoft', 'Windows', 'Start Menu', 'Programs', 'Startup');
      if (!fs.existsSync(startupDir)) {
        fs.mkdirSync(startupDir, { recursive: true });
      }
      
      const startupScript = path.join(startupDir, 'go-fileprocessor.bat');
      fs.writeFileSync(startupScript, scriptContent);
      
      console.log(`   ${colors.green}✓${colors.reset} Startup shortcut created at ${startupScript}`);
    } else {
      // Linux: Use systemd service
      console.log('   Starting systemd service...');
      execSync(`sudo systemctl start ${process.env.SERVICE_NAME}`, { stdio: 'inherit' });
      
      // Check if application started
      try {
        execSync(`systemctl is-active --quiet ${process.env.SERVICE_NAME}`);
        console.log(`   ${colors.green}✓${colors.reset} Application started successfully`);
      } catch (error) {
        console.log(`   ${colors.red}✗${colors.reset} Failed to start application`);
        console.log('   Checking logs for errors...');
        
        try {
          const journalLogs = execSync(`sudo journalctl -u ${process.env.SERVICE_NAME} -n 20 --no-pager`, { encoding: 'utf8' });
          const errorLines = journalLogs.split('\n').filter(line => 
            line.toLowerCase().includes('error') || 
            line.toLowerCase().includes('failed') || 
            line.toLowerCase().includes('fatal')
          ).slice(-5).join('\n');
          
          if (errorLines) {
            console.log(`\n${colors.red}Error details:${colors.reset}`);
            console.log(errorLines);
          }
        } catch (logError) {
          console.error(`   ${colors.red}Could not retrieve service logs: ${logError.message}${colors.reset}`);
        }
        
        const errorLog = path.join(appDir, 'logs', 'error.log');
        if (fs.existsSync(errorLog)) {
          const appError = fs.readFileSync(errorLog, 'utf8').split('\n').slice(-10).join('\n');
          console.log(`\n${colors.red}Application error log:${colors.reset}`);
          console.log(appError);
        }
        
        // Try to fix permission issues
        console.log('\nFixing potential permission issues...');
        try {
          if (!isWindows) {
            execSync(`sudo chown -R ${os.userInfo().username}:${os.userInfo().username} ${appDir}`, { stdio: 'inherit' });
            execSync(`sudo chmod -R 755 ${appDir}`, { stdio: 'inherit' });
          }
        } catch (permError) {
          console.error(`   ${colors.red}Error fixing permissions: ${permError.message}${colors.reset}`);
        }
        
        throw new Error('Failed to start application');
      }
    }
  } catch (error) {
    console.error(`   ${colors.red}Error starting application: ${error.message}${colors.reset}`);
    throw new Error('Application start failed');
  }
}

// Display connection information
function displayConnectionInfo() {
  console.log(`${colors.yellow}[*] Your Go File Processor application is ready!${colors.reset}`);
  
  let publicIP = 'localhost';
  try {
    publicIP = execSync('curl -s https://api.ipify.org || curl -s http://checkip.amazonaws.com', { encoding: 'utf8' }).trim();
  } catch (error) {
    try {
      if (isWindows) {
        const ipconfig = execSync('ipconfig', { encoding: 'utf8' });
        const ipMatch = ipconfig.match(/IPv4 Address[. ]+: ([0-9.]+)/);
        if (ipMatch) {
          publicIP = ipMatch[1];
        }
      } else {
        const hostname = execSync('hostname -I | awk \'{print $1}\'', { encoding: 'utf8' }).trim();
        if (hostname) {
          publicIP = hostname;
        }
      }
    } catch (localError) {
      console.log(`   ${colors.yellow}Could not determine IP address${colors.reset}`);
    }
  }
  
  console.log(`\n${colors.green}Connection Information:${colors.reset}`);
  console.log(`   Main application: http://${publicIP}`);
  console.log(`   Direct port access: http://${publicIP}:${process.env.PORT}`);
  
  console.log(`\n${colors.green}Troubleshooting Tips:${colors.reset}`);
  if (isWindows) {
    console.log(`   • Check application logs at: ${process.env.APP_DIR}\\logs\\error.log`);
    console.log('   • Ensure Windows Firewall allows connections on port ' + process.env.PORT);
  } else {
    console.log(`   • Run 'sudo systemctl status ${process.env.SERVICE_NAME}' to check application status`);
    console.log('   • Run \'sudo systemctl status nginx\' to check web server status');
    console.log(`   • View application logs: sudo cat ${process.env.APP_DIR}/logs/error.log`);
    console.log(`   • View service logs: sudo journalctl -u ${process.env.SERVICE_NAME} -n 50`);
  }
  
  console.log(`\n${colors.blue}============================================${colors.reset}`);
  console.log(`${colors.blue}      Deployment Complete!     ${colors.reset}`);
  console.log(`${colors.blue}============================================${colors.reset}`);
}

// Local deployment function
async function deployLocal() {
  try {
    await configurePorts();
    createDirectories();
    copyFiles();
    updateConfigFile();
    await installGo();
    await buildApplication();
    
    if (!isWindows) {
      createService();
      configureFirewall();
      
      // Ask about Nginx
      const installNginx = await question('Do you want to install and configure Nginx? (y/n): ');
      if (installNginx.toLowerCase() === 'y') {
        console.log('Nginx installation is not implemented in this version of the script.');
        // In a complete implementation, we would add the Nginx installation code here
      } else {
        console.log(`   ${colors.yellow}Skipping Nginx installation.${colors.reset}`);
      }
    }
    
    await startApplication();
    runNetworkDiagnostics();
    displayConnectionInfo();
  } catch (error) {
    console.error(`${colors.red}Local deployment failed: ${error.message}${colors.reset}`);
    process.exit(1);
  }
}

// Setup remote config
async function setupRemoteConfig() {
  console.log(`\n${colors.yellow}Remote Deployment Configuration${colors.reset}`);
  
  process.env.VPS_USER = await question('Enter VPS username (e.g., ubuntu): ');
  process.env.VPS_HOST = await question('Enter VPS IP address: ');
  const sshKeyInput = await question('Enter SSH key path [default: ~/.ssh/id_rsa]: ');
  
  if (sshKeyInput) {
    process.env.SSH_KEY_PATH = sshKeyInput;
  }
  
  // Verify connection
  console.log(`\n${colors.yellow}Verifying SSH connection...${colors.reset}`);
  try {
    if (isWindows) {
      execSync(`ssh -i "${process.env.SSH_KEY_PATH}" -o BatchMode=yes -o ConnectTimeout=5 -o StrictHostKeyChecking=no ${process.env.VPS_USER}@${process.env.VPS_HOST} echo "Connection successful"`, { stdio: 'ignore' });
    } else {
      execSync(`ssh -i ${process.env.SSH_KEY_PATH} -o BatchMode=yes -o ConnectTimeout=5 ${process.env.VPS_USER}@${process.env.VPS_HOST} echo "Connection successful"`, { stdio: 'ignore' });
    }
    console.log(`${colors.green}✓ SSH connection successful${colors.reset}`);
  } catch (error) {
    console.error(`${colors.red}× SSH connection failed${colors.reset}`);
    console.error('Please check your SSH settings and try again.\n');
    console.error('If you haven\'t set up SSH key authentication yet, run:');
    console.error(`  ssh-keygen -t rsa -b 4096  # Generate SSH key if needed`);
    console.error(`  ssh-copy-id ${process.env.VPS_USER}@${process.env.VPS_HOST}  # Copy your key to the VPS`);
    throw new Error('SSH connection failed');
  }
  
  await configurePorts();
}

// Remote deployment function
async function deployRemote() {
  console.log(`${colors.yellow}Preparing remote deployment to ${process.env.VPS_USER}@${process.env.VPS_HOST}...${colors.reset}`);
  
  try {
    // Implementation details for remote deployment would go here
    console.log(`${colors.yellow}Remote deployment feature is not fully implemented in this version.${colors.reset}`);
    console.log('This would involve:');
    console.log('1. Building the application locally for Linux');
    console.log('2. Creating configuration files');
    console.log('3. Copying files to the remote server');
    console.log('4. Setting up systemd services');
    console.log('5. Configuring firewalls');
    console.log('6. Starting the application');
    
    console.log(`\n${colors.yellow}For now, consider using SCP to copy files and SSH to run commands manually.${colors.reset}`);
  } catch (error) {
    console.error(`${colors.red}Remote deployment failed: ${error.message}${colors.reset}`);
    process.exit(1);
  }
}

// Direct server deployment
async function deployDirectServer() {
  try {
    await configurePorts();
    createDirectories();
    copyFiles();
    updateConfigFile();
    await installGo();
    await buildApplication();
    
    if (!isWindows) {
      createService();
      configureFirewall();
      
      // Ask about Nginx
      const installNginx = await question('Do you want to install and configure Nginx? (y/n): ');
      if (installNginx.toLowerCase() === 'y') {
        console.log('Nginx installation is not implemented in this version of the script.');
        // In a complete implementation, we would add the Nginx installation code here
      } else {
        console.log(`   ${colors.yellow}Skipping Nginx installation.${colors.reset}`);
      }
    }
    
    await startApplication();
    runNetworkDiagnostics();
    displayConnectionInfo();
  } catch (error) {
    console.error(`${colors.red}Server deployment failed: ${error.message}${colors.reset}`);
    process.exit(1);
  }
}

// Start the script
main().catch(error => {
  console.error(`${colors.red}Fatal error: ${error.message}${colors.reset}`);
  process.exit(1);
});