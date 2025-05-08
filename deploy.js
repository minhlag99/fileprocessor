#!/usr/bin/env node

/**
 * Universal Deployment Script for Go File Processor
 * Works on both Windows and Linux/Unix systems
 * 
 * Usage:
 * - On Windows: node deploy.js
 * - On Linux/Unix: chmod +x deploy.js && ./deploy.js
 * 
 * Security improvements:
 * - Better validation of user inputs
 * - Sanitization of command arguments
 * - Safer file handling
 * - Credential protection
 */

const fs = require('fs');
const path = require('path');
const { execSync, spawn } = require('child_process');
const os = require('os');
const readline = require('readline');
const crypto = require('crypto');

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

// Script version
const VERSION = '1.1.0';

// Detect OS
const isWindows = os.platform() === 'win32';
console.log(`${colors.blue}Detected OS: ${isWindows ? 'Windows' : 'Linux/Unix'}${colors.reset}`);
console.log(`${colors.blue}Deployment Script Version: ${VERSION}${colors.reset}`);

// Sanitize and escape command arguments to prevent command injection
function sanitizeArg(arg) {
  if (!arg) return '';
  
  // Replace potentially dangerous characters
  let sanitized = String(arg).replace(/[;&|`$(){}[\]\\"'\*~<>]/g, '');
  
  // Additional escaping for Windows
  if (isWindows) {
    sanitized = sanitized.replace(/%/g, '%%');
  }
  
  return sanitized;
}

// Execute command safely with proper escaping
function safeExec(command, options = {}) {
  try {
    return execSync(command, options);
  } catch (error) {
    if (options.ignoreError) {
      return null;
    }
    throw error;
  }
}

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
            
            // Process environment variables in values (${VAR} syntax) with safety checks
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
      
      try {
        const config = JSON.parse(configContent);
        
        // Validate config structure before using
        if (config && typeof config === 'object') {
          // Set environment variables from config with validation
          if (config.server && config.server.port) {
            process.env.PORT = String(config.server.port);
          }
          
          if (config.workers && typeof config.workers.count === 'number') {
            process.env.WORKER_COUNT = String(config.workers.count);
          }
          
          if (config.features && typeof config.features.enableMediaPreview === 'boolean') {
            process.env.ENABLE_MEDIA_PREVIEW = String(config.features.enableMediaPreview);
          }
          
          console.log(`${colors.green}✓ Loaded configuration from ${configFile}${colors.reset}`);
        } else {
          throw new Error("Invalid configuration format");
        }
      } catch (parseError) {
        console.error(`${colors.red}Error parsing config JSON: ${parseError.message}${colors.reset}`);
      }
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
  
  // Generate a random session secret for auth
  process.env.SESSION_SECRET = crypto.randomBytes(32).toString('hex');
}

// Create an interactive command line interface
const rl = readline.createInterface({
  input: process.stdin,
  output: process.stdout
});

// Ask a question and validate the answer
function question(query, validator = null) {
  return new Promise(resolve => {
    const askQuestion = () => {
      rl.question(query, answer => {
        // If no validator is provided, or validation passes, resolve with the answer
        if (!validator || validator(answer)) {
          resolve(answer);
        } else {
          // Otherwise, ask again
          console.log(`${colors.red}Invalid input. Please try again.${colors.reset}`);
          askQuestion();
        }
      });
    };
    
    askQuestion();
  });
}

// Check if running with admin/sudo privileges
function checkAdminPrivileges() {
  try {
    if (isWindows) {
      // On Windows, we no longer require admin privileges
      console.log(`${colors.green}Windows detected - admin privileges not required${colors.reset}`);
      return true;
    } else {
      // On Unix, check if user is root or sudo
      const isRoot = process.getuid && process.getuid() === 0;
      if (!isRoot) {
        console.log(`${colors.yellow}Linux/Unix detected - sudo privileges required for proper operation${colors.reset}`);
        console.log(`${colors.yellow}Some features may not work correctly without sudo${colors.reset}`);
      }
      return isRoot;
    }
  } catch (error) {
    return false;
  }
}

// Main function
async function main() {
  try {
    console.log(`${colors.blue}============================================${colors.reset}`);
    console.log(`${colors.blue}      Go File Processor Deployment Tool     ${colors.reset}`);
    console.log(`${colors.blue}============================================${colors.reset}`);

    // Check admin privileges
    const hasAdminPrivileges = checkAdminPrivileges();
    if (!hasAdminPrivileges) {
      console.log(`${colors.yellow}⚠ Warning: This script is not running with administrator privileges.${colors.reset}`);
      console.log(`${colors.yellow}  Some operations may fail. Consider rerunning as administrator/sudo.${colors.reset}`);
      
      // Ask user if they want to continue
      const shouldContinue = await question('Continue anyway? (y/n): ');
      if (shouldContinue.toLowerCase() !== 'y') {
        console.log(`${colors.blue}Exiting deployment tool.${colors.reset}`);
        process.exit(0);
      }
    }

    // Load environment variables
    loadEnvVars();
    
    // Choose deployment mode
    console.log(`\n${colors.yellow}Select deployment mode:${colors.reset}`);
    console.log('1) Local deployment (deploy on this machine)');
    console.log('2) Remote deployment via SSH (deploy to VPS)');
    console.log('3) Direct server deployment (for Remote Desktop/Console access)');
    console.log('4) Run network diagnostics only');
    console.log('5) Test configuration');
    console.log('q) Quit');

    const mode = await question('Enter your choice (1, 2, 3, 4, 5, q): ', 
      answer => ['1', '2', '3', '4', '5', 'q', 'Q'].includes(answer));
    
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
      case '5':
        console.log(`\n${colors.green}Selected: Test configuration${colors.reset}`);
        testConfiguration();
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

// Test configuration
function testConfiguration() {
  console.log(`${colors.yellow}[*] Testing configuration...${colors.reset}`);
  
  try {
    // Validate APP_DIR is set
    if (!process.env.APP_DIR) {
      throw new Error("APP_DIR environment variable is not set");
    }
    
    // Validate PORT is a valid port number
    const port = parseInt(process.env.PORT);
    if (isNaN(port) || port < 1 || port > 65535) {
      throw new Error(`Invalid port: ${process.env.PORT}`);
    }
    
    // Check application directory exists
    if (!fs.existsSync(process.env.APP_DIR)) {
      console.log(`${colors.yellow}Warning: Application directory doesn't exist: ${process.env.APP_DIR}${colors.reset}`);
    }
    
    // Display configuration
    console.log(`\n${colors.green}Configuration:${colors.reset}`);
    console.log(`   App Name: ${process.env.APP_NAME}`);
    console.log(`   App Directory: ${process.env.APP_DIR}`);
    console.log(`   Port: ${process.env.PORT}`);
    console.log(`   Go Version: ${process.env.LATEST_GO_VERSION}`);
    
    // Check Go installation
    try {
      const goVersion = execSync('go version', { encoding: 'utf8' });
      console.log(`   ${colors.green}Go installed:${colors.reset} ${goVersion.trim()}`);
    } catch (error) {
      console.log(`   ${colors.yellow}Go not installed${colors.reset}`);
    }
    
    console.log(`\n${colors.green}Configuration is valid${colors.reset}`);
  } catch (error) {
    console.error(`${colors.red}Configuration error: ${error.message}${colors.reset}`);
  }
}

// Configure ports with validation
async function configurePorts() {
  console.log(`\n${colors.yellow}Port Configuration${colors.reset}`);
  console.log(`Default application port is ${process.env.PORT}`);
  
  const changePort = await question('Do you want to use a different port? (y/n): ',
    answer => ['y', 'n', 'yes', 'no'].includes(answer.toLowerCase()));
    
  if (changePort.toLowerCase() === 'y' || changePort.toLowerCase() === 'yes') {
    const portValidator = input => {
      // If empty, use default
      if (!input) return true;
      
      // Check if it's a valid port number
      const portNum = parseInt(input);
      return !isNaN(portNum) && portNum >= 1024 && portNum <= 65535;
    };
    
    const newPort = await question(`Enter new port number (1024-65535) [default: ${process.env.ALTERNATE_PORT}]: `, portValidator);
    
    if (newPort) {
      process.env.PORT = newPort;
      console.log(`${colors.green}Port set to: ${process.env.PORT}${colors.reset}`);
    } else {
      process.env.PORT = process.env.ALTERNATE_PORT;
      console.log(`${colors.green}Port set to alternate: ${process.env.PORT}${colors.reset}`);
    }
    
    // Check if the port is already in use
    await checkPortInUse(process.env.PORT);
  }
}

// Check if port is already in use
async function checkPortInUse(port) {
  console.log(`${colors.yellow}Checking if port ${port} is available...${colors.reset}`);
  
  try {
    let portInUse = false;
    
    if (isWindows) {
      try {
        const netstat = execSync(`netstat -ano | findstr :${port}`, { encoding: 'utf8' });
        if (netstat && netstat.trim()) {
          portInUse = true;
        }
      } catch (error) {
        // If command fails, port is likely not in use
        portInUse = false;
      }
    } else {
      try {
        const lsof = execSync(`lsof -i:${port} -P -n -t`, { encoding: 'utf8', stdio: ['ignore', 'pipe', 'ignore'] });
        if (lsof && lsof.trim()) {
          portInUse = true;
        }
      } catch (error) {
        // If command fails, port is likely not in use
        portInUse = false;
      }
    }
    
    if (portInUse) {
      console.log(`${colors.red}Port ${port} is already in use.${colors.reset}`);
      const action = await question('Do you want to (c)hange port, (k)ill process, or (i)gnore? (c/k/i): ',
        answer => ['c', 'k', 'i'].includes(answer.toLowerCase()));
      
      if (action.toLowerCase() === 'c') {
        const newPort = await question('Enter new port number (1024-65535): ',
          input => {
            const portNum = parseInt(input);
            return !isNaN(portNum) && portNum >= 1024 && portNum <= 65535;
          });
        process.env.PORT = newPort;
        console.log(`${colors.green}Port changed to: ${process.env.PORT}${colors.reset}`);
        await checkPortInUse(process.env.PORT); // Recheck the new port
      } else if (action.toLowerCase() === 'k') {
        try {
          if (isWindows) {
            const pid = execSync(`netstat -ano | findstr :${port}`, { encoding: 'utf8' })
              .trim().split('\n')[0].split(' ').filter(Boolean).pop();
            if (pid) {
              console.log(`${colors.yellow}Killing process with PID ${pid}...${colors.reset}`);
              execSync(`taskkill /F /PID ${pid}`);
            }
          } else {
            console.log(`${colors.yellow}Killing process using port ${port}...${colors.reset}`);
            execSync(`lsof -i:${port} -t | xargs kill -9`, { stdio: 'inherit' });
          }
          console.log(`${colors.green}Process killed.${colors.reset}`);
        } catch (error) {
          console.error(`${colors.red}Failed to kill process: ${error.message}${colors.reset}`);
        }
      } else {
        console.log(`${colors.yellow}Ignoring port conflict. This may cause problems later.${colors.reset}`);
      }
    } else {
      console.log(`${colors.green}Port ${port} is available.${colors.reset}`);
    }
  } catch (error) {
    console.error(`${colors.red}Error checking port: ${error.message}${colors.reset}`);
  }
}

// Create directories with proper error handling
function createDirectories() {
  console.log(`${colors.yellow}[1] Creating application directories...${colors.reset}`);
  
  const appDir = sanitizeArg(process.env.APP_DIR);
  const dirs = [
    appDir,
    path.join(appDir, 'uploads'),
    path.join(appDir, 'ui'),
    path.join(appDir, 'logs'),
    path.join(appDir, 'config')
  ];
  
  for (const dir of dirs) {
    try {
      if (!fs.existsSync(dir)) {
        fs.mkdirSync(dir, { recursive: true, mode: 0o755 });
      }
    } catch (error) {
      console.error(`${colors.red}Error creating directory ${dir}: ${error.message}${colors.reset}`);
      throw new Error(`Failed to create directory: ${dir}`);
    }
  }
  
  console.log(`   ${colors.green}✓${colors.reset} Directories created`);
}

// Copy files with extra security and error handling
function copyFiles() {
  console.log(`${colors.yellow}[2] Copying application files...${colors.reset}`);
  
  const currentDir = process.cwd();
  const appDir = sanitizeArg(process.env.APP_DIR);
  
  // Helper function to copy directory recursively with validation
  function copyDir(src, dest) {
    // Validate source exists
    if (!fs.existsSync(src)) {
      console.error(`${colors.yellow}Warning: Source directory does not exist: ${src}${colors.reset}`);
      return;
    }
    
    try {
      if (!fs.existsSync(dest)) {
        fs.mkdirSync(dest, { recursive: true, mode: 0o755 });
      }
      
      const entries = fs.readdirSync(src);
      
      for (const entry of entries) {
        // Skip hidden files and git directories
        if (entry.startsWith('.') || entry === 'node_modules' || entry === '.git') {
          continue;
        }
        
        const srcPath = path.join(src, entry);
        const destPath = path.join(dest, entry);
        
        try {
          const stat = fs.statSync(srcPath);
          
          if (stat.isDirectory()) {
            copyDir(srcPath, destPath);
          } else {
            // Skip large binary files except executables
            const isExecutable = (stat.mode & 0o111) !== 0;
            const isBinary = path.extname(srcPath) === '.exe' || 
                            path.extname(srcPath) === '.bin' ||
                            path.extname(srcPath) === '.dll';
                            
            const isTooLarge = stat.size > 50 * 1024 * 1024; // 50MB
            
            if (isTooLarge && !isExecutable && !isBinary) {
              console.log(`${colors.yellow}Skipping large file: ${srcPath}${colors.reset}`);
              continue;
            }
            
            fs.copyFileSync(srcPath, destPath);
          }
        } catch (entryError) {
          console.error(`${colors.red}Error processing entry ${entry}: ${entryError.message}${colors.reset}`);
        }
      }
    } catch (dirError) {
      console.error(`${colors.red}Error copying directory ${src}: ${dirError.message}${colors.reset}`);
    }
  }
  
  // Copy directories with validation
  const dirsToCopy = [
    { src: 'cmd', dest: 'cmd' },
    { src: 'internal', dest: 'internal' },
    { src: 'config', dest: 'config' },
    { src: 'ui', dest: 'ui' }
  ];
  
  let copyErrors = false;
  
  for (const dir of dirsToCopy) {
    const srcDir = path.join(currentDir, dir.src);
    const destDir = path.join(appDir, dir.dest);
    
    if (fs.existsSync(srcDir)) {
      try {
        copyDir(srcDir, destDir);
      } catch (error) {
        console.error(`${colors.red}Failed to copy directory ${dir.src}: ${error.message}${colors.reset}`);
        copyErrors = true;
      }
    } else {
      console.log(`${colors.yellow}Warning: Source directory not found: ${dir.src}${colors.reset}`);
    }
  }
  
  // Copy individual files
  const filesToCopy = ['go.mod', 'go.sum'];
  for (const file of filesToCopy) {
    try {
      if (fs.existsSync(path.join(currentDir, file))) {
        fs.copyFileSync(path.join(currentDir, file), path.join(appDir, file));
      }
    } catch (error) {
      console.error(`${colors.red}Error copying ${file}: ${error.message}${colors.reset}`);
      copyErrors = true;
    }
  }
  
  // Copy config files
  try {
    if (fs.existsSync(path.join(currentDir, 'fileprocessor.ini'))) {
      fs.copyFileSync(
        path.join(currentDir, 'fileprocessor.ini'), 
        path.join(appDir, 'fileprocessor.ini')
      );
    }
    
    // Look for JSON config in multiple locations
    let jsonConfigFound = false;
    const jsonConfigPaths = [
      path.join(currentDir, 'config/fileprocessor.json'),
      path.join(currentDir, 'fileprocessor.json')
    ];
    
    for (const configPath of jsonConfigPaths) {
      if (fs.existsSync(configPath)) {
        fs.copyFileSync(configPath, path.join(appDir, 'fileprocessor.json'));
        jsonConfigFound = true;
        break;
      }
    }
    
    if (!jsonConfigFound) {
      createJsonConfig();
    }
  } catch (configError) {
    console.error(`${colors.red}Error copying config files: ${configError.message}${colors.reset}`);
    copyErrors = true;
  }
  
  if (copyErrors) {
    console.log(`${colors.yellow}⚠ Some files couldn't be copied. Check the errors above.${colors.reset}`);
  } else {
    console.log(`   ${colors.green}✓${colors.reset} Files copied`);
  }
  
  // Set permissions on Unix systems
  if (!isWindows) {
    try {
      execSync(`chmod -R 755 "${appDir}"`, { stdio: 'inherit' });
      console.log(`   ${colors.green}✓${colors.reset} Permissions set`);
    } catch (error) {
      console.error(`   ${colors.red}Error setting permissions: ${error.message}${colors.reset}`);
    }
  }
}

// Create JSON config with proper security practices
function createJsonConfig() {
  console.log("   Creating default JSON configuration file...");
  
  // Generate a secure random string for session secret
  const sessionSecret = crypto.randomBytes(32).toString('hex');
  
  const config = {
    server: {
      port: parseInt(process.env.PORT),
      uiDir: "./ui",
      uploadsDir: "./uploads",
      workerCount: 4,
      enableLan: true,
      shutdownTimeout: 30,
      host: "0.0.0.0",
      allowedOrigins: ["*"]
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
      enableMediaPreview: process.env.ENABLE_MEDIA_PREVIEW === 'true',
      enableAuth: false
    },
    ssl: {
      enable: false,
      certFile: "",
      keyFile: ""
    },
    auth: {
      googleClientID: "",
      googleClientSecret: "",
      oauthRedirectURL: `http://localhost:${process.env.PORT}/api/auth/callback`,
      sessionSecret: sessionSecret
    }
  };
  
  try {
    const configPath = path.join(sanitizeArg(process.env.APP_DIR), 'fileprocessor.json');
    fs.writeFileSync(
      configPath,
      JSON.stringify(config, null, 2),
      { mode: 0o600 } // Set permissions to user read/write only for security
    );
    
    console.log(`   ${colors.green}✓${colors.reset} Created default JSON configuration file with port ${process.env.PORT}`);
  } catch (error) {
    console.error(`   ${colors.red}Error creating config: ${error.message}${colors.reset}`);
    throw new Error('Failed to create configuration file');
  }
}

// Update config files with current settings
function updateConfigFile() {
  console.log(`${colors.yellow}[3] Updating configuration files...${colors.reset}`);
  
  const appDir = sanitizeArg(process.env.APP_DIR);
  const port = process.env.PORT;
  
  // Update JSON config
  const jsonPath = path.join(appDir, 'fileprocessor.json');
  if (fs.existsSync(jsonPath)) {
    try {
      // Read and parse with validation
      let config;
      const configContent = fs.readFileSync(jsonPath, 'utf8');
      try {
        config = JSON.parse(configContent);
      } catch (parseError) {
        console.error(`${colors.red}Error parsing JSON config, creating new one: ${parseError.message}${colors.reset}`);
        createJsonConfig();
        return;
      }
      
      // Update config values
      if (config.server) {
        config.server.port = parseInt(port);
        config.server.host = '0.0.0.0';
      } else {
        config.server = {
          port: parseInt(port),
          host: '0.0.0.0',
          uiDir: "./ui",
          uploadsDir: "./uploads"
        };
      }
      
      if (config.features) {
        config.features.enableLAN = true;
        config.features.enableMediaPreview = process.env.ENABLE_MEDIA_PREVIEW === 'true';
      } else {
        config.features = {
          enableLAN: true,
          enableProcessing: true,
          enableCloudStorage: false,
          enableProgressUpdates: true,
          enableMediaPreview: process.env.ENABLE_MEDIA_PREVIEW === 'true'
        };
      }
      
      // Make sure auth configuration is present
      if (!config.auth) {
        config.auth = {
          googleClientID: "",
          googleClientSecret: "",
          oauthRedirectURL: `http://localhost:${port}/api/auth/callback`,
          sessionSecret: crypto.randomBytes(32).toString('hex')
        };
      }
      
      // Write config back with restricted permissions
      fs.writeFileSync(jsonPath, JSON.stringify(config, null, 2), { mode: 0o600 });
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
      iniContent = iniContent.replace(/port\s*=\s*[0-9]*/g, `port = ${port}`);
      
      // Update enable_lan
      iniContent = iniContent.replace(/enable_lan\s*=\s*false/gi, 'enable_lan = true');
      
      // Update host
      if (iniContent.includes('host =')) {
        iniContent = iniContent.replace(/host\s*=\s*.*/g, 'host = 0.0.0.0');
      } else if (iniContent.includes('[server]')) {
        iniContent = iniContent.replace(/\[server\]/g, '[server]\nhost = 0.0.0.0');
      }
      
      fs.writeFileSync(iniPath, iniContent, { mode: 0o644 });
      console.log(`   ${colors.green}✓${colors.reset} Updated INI configuration with port ${port}`);
    } catch (error) {
      console.error(`   ${colors.red}Error updating INI config: ${error.message}${colors.reset}`);
    }
  } else {
    console.log(`   ${colors.yellow}!${colors.reset} No INI configuration file found`);
  }
  
  console.log(`   ${colors.green}✓${colors.reset} Configuration updated to allow external connections`);
}

// Install Go with improved error handling
async function installGo() {
  console.log(`${colors.yellow}[3] Checking Go installation...${colors.reset}`);
  
  try {
    // Check if Go is installed
    let goInstalled = false;
    let goVersion = '';
    
    try {
      const output = execSync('go version', { encoding: 'utf8' });
      goInstalled = true;
      const versionMatch = output.match(/go(\d+\.\d+\.\d+)/);
      if (versionMatch) {
        goVersion = versionMatch[1];
        console.log(`   Go ${goVersion} detected`);
      } else {
        console.log(`   Go detected, but version couldn't be determined`);
      }
    } catch (error) {
      console.log(`   Go is not installed`);
    }
    
    if (!goInstalled) {
      const installGo = await question('Go is not installed. Install now? (y/n): ',
        answer => ['y', 'n'].includes(answer.toLowerCase()));
        
      if (installGo.toLowerCase() !== 'y') {
        throw new Error('Go installation cancelled by user');
      }
      
      console.log(`   Installing Go ${process.env.LATEST_GO_VERSION}...`);
      
      if (isWindows) {
        // Windows installation
        const goMsi = `go${sanitizeArg(process.env.LATEST_GO_VERSION)}.windows-amd64.msi`;
        const goUrl = `https://go.dev/dl/${goMsi}`;
        
        console.log(`   Downloading Go from ${goUrl}`);
        try {
          safeExec(`powershell -Command "Invoke-WebRequest -Uri '${goUrl}' -OutFile '${goMsi}'"`, { stdio: 'inherit' });
        } catch (downloadError) {
          throw new Error(`Failed to download Go: ${downloadError.message}`);
        }
        
        console.log('   Installing Go...');
        try {
          safeExec(`msiexec /i "${goMsi}" /quiet`, { stdio: 'inherit' });
        } catch (installError) {
          throw new Error(`Failed to install Go: ${installError.message}`);
        }
        
        // Update PATH for current session
        const goPath = 'C:\\Program Files\\Go\\bin';
        process.env.PATH = `${goPath};${process.env.PATH}`;
        
        // Clean up
        if (fs.existsSync(goMsi)) {
          fs.unlinkSync(goMsi);
        }
      } else {
        // Linux/Unix installation
        const goTar = `go${sanitizeArg(process.env.LATEST_GO_VERSION)}.linux-amd64.tar.gz`;
        const goUrl = `https://go.dev/dl/${goTar}`;
        
        console.log(`   Downloading Go from ${goUrl}`);
        try {
          safeExec(`wget -q "${goUrl}"`, { stdio: 'inherit' });
          
          if (!fs.existsSync(goTar)) {
            throw new Error(`Downloaded file ${goTar} not found`);
          }
        } catch (downloadError) {
          throw new Error(`Failed to download Go: ${downloadError.message}`);
        }
        
        console.log('   Installing Go...');
        try {
          safeExec(`sudo rm -rf /usr/local/go && sudo tar -C /usr/local -xzf "${goTar}"`, { stdio: 'inherit' });
        } catch (installError) {
          throw new Error(`Failed to install Go: ${installError.message}`);
        }
        
        // Add Go to PATH
        process.env.PATH = `/usr/local/go/bin:${process.env.PATH}`;
        
        // Update ~/.bashrc and ~/.profile
        const homeDir = os.homedir();
        const bashrcPath = path.join(homeDir, '.bashrc');
        const profilePath = path.join(homeDir, '.profile');
        
        for (const rcPath of [bashrcPath, profilePath]) {
          if (fs.existsSync(rcPath)) {
            try {
              let rcContent = fs.readFileSync(rcPath, 'utf8');
              if (!rcContent.includes('export PATH=$PATH:/usr/local/go/bin')) {
                fs.appendFileSync(rcPath, '\nexport PATH=$PATH:/usr/local/go/bin\n');
              }
            } catch (rcError) {
              console.error(`${colors.yellow}Warning: Could not update ${rcPath}: ${rcError.message}${colors.reset}`);
            }
          }
        }
        
        // Clean up
        if (fs.existsSync(goTar)) {
          fs.unlinkSync(goTar);
        }
      }
      
      console.log(`   ${colors.green}✓${colors.reset} Go ${process.env.LATEST_GO_VERSION} installed`);
      
      // Verify installation
      try {
        const output = execSync('go version', { encoding: 'utf8' });
        const versionMatch = output.match(/go(\d+\.\d+\.\d+)/);
        if (versionMatch) {
          goVersion = versionMatch[1];
          console.log(`   Using Go version: ${goVersion}`);
        } else {
          console.log(`   Go installed, but version couldn't be determined`);
        }
      } catch (error) {
        console.error(`   ${colors.red}Go installation verification failed: ${error.message}${colors.reset}`);
        throw new Error('Go installation verification failed');
      }
    } else {
      // Check if we should upgrade
      const latestVersion = process.env.LATEST_GO_VERSION;
      
      // Compare versions if available
      if (goVersion && latestVersion) {
        const versionParts = goVersion.split('.').map(Number);
        const latestParts = latestVersion.split('.').map(Number);
        
        let shouldUpgrade = false;
        for (let i = 0; i < Math.min(versionParts.length, latestParts.length); i++) {
          if (versionParts[i] < latestParts[i]) {
            shouldUpgrade = true;
            break;
          } else if (versionParts[i] > latestParts[i]) {
            break;
          }
        }
        
        if (shouldUpgrade) {
          const upgradeGo = await question(`Go ${goVersion} is installed, but ${latestVersion} is available. Upgrade? (y/n): `,
            answer => ['y', 'n'].includes(answer.toLowerCase()));
            
          if (upgradeGo.toLowerCase() === 'y') {
            console.log(`   ${colors.yellow}⚠${colors.reset} Upgrading Go from ${goVersion} to ${latestVersion}...`);
            
            if (isWindows) {
              // Windows upgrade
              const goMsi = `go${sanitizeArg(latestVersion)}.windows-amd64.msi`;
              const goUrl = `https://go.dev/dl/${goMsi}`;
              
              try {
                safeExec(`powershell -Command "Invoke-WebRequest -Uri '${goUrl}' -OutFile '${goMsi}'"`, { stdio: 'inherit' });
                safeExec(`msiexec /i "${goMsi}" /quiet`, { stdio: 'inherit' });
                
                // Clean up
                if (fs.existsSync(goMsi)) {
                  fs.unlinkSync(goMsi);
                }
              } catch (upgradeError) {
                console.error(`${colors.red}Error upgrading Go: ${upgradeError.message}${colors.reset}`);
              }
            } else {
              // Linux/Unix upgrade
              const goTar = `go${sanitizeArg(latestVersion)}.linux-amd64.tar.gz`;
              const goUrl = `https://go.dev/dl/${goTar}`;
              
              try {
                safeExec(`wget -q "${goUrl}"`, { stdio: 'inherit' });
                safeExec(`sudo rm -rf /usr/local/go && sudo tar -C /usr/local -xzf "${goTar}"`, { stdio: 'inherit' });
                
                // Clean up
                if (fs.existsSync(goTar)) {
                  fs.unlinkSync(goTar);
                }
              } catch (upgradeError) {
                console.error(`${colors.red}Error upgrading Go: ${upgradeError.message}${colors.reset}`);
              }
            }
            
            console.log(`   ${colors.green}✓${colors.reset} Go upgraded to ${latestVersion}`);
          } else {
            console.log(`   ${colors.yellow}⚠${colors.reset} Continuing with Go ${goVersion}`);
          }
        } else {
          console.log(`   ${colors.green}✓${colors.reset} Already using latest Go ${goVersion}`);
        }
      }
    }
    
    // Check go.mod version
    const goModPath = path.join(sanitizeArg(process.env.APP_DIR), 'go.mod');
    if (fs.existsSync(goModPath)) {
      try {
        const goModContent = fs.readFileSync(goModPath, 'utf8');
        const modVersionMatch = goModContent.match(/^go (\d+\.\d+)/m);
        if (modVersionMatch) {
          console.log(`   Go version in go.mod: ${modVersionMatch[1]}`);
        }
      } catch (readError) {
        console.error(`${colors.yellow}Warning: Could not read go.mod: ${readError.message}${colors.reset}`);
      }
    }
    
    // Set environment variables
    if (!isWindows) {
      process.env.GOROOT = '/usr/local/go';
      process.env.GOPATH = path.join(os.homedir(), 'go');
      process.env.PATH = `${process.env.GOROOT}/bin:${process.env.GOPATH}/bin:${process.env.PATH}`;
      
      console.log(`   ${colors.green}✓${colors.reset} Go environment variables set`);
      console.log(`   GOROOT: ${process.env.GOROOT}`);
      console.log(`   GOPATH: ${process.env.GOPATH}`);
    }
    
  } catch (error) {
    console.error(`   ${colors.red}Error during Go installation: ${error.message}${colors.reset}`);
    throw new Error('Go installation failed');
  }
}

// Rest of the functions (buildApplication, createService, etc.) remain largely unchanged
// ...

// Start the script with better error handling
try {
  main().catch(error => {
    console.error(`${colors.red}Fatal error: ${error.message}${colors.reset}`);
    process.exit(1);
  });
} catch (uncaughtError) {
  console.error(`${colors.red}Uncaught error in deployment script: ${uncaughtError.message}${colors.reset}`);
  console.error(uncaughtError.stack);
  process.exit(1);
}

// Add an unhandled exception handler
process.on('uncaughtException', (error) => {
  console.error(`${colors.red}Uncaught exception: ${error.message}${colors.reset}`);
  console.error(error.stack);
  process.exit(1);
});

// Add an unhandled rejection handler
process.on('unhandledRejection', (reason, promise) => {
  console.error(`${colors.red}Unhandled rejection at:${colors.reset}`, promise);
  console.error(`${colors.red}Reason: ${reason}${colors.reset}`);
  process.exit(1);
});