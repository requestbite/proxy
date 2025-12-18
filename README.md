# RequestBite Slingshot Proxy

## About

Since it's almost impossible for webapps (such as
[Slingshot](https://s.requestbite.com)) to directly make HTTP requests to
arbitrary HTTP resources because of CORS restrictions, a proxy is needed. This
repo holds the proxy used by Slingshot. It's written in Go and hosted at
[p.requestbite.com](https://p.requestbite.com/health) which is the one used by
default by Slingshot. However, nothing prevents you from running it yourself
(and configuring Slingshot to use it).

Running it yourself means you don't have to proxy any requests via our servers
(unless you want to) and it means you can access resources normally not
reachable over the public Internet, such as those on your machine, on your local
network, or on any VPN you might be connected to.

## Installation

### Quick Install (Recommended)

Install the latest version on macOS or Linux with a single command:

```bash
curl -fsSL https://raw.githubusercontent.com/requestbite/proxy-go/main/install.sh | bash
```

The binary will be installed to `~/.local/bin` by default.

### Custom Installation Directory

```bash
curl -fsSL https://raw.githubusercontent.com/requestbite/proxy-go/main/install.sh | bash -s -- --prefix=$HOME/bin
```

### Manual Download

Download pre-built binaries from [GitHub Releases](https://github.com/requestbite/proxy-go/releases).

**Supported Platforms:**

| OS | Architecture | Binary Name |
|----|--------------|-------------|
| macOS | Intel (x86-64) | `requestbite-proxy-*-darwin-amd64.tar.gz` |
| macOS | Apple Silicon (ARM64) | `requestbite-proxy-*-darwin-arm64.tar.gz` |
| Linux | x86-64 | `requestbite-proxy-*-linux-amd64.tar.gz` |
| Windows | x86-64 | `requestbite-proxy-*-windows-amd64.zip` |

After downloading, extract the archive and move the binary to a directory in your PATH:

```bash
# macOS/Linux
tar -xzf requestbite-proxy-*.tar.gz
mv requestbite-proxy/requestbite-proxy ~/.local/bin/

# Make sure ~/.local/bin is in your PATH
export PATH="$HOME/.local/bin:$PATH"
```

### Build from Source

Requires Go 1.19 or later:

```bash
# Clone the repository
git clone https://github.com/requestbite/proxy-go.git
cd proxy-go

# Build using Makefile
make build

# Or build manually
go build -o requestbite-proxy .
```

### Development Workflow

For active development with hot reload (requires [Air](https://github.com/air-verse/air)):

```bash
# Install Air (one-time setup)
go install github.com/air-verse/air@latest

# Run with hot reload (no arguments)
make dev

# Run with CLI arguments
make dev ARGS="--enable-local-files"

# Run with multiple arguments
make dev ARGS="--enable-local-files --port 9090"

# Show help
make dev ARGS="--help"
```

The `ARGS` variable allows you to pass any CLI arguments to the proxy when using `make dev`, making it easy to test different configurations during development.

## Quick Start

### Run

```bash
# Start on default port 8080
requestbite-proxy

# Start on custom port
requestbite-proxy -port 8081

# Show version
requestbite-proxy -version

# Show help
requestbite-proxy -help
```

For local development, you can still use the simple build commands:

```bash
go build -o proxy .
./proxy
```

## API Endpoints

### POST /proxy/request

Executes HTTP requests from JSON payload.

**Request Body:**

```json
{
  "method": "GET",
  "url": "https://example.com/api",
  "headers": [
    "Content-Type: application/json",
    "Authorization: Bearer token"
  ],
  "body": "request body content",
  "timeout": 30,
  "followRedirects": true,
  "path_params": {
    ":id": "123",
    ":category": "users"
  }
}
```

**Response:**

```json
{
  "success": true,
  "response_status": 200,
  "response_headers": {
    "content-type": "application/json"
  },
  "response_data": "response body",
  "response_size": "1.2 KB",
  "response_time": "156.78 ms",
  "content_type": "application/json",
  "is_binary": false,
  "cancelled": false
}
```

### POST /proxy/form

Executes form-based HTTP requests.

**Query Parameters:**

- `url`: Target URL (required)
- `method`: HTTP method (default: POST)
- `timeout`: Timeout in seconds (default: 60)
- `followRedirects`: Whether to follow redirects (default: true)
- `contentType`: Content type (application/x-www-form-urlencoded or multipart/form-data)
- `headers`: Comma-separated header list

**Form Data:**
Standard form data in request body.

### POST /file

Serves local files from the filesystem. **This endpoint is disabled by default** and must be explicitly enabled with the `--enable-local-files` flag for security reasons.

**⚠️ Security Warning:** Only enable this feature in trusted environments. This allows the proxy to read any file accessible to the process user.

**Request Body:**

```json
{
  "path": "/absolute/path/to/file.txt"
}
```

**Request Fields:**
- `path`: Absolute path to the file (required). Works with both Unix-style (`/home/user/file.txt`) and Windows-style (`C:\Users\user\file.txt`) paths.

**Response:**
- **Success (200)**: Returns the raw file content with appropriate `Content-Type` header based on file extension and content detection
- **Not Found (404)**: File doesn't exist or feature is disabled
- **Error**: JSON error response for invalid paths, directories, or access errors

**Example:**

```bash
# Enable local file serving
requestbite-proxy --enable-local-files

# Request a file
curl -X POST http://localhost:8080/file \
  -H "Content-Type: application/json" \
  -d '{"path": "/home/user/document.pdf"}'
```

**Supported Scenarios:**
- Text files (`.txt`, `.json`, `.xml`, etc.) - served with appropriate text MIME types
- Images (`.png`, `.jpg`, `.gif`, etc.) - served with image MIME types
- Documents (`.pdf`, `.docx`, etc.) - served with document MIME types
- Binary files - served as `application/octet-stream`

**Error Responses:**

When feature is disabled (404):
```
(Empty response body with 404 status)
```

File not found:
```json
{
  "success": false,
  "error_type": "file_not_found",
  "error_title": "File Not Found",
  "error_message": "File not found: /path/to/file.txt"
}
```

### POST /dir

Lists files and directories in a specified path. **This endpoint is disabled by default** and must be explicitly enabled with the `--enable-local-files` flag.

**⚠️ Security Warning:** Only enable this feature in trusted environments. This allows the proxy to list directory contents accessible to the process user.

**Request Body:**

```json
{
  "path": "/home/user"
}
```

For root directory, use `null`:
```json
{
  "path": null
}
```

**Response:**

Returns a JSON array of directory entries sorted with directories first, then files, alphabetically within each group:

```json
[
  {
    "name": "Documents",
    "type": "directory"
  },
  {
    "name": "Downloads",
    "type": "directory"
  },
  {
    "name": "file.txt",
    "type": "file"
  },
  {
    "name": "photo.jpg",
    "type": "file"
  }
]
```

**Platform Defaults:**

When `path` is `null`, the endpoint returns the root directory for the platform:
- **Unix/Linux/macOS**: `/`
- **Windows**: `C:\`

**Example:**

```bash
# Enable local file serving
requestbite-proxy --enable-local-files

# List directory contents
curl -X POST http://localhost:8080/dir \
  -H "Content-Type: application/json" \
  -d '{"path": "/home/user/Documents"}'

# List root directory
curl -X POST http://localhost:8080/dir \
  -H "Content-Type: application/json" \
  -d '{"path": null}'
```

**Error Responses:**

The `/dir` endpoint uses the same error response format as the `/file` endpoint:

When feature is disabled (404):
```
(Empty response body with 404 status)
```

Directory not found:
```json
{
  "success": false,
  "error_type": "file_not_found",
  "error_title": "File Not Found",
  "error_message": "Directory not found: /path/to/dir"
}
```

Path is not a directory:
```json
{
  "success": false,
  "error_type": "file_access_error",
  "error_title": "File Access Error",
  "error_message": "Path is not a directory: /path/to/file.txt"
}
```

## Testing

Run the functionality tests:

```bash
./tests.sh
```

This tests the main functionality of the proxy.

## How to run the proxy

Run the proxy in any of the two following supported modes:

### 1. Standalone HTTP Service

Run the proxy as a service and configure nginx to proxy to it:

```nginx
upstream proxy_backend {
    server localhost:8080;
}

location /proxy/ {
    proxy_pass http://proxy_backend;
    proxy_set_header Host $host;
    proxy_set_header X-Real-IP $remote_addr;
}
```

### 2. Direct CLI Usage

CLI version of proxy.

```bash
./proxy-go -port 8080
```

## Configuration

Command-line flags:

- `-port`: Server port (default: 8080)
- `-enable-local-files`: Enable local file serving via `/file` endpoint (default: false, **disabled for security**)
- `-help`: Show help information
- `-version`: Show version information

**Example:**
```bash
# Run with default settings
requestbite-proxy

# Run on custom port with local file serving enabled
requestbite-proxy -port 3000 -enable-local-files
```

## Loop Detection

The proxy implements multiple strategies to prevent infinite request loops:

### User-Agent Detection
Incoming requests with User-Agent header containing "rb-slingshot" are blocked. This prevents:
- Proxy calling itself
- Chained proxy instances
- Accidental infinite loops in self-hosted deployments

### Hostname Blocking
Requests targeting specific hostnames are blocked:
- `p.requestbite.com` (production)
- `dev.p.requestbite.com` (development)

Exception: Requests to `/health` endpoint are always allowed for health checks.

When a loop is detected, the proxy returns HTTP 508 Loop Detected with `error_type: "loop_detected"`.

**Testing Loop Protection:**
```bash
# This should return 508 Loop Detected
curl -X POST http://localhost:8080/proxy/request \
  -H "Content-Type: application/json" \
  -d '{
    "method": "POST",
    "url": "http://localhost:8080/proxy/request",
    "body": "{\"method\":\"GET\",\"url\":\"https://example.com\"}"
  }'
```

## Error Types

The proxy returns standardized error responses:

- `url_validation_error`: Invalid URL format or scheme
- `timeout`: Request exceeded specified timeout
- `connection_error`: Network connection failed
- `redirect_not_followed`: Redirect encountered but `followRedirects: false`
- `request_format_error`: Invalid JSON or missing required fields
- `loop_detected`: Request would create an infinite loop. Detected by:
  - User-Agent header matching `rb-slingshot` (any version, any instance)
  - Target hostname in blocked list (p.requestbite.com, dev.p.requestbite.com)
- `file_not_found`: Requested file does not exist (404)
- `file_access_error`: Cannot access file (permissions, is directory, etc.)
- `feature_disabled`: Attempted to use disabled feature

## Monitoring

Health check endpoint available at `/health`:

```bash
curl http://localhost:8080/health
```

Returns:

```json
{
  "status": "ok",
  "user-agent": "rb-slingshot/0.1.0 (https://requestbite.com/slingshot)",
  "version": "0.1.0"
}
```

## License

Same as the parent RequestBite project.
