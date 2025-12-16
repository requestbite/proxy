# RequestBite Slingshot Proxy

## About

Since it's almost impossible for webapps (such as
[Slingshot](https://s.requestbite.com)) to successfully make HTTP requests to
arbitrary HTTP resources because of CORS restrictions, a proxy is needed. This
repo holds the proxy used by Slingshot. It's written in Go and hosted at
[p.requestbite.com](https://p.requestbite.com/health) which is the one used by
default by Slingshot, but nothing prevents you from running it yourself (and
configuring Slingshot to use it).

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

Environment variables and configuration options can be added as needed.
Currently supports:

- `-port`: Server port (default: 8080)
- `-help`: Show help information
- `-version`: Show version information

## Error Types

The proxy returns standardized error responses:

- `url_validation_error`: Invalid URL format or scheme
- `timeout`: Request exceeded specified timeout
- `connection_error`: Network connection failed
- `redirect_not_followed`: Redirect encountered but `followRedirects: false`
- `request_format_error`: Invalid JSON or missing required fields
- `loop_detected`: If proxy is called to call itself

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
