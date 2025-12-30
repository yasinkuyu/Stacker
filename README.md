# Stacker

A cross-platform local development environment for PHP applications with all Pro features. Built with Go for maximum performance.

**GitHub Repository**: https://github.com/yasinkuyu/Stacker

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

### Web UI & System Tray
- 🖥️ **Web Interface** - Modern dark-themed web dashboard
- 📱 **System Tray** - Status bar icon on all platforms
- 🎨 **Responsive Design** - Clean and intuitive interface
- 🚀 **Auto-start** - Start automatically on system login (macOS/Linux/Windows)

## Installation

### Download & Run (Recommended)

#### macOS
1. Download `.app` bundle from [Releases](https://github.com/yasinkuyu/Stacker/releases)
2. Double-click `Stacker.app` to run
3. System tray icon will appear in the menu bar

**Note**: For tray icon support on macOS, run the `.app` bundle (not the binary from terminal).

##### Auto-start on macOS
```bash
# Enable auto-start on login
./stacker startup enable

# Disable auto-start
./stacker startup disable
```

#### Linux & Windows
1. Download binary from [Releases](https://github.com/yasinkuyu/Stacker/releases)
2. Run from terminal:
```bash
chmod +x stacker
./stacker ui
```

##### Auto-start on Linux (Systemd)
```bash
# Enable auto-start
./stacker startup enable

# Disable auto-start
./stacker startup disable
```

##### Auto-start on Windows
```bash
# Enable auto-start
./stacker startup enable

# Disable auto-start
./stacker startup disable
```

### Build from Source
```bash
git clone https://github.com/yasinkuyu/Stacker.git
cd Stacker
./build.sh
```

The build script creates:
- macOS `.app` bundles (with tray icon)
- Standalone binaries (Linux/Windows)

## Quick Start

```bash
# Add a site
./stacker add myproject /path/to/project

# Start the server
./stacker serve

# Visit https://myproject.test
```

## Commands

### Site Management
```bash
# Add a new site
./stacker add <name> <path>

# List all sites
./stacker list

# Remove a site
./stacker remove <name>
```

### Server
```bash
# Start development server
./stacker serve

# Start Web UI with system tray
./stacker ui
```

### Dumps
```bash
# List all dumps
./stacker dumps list

# Clear all dumps
./stacker dumps clear
```

### Mail
```bash
# List all emails
./stacker mail list

# Clear all emails
./stacker mail clear
```

### Logs
```bash
# List all log files
./stacker logs list

# Tail a log file
./stacker logs tail <file>

# Search logs
./stacker logs search <query>
```

### Services
```bash
# List all services
./stacker services list

# Add a service
./stacker services add <name> <type> <port>

# Start a service
./stacker services start <name>

# Stop a service
./stacker services stop <name>

# Stop all services
./stacker services stop-all
```

### PHP Management
```bash
# List PHP versions
./stacker php list

# Set default PHP version
./stacker php set <version>
```

### Node.js Management
```bash
# List Node.js versions
./stacker node list

# Set default Node.js version
./stacker node set <version>

# Install a Node.js version
./stacker node install <version>
```

### XDebug
```bash
# Enable XDebug
./stacker xdebug enable

# Disable XDebug
./stacker xdebug disable
```

### Laravel Forge Integration
```bash
# List Forge servers
FORGE_API_KEY=your_key ./stacker forge servers

# Deploy a site
FORGE_API_KEY=your_key ./stacker forge deploy <server-id> <site-id>
```

### Status
```bash
# Show system status
./stacker status
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
