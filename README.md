# Go File Management System

A complete file management system built with Go that can handle, load, preview, and process various file types including CSV, text, Word documents, and images. The system supports storing files locally or in cloud storage services like Amazon S3 and Google Cloud Storage.

## Features

- **Multiple File Type Support**:
  - CSV: Parsing and data extraction
  - Text: Content analysis and preview
  - Word Documents: Text extraction and metadata reading
  - Images: Preview and metadata extraction

- **Storage Integrations**:
  - Local file system storage
  - Amazon S3 integration
  - Google Cloud Storage integration

- **Real-time Processing**:
  - File content analysis
  - Metadata extraction
  - Preview generation

- **Web Interface**:
  - Upload files to any storage provider
  - Browse and manage files
  - Preview file contents
  - Download files

## Getting Started

### Prerequisites

- Go (version 1.18 or higher)
- For cloud storage features:
  - AWS account (for S3 integration)
  - Google Cloud account (for GCS integration)

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
   go build -o fileserver ./cmd/server/main.go
   ```

### Running the Server

```bash
./fileserver --port=8080 --ui=./ui --uploads=./uploads
```

Command line options:
- `--port`: Server port (default: 8080)
- `--ui`: Directory containing UI files (default: ./ui)
- `--uploads`: Directory for local file storage (default: ./uploads)
- `--cert`: TLS certificate file for HTTPS (optional)
- `--key`: TLS key file for HTTPS (optional)

## Using Cloud Storage Providers

### Amazon S3

To use Amazon S3 for file storage, you will need to provide:
- Region
- Bucket name
- Access Key
- Secret Key

These can be configured in the web interface when uploading or listing files.

### Google Cloud Storage

To use Google Cloud Storage, you will need:
- Bucket name
- Path to a service account credential JSON file

## API Documentation

The server provides the following REST API endpoints:

### File Operations

- **Upload a File**
  - URL: `/api/upload`
  - Method: `POST`
  - Parameters:
    - `file`: File to upload (form-data)
    - `storageType`: Storage provider (`local`, `s3`, or `google`)
    - `processFile`: Whether to process the file after upload (`true` or `false`)
    - Storage-specific parameters (region, bucket, etc.)

- **Download a File**
  - URL: `/api/download`
  - Method: `GET`
  - Parameters:
    - `id`: File ID
    - `storageType`: Storage provider

- **List Files**
  - URL: `/api/list`
  - Method: `GET`
  - Parameters:
    - `storageType`: Storage provider
    - `prefix`: Filter files by prefix (optional)

- **Get Signed URL**
  - URL: `/api/url`
  - Method: `GET`
  - Parameters:
    - `id`: File ID
    - `storageType`: Storage provider
    - `operation`: `read` or `write`
    - `expiry`: URL expiry time in minutes

- **Delete a File**
  - URL: `/api/delete`
  - Method: `DELETE`
  - Parameters:
    - `id`: File ID
    - `storageType`: Storage provider

## License

This project is licensed under the MIT License - see the LICENSE file for details.

## Acknowledgments

- [AWS SDK for Go](https://github.com/aws/aws-sdk-go) for S3 integration
- [Google Cloud SDK for Go](https://github.com/googleapis/google-cloud-go) for GCS integration
- [UniDoc](https://github.com/unidoc/unioffice) for Word document processing