# Go File Management System

A comprehensive file management system built with Go that handles, loads, previews, and processes various file types including CSV, text, Word documents, images, audio, and video. The system supports storing files locally or in cloud storage services like Amazon S3 and Google Cloud Storage.

## Features

- **Multiple File Type Support**:
  - CSV: Parsing, data extraction and analysis
  - Text: Content analysis and preview
  - Word Documents: Text extraction and metadata reading
  - Images: Preview, metadata extraction and basic processing
  - Audio: Metadata extraction and format conversion
  - Video: Metadata extraction and thumbnail generation

- **Storage Integrations**:
  - Local file system storage
  - Amazon S3 integration
  - Google Cloud Storage integration

- **Real-time Processing**:
  - File content analysis
  - Metadata extraction
  - Preview generation
  - WebSocket updates for real-time progress tracking

- **Web Interface**:
  - Modern responsive UI built with Bootstrap
  - Upload files to any storage provider
  - Browse and manage files
  - Preview file contents
  - Download processed files

- **Authentication & User Management**:
  - Google OAuth integration
  - User-specific cloud storage configurations
  - Secure credential handling 
  - Session management

- **LAN File Transfer**:
  - Peer discovery on local network
  - Direct device-to-device file transfer
  - No internet connection required
  - Secure token-based transfer validation
  - Rate limiting to prevent abuse

- **Enhanced Security Features**:
  - HTTPS support with proper certificate validation
  - Cross-origin resource sharing (CORS) protection
  - Secure credential storage with filesystem isolation
  - Cryptographically secure session management
  - File permission enforcement
  - Protection against common web vulnerabilities
  - IP-based rate limiting for sensitive operations
  - Token-based verification for all operations
  - Automatic cleanup of stale sessions and files
  - WebSocket connection security with heartbeats and reconnection

## Getting Started

### Prerequisites

- Go (version 1.18 or higher)
- For cloud storage features:
  - AWS account (for S3 integration)
  - Google Cloud account (for GCS integration)
- For authentication:
  - Google OAuth credentials

### Installation

1. Clone the repository:
   ```bash
   git clone https://github.com/yourusername/fileprocessor.git
   cd fileprocessor
   ```

2. Install the required Go packages:
   ```bash
   go mod tidy
   ```

3. Build the application:
   ```bash
   go build -o fileprocessor ./cmd/server/main.go
   ```

### Running the Server

```bash
./fileprocessor --config=./config/fileprocessor.json
```

Command line options:
- `--config`: Path to the configuration file (default: fileprocessor.json)
- `--port`: Server port (overrides config, default: 8080)
- `--ui`: Directory containing UI files (overrides config, default: ./ui)
- `--uploads`: Directory for local file storage (overrides config, default: ./uploads)
- `--cert`: TLS certificate file for HTTPS (overrides config)
- `--key`: TLS key file for HTTPS (overrides config)
- `--test-config`: Test the configuration and exit

### Configuration

The application can be configured using a JSON configuration file or environment variables.

#### Configuration File (fileprocessor.json)

```json
{
  "server": {
    "port": 8080,
    "uiDir": "./ui",
    "uploadsDir": "./uploads",
    "host": "0.0.0.0",
    "allowedOrigins": ["*"],
    "readTimeout": 30,
    "writeTimeout": 60,
    "idleTimeout": 120
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
    "enableCloudStorage": true,
    "enableProgressUpdates": true,
    "enableAuth": false,
    "enableMediaPreview": true
  },
  "auth": {
    "googleClientID": "",
    "googleClientSecret": "",
    "oauthRedirectURL": "http://localhost:8080/api/auth/callback",
    "sessionSecret": "",
    "cookieSecure": false,
    "cookieMaxAge": 604800
  },
  "ssl": {
    "enable": false,
    "certFile": "",
    "keyFile": ""
  }
}
```

#### Environment Variables

Configuration can be overridden with environment variables:

| Environment Variable | Description |
|---------------------|-------------|
| `FP_PORT` | Server port |
| `FP_UI_DIR` | UI directory path |
| `FP_UPLOADS_DIR` | Uploads directory path |
| `FP_HOST` | Server host (e.g., 0.0.0.0 for all interfaces) |
| `FP_CERT_FILE` | SSL certificate file path |
| `FP_KEY_FILE` | SSL key file path |
| `FP_ALLOWED_ORIGINS` | Comma-separated list of allowed origins for CORS |
| `FP_READ_TIMEOUT` | HTTP server read timeout in seconds |
| `FP_WRITE_TIMEOUT` | HTTP server write timeout in seconds |
| `FP_IDLE_TIMEOUT` | HTTP server idle timeout in seconds |
| `FP_STORAGE_PROVIDER` | Default storage provider (local, s3, google) |
| `FP_S3_REGION` | AWS S3 region |
| `FP_S3_BUCKET` | AWS S3 bucket name |
| `FP_S3_ACCESS_KEY` | AWS S3 access key |
| `FP_S3_SECRET_KEY` | AWS S3 secret key |
| `FP_S3_PREFIX` | AWS S3 object prefix |
| `FP_GOOGLE_BUCKET` | Google Cloud Storage bucket name |
| `FP_GOOGLE_CRED_FILE` | Google Cloud credentials file path |
| `FP_GOOGLE_PREFIX` | Google Cloud Storage object prefix |
| `FP_WORKER_COUNT` | Number of worker threads |
| `FP_ENABLE_LAN` | Enable LAN transfer feature (true/false) |
| `FP_ENABLE_AUTH` | Enable authentication (true/false) |
| `FP_GOOGLE_CLIENT_ID` | Google OAuth client ID |
| `FP_GOOGLE_CLIENT_SECRET` | Google OAuth client secret |
| `FP_OAUTH_REDIRECT_URL` | OAuth redirect URL |
| `FP_SESSION_SECRET` | Session encryption secret |
| `FP_COOKIE_SECURE` | Use secure cookies (true/false) |
| `FP_SSL_ENABLE` | Enable HTTPS (true/false) |

## Deployment

The application includes multiple deployment options:

1. **Local Deployment**: Deploy on the current machine
   ```bash
   ./deploy_unified.sh
   ```
   Select option 1 when prompted.

2. **Remote Deployment**: Deploy to a remote server via SSH
   ```bash
   ./deploy_unified.sh
   ```
   Select option 2 when prompted and provide SSH details.

3. **Docker Deployment**: Use Docker Compose
   ```bash
   docker-compose up -d
   ```

For detailed deployment instructions, see [DEPLOYMENT_GUIDE.md](deploy/DEPLOYMENT_GUIDE.md).

## Using Cloud Storage Providers

### Amazon S3

To use Amazon S3 for file storage, you will need to provide:
- Region
- Bucket name
- Access Key
- Secret Key

These can be configured in the web interface when uploading or listing files, or in the server configuration.

### Google Cloud Storage

To use Google Cloud Storage, you will need:
- Bucket name
- Path to a service account credential JSON file

These can be configured in the web interface or in the server configuration.

## Authentication

Authentication is disabled by default. To enable it:

1. Obtain Google OAuth credentials from [Google Developer Console](https://console.developers.google.com/)
2. Set the following in your configuration:
   ```json
   {
     "features": {
       "enableAuth": true
     },
     "auth": {
       "googleClientID": "YOUR_CLIENT_ID",
       "googleClientSecret": "YOUR_CLIENT_SECRET",
       "oauthRedirectURL": "http://your-domain/api/auth/callback",
       "sessionSecret": "",  // A secure random secret will be generated automatically if empty
       "cookieSecure": true  // Always use true in production
     }
   }
   ```

## LAN File Transfer

The LAN file transfer feature allows direct device-to-device file transfer on a local network:

1. Enable the feature in configuration:
   ```json
   {
     "features": {
       "enableLAN": true
     }
   }
   ```
2. Click "Discover Peers on LAN" in the web interface
3. Select a peer to transfer files to/from

### Security Features in LAN File Transfer

The LAN file transfer system includes several security enhancements:

- Token-based transfer authentication to prevent unauthorized transfers
- Automatic session expiration for incomplete transfers
- Rate limiting to prevent abuse of the discovery mechanism
- Input validation to protect against malicious data
- Encrypted transfer tokens using cryptographic random generation
- Peer information sanitization to prevent sensitive data exposure
- Internal IP address masking in responses
- Validation of peer activity before allowing transfers

## WebSocket Communication

The application uses WebSockets for real-time updates during file processing and transfers:

### Security Features in WebSocket Communication

- Secure connection establishment with origin verification
- Automatic reconnection with exponential backoff
- Heartbeat mechanism to detect disconnections
- Message encryption for sensitive operations
- Rate limiting to prevent flooding
- Proper error handling and connection status management

## API Documentation

The server provides the following REST API endpoints:

### File Operations
- `GET /api/files` - List files from a provider
- `POST /api/files/upload` - Upload a file
- `GET /api/files/:id` - Get file metadata
- `GET /api/files/:id/download` - Download a file
- `DELETE /api/files/:id` - Delete a file
- `POST /api/files/:id/process` - Process a file

### Storage Operations
- `GET /api/storage/status` - Get storage provider status
- `POST /api/storage/configure` - Configure a storage provider

### Authentication
- `GET /api/auth/profile` - Get the current user profile
- `GET /api/auth/login` - Start OAuth login flow
- `GET /api/auth/callback` - OAuth callback handler
- `POST /api/auth/logout` - Log out the current user
- `GET /api/auth/cloud-config` - Get user's cloud storage configuration
- `POST /api/auth/cloud-config` - Save user's cloud storage configuration

### LAN Transfer
- `GET /api/lan/discover` - Discover peers on the local network
- `POST /api/lan/initiate` - Initiate a file transfer to a peer
- `POST /api/lan/accept` - Accept or reject a file transfer
- `GET /api/lan/status` - Get file transfer status

## Enhanced Security Features

### Credential Protection

- AWS credentials are stored in a secure, user-specific file with restricted permissions (0600)
- Google Cloud credentials are validated and permissions are hardened
- Session secrets are generated using cryptographically secure random values
- Configuration files are given restricted permissions
- Sensitive values are removed from memory after secure storage

### Network Security

- All file transfers include token-based authentication
- Rate limiting is applied to sensitive endpoints
- IP-based request tracking for abuse prevention
- Proper input validation and sanitization
- WebSocket connection security with automatic reconnection and heartbeats

### Filesystem Security

- Uploads directory has proper permissions
- Temporary files are automatically cleaned up
- File paths are validated to prevent directory traversal
- Content types are verified before processing

### Error Handling and Logging

- Proper error handling to prevent information disclosure
- Secure logging practices to prevent credential leakage
- Stack traces are sanitized in production mode
- Custom error responses that don't expose internal details

## Security Considerations

- Store the configuration file with restricted permissions (0600)
- Use environment variables for sensitive credentials in production
- Enable HTTPS in production environments
- Set a strong session secret or let the application generate one
- Restrict allowed origins in CORS settings to your domains
- Keep Go and all dependencies updated
- Run the application with the principle of least privilege
- Enable authentication in production environments
- Use a reverse proxy (like Nginx) in front of the application for additional security layers
- Implement network-level security with firewalls
- Regularly back up configuration and data

## License

This project is licensed under the MIT License - see the LICENSE file for details.

## Acknowledgments

- [AWS SDK for Go](https://github.com/aws/aws-sdk-go) for S3 integration
- [Google Cloud SDK for Go](https://github.com/googleapis/google-cloud-go) for GCS integration
- [UniDoc](https://github.com/unidoc/unioffice) for Word document processing
- [Bootstrap](https://getbootstrap.com/) for the web interface