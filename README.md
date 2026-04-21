[![Release](https://github.com/requestbite/proxy/actions/workflows/release.yml/badge.svg)](https://github.com/requestbite/proxy/actions/workflows/release.yml)

# RequestBite Slingshot Proxy

## About

The RequestBite Slingshot Proxy is a highly performant REST API written in Go
that can proxy HTTP requests for webapps and be used to read files and browse
directories on the machine on which it is installed.

It is used as a proxy server by [Slingshot](https://s.requestbite.com) as it's
almost impossible for webapps to directly make HTTP requests to arbitrary HTTP
resources because of CORS restrictions. If installed locally (with necessary
config options enabled), Slingshot will also do local file browsing in certain
situations (e.g. when importing or updating files).

The RequestBite Slingshot Proxy is hosted at
[p.requestbite.com](https://p.requestbite.com/health) but nothing prevents you
from running it yourself. Doing so means you won't proxy any requests via our
servers (unless you want to) and it means you can access resources normally not
reachable over the public Internet, such as those on your machine, on your local
network, or on any VPN you might be connected to.

## Installation

### Quick Install

> [!NOTE]
> While it's possible to install Proxy using the provided install script below,
> we do recommend installing [RBite
> CLI](https://github.com/requestbite/rbite#quick-install) which installs Proxy
> along with it.

Install the latest release on MacOS or Linux like so:

```bash
curl -fsSL https://raw.githubusercontent.com/requestbite/proxy/main/install.sh | bash
```

The binary will be installed to `~/.local/bin` by default.

### Custom Installation Directory

To install the latest release to a custom directory, do like so:

```bash
curl -fsSL https://raw.githubusercontent.com/requestbite/proxy/main/install.sh | bash -s -- --prefix=$HOME/bin
```

### Install Older Version

To install a specific version (in this example, version 0.3.1), do like so:

```bash
curl -fsSL https://raw.githubusercontent.com/requestbite/proxy/main/install.sh | bash -s -- --version=0.3.1
```

### Manual Download

Download pre-built binaries from [GitHub
Releases](https://github.com/requestbite/proxy/releases).

**Supported Platforms:**

| OS      | Architecture          | Binary Name                         |
|---------|-----------------------|-------------------------------------|
| macOS   | Intel (x86-64)        | `rbite-proxy-*-darwin-amd64.tar.gz` |
| macOS   | Apple Silicon (ARM64) | `rbite-proxy-*-darwin-arm64.tar.gz` |
| Linux   | x86-64                | `rbite-proxy-*-linux-amd64.tar.gz`  |
| Windows | x86-64                | `rbite-proxy-*-windows-amd64.zip`   |

After downloading, extract the archive and move the binary to a directory in
your PATH:

```bash
# macOS/Linux
tar -xzf rb-proxy-*.tar.gz
mv rb-proxy/rb-proxy ~/.local/bin/

# Make sure ~/.local/bin is in your PATH
export PATH="$HOME/.local/bin:$PATH"
```

## Usage

Full up-to-date documentation about what you can do with Proxy can be found
at <https://docs.requestbite.com/proxy/>.
