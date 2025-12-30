# Stacker App

A cross-platform local development environment for PHP applications with all Pro features. Built with Go for maximum performance.

## Features

### Core Features
- 🌐 **Site Management** - Add, list, and remove sites instantly
- 🚀 **Fast Server** - Built-in HTTP/HTTPS server with mkcert support
- 🔒 **HTTPS Support** - Automatic SSL certificates with mkcert
- 📋 **Hosts File** - Automatic hosts file management

### Pro Features
- 📦 **Dumps** - Intercept and view `dump()` and `dd()` calls
- 📧 **Mail Service** - Local email catcher and viewer
- 📄 **Log Viewer** - View, search, and tail log files
- ⚙️ **Services** - MySQL, PostgreSQL, Redis, Meilisearch, MinIO
- 🐘 **PHP Management** - Multiple PHP versions (7.4-8.4) with XDebug support
- 🎯 **XDebug** - Automatic XDebug detection and management
- 📦 **Node.js** - NVM integration for Node.js version management
- 🔗 **Forge Integration** - Deploy to Laravel Forge
- 📝 **Stacker Config** - `stacker.yml` for project configuration

## Installation

### Download Binaries
```bash
# macOS (Apple Silicon)
curl -L https://releases.stacker.app/latest/darwin-arm64 -o stacker-app
chmod +x stacker-app

# macOS (Intel)
curl -L https://releases.stacker.app/latest/darwin-amd64 -o stacker-app
chmod +x stacker-app

# Linux
curl -L https://releases.stacker.app/latest/linux-amd64 -o stacker-app
chmod +x stacker-app

# Windows
curl -L https://releases.stacker.app/latest/windows-amd64.exe -o stacker-app.exe
```

### Build from Source
```bash
git clone https://github.com/yourusername/stacker-app.git
cd stacker-app
go build -o stacker-app main.go
```

## Quick Start

```bash
# Add a site
./stacker-app add myproject /path/to/project

# Start the server
./stacker-app serve

# Visit https://myproject.test
```

## Commands

### Site Management
```bash
# Add a new site
./stacker-app add <name> <path>

# List all sites
./stacker-app list

# Remove a site
./stacker-app remove <name>
```

### Server
```bash
# Start the development server
./stacker-app serve
```

### Dumps
```bash
# List all dumps
./stacker-app dumps list

# Clear all dumps
./stacker-app dumps clear
```

### Mail
```bash
# List all emails
./stacker-app mail list

# Clear all emails
./stacker-app mail clear
```

### Logs
```bash
# List all log files
./stacker-app logs list

# Tail a log file
./stacker-app logs tail <file>

# Search logs
./stacker-app logs search <query>
```

### Services
```bash
# List all services
./stacker-app services list

# Add a service
./stacker-app services add <name> <type> <port>

# Start a service
./stacker-app services start <name>

# Stop a service
./stacker-app services stop <name>

# Stop all services
./stacker-app services stop-all
```

### PHP Management
```bash
# List PHP versions
./stacker-app php list

# Set default PHP version
./stacker-app php set <version>
```

### Node.js Management
```bash
# List Node.js versions
./stacker-app node list

# Set default Node.js version
./stacker-app node set <version>

# Install a Node.js version
./stacker-app node install <version>
```

### XDebug
```bash
# Enable XDebug
./stacker-app xdebug enable

# Disable XDebug
./stacker-app xdebug disable
```

### Laravel Forge Integration
```bash
# List Forge servers
FORGE_API_KEY=your_key ./stacker-app forge servers

# Deploy a site
FORGE_API_KEY=your_key ./stacker-app forge deploy <server-id> <site-id>
```

### Status
```bash
# Show system status
./stacker-app status
```

## Configuration

### stacker.yml

Create a `stacker.yml` file in your project root:

```yaml
php: "8.3"
services:
  - type: mysql
    version: "8.0"
    port: 3306
  - type: redis
    port: 6379

forge:
  server_id: "12345"
  site_id: "67890"

env:
  APP_ENV: "local"
  APP_DEBUG: "true"
```

## Environment Variables

- `STACKER_CONFIG` - Path to config file (default: `$HOME/.stacker-app/config.yaml`)
- `FORGE_API_KEY` - Laravel Forge API key for deployment

## Requirements

- Go 1.19+ (if building from source)
- PHP 7.4-8.4 (installed on system)
- mkcert (for HTTPS support)
- NVM (optional, for Node.js management)

## Cross-Platform Support

- ✅ macOS (Apple Silicon & Intel)
- ✅ Linux
- ✅ Windows

## License

MIT License

## Contributing

Contributions are welcome! Please open an issue or submit a pull request.
