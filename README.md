# Stacker

**A cross-platform, standalone local development environment for PHP applications with all Pro features. Built with Go for maximum performance.**

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
- ⚙️ **Services** - MariaDB, MySQL, Nginx, Apache, Redis
- 🐘 **PHP Management** - Multiple PHP versions (7.4-8.4) with XDebug support
- 🎯 **XDebug** - Automatic XDebug detection and management
- 📦 **Node.js** - NVM integration for Node.js version management
- 🔗 **Forge Integration** - Deploy to Laravel Forge
- 📝 **Stacker Config** - `stacker.yml` for project configuration

### ✨ Standalone Features
- 🚀 **No System Dependencies** - All services run independently in Stacker's data directory
- 📦 **Source Compilation** - Downloads and compiles services from source code
- 🔄 **Version Management** - Install multiple versions of each service
- 📊 **PID Tracking** - Monitor and manage service processes
- 💾 **Database Initialization** - Auto-setup databases with configuration
- 📱 **Tray Status** - Real-time service status in system tray

### Web UI & System Tray
- 🖥️ **Web Interface** - Modern dark-themed web dashboard
- 📱 **System Tray** - Status bar icon on all platforms with live service indicators
- 🎨 **Responsive Design** - Clean and intuitive interface
- 🚀 **Auto-start** - Start automatically on system login (macOS/Linux/Windows)

## Installation

### Download & Run (Recommended)

#### macOS
1. Download `.app` bundle from [Releases](https://github.com/yasinkuyu/Stacker/releases)
2. Double-click `Stacker.app` to run
3. System tray icon will appear in menu bar

**Note**: For tray icon support on macOS, run `.app` bundle (not binary from terminal).

#### Linux & Windows
1. Download binary from [Releases](https://github.com/yasinkuyu/Stacker/releases)
2. Run from terminal:
```bash
chmod +x stacker
./stacker ui
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
# Start Web UI
./stacker ui

# Or start as tray app
./stacker tray

# Open browser: http://localhost:8080
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

# Start Web UI
./stacker ui

# Start tray app
./stacker tray
```

### Services (Standalone)
```bash
# List available service versions
./stacker services versions
./stacker services versions mariadb

# Install a service (downloads, compiles, configures)
./stacker services install mariadb 11.2
./stacker services install nginx 1.25
./stacker services install redis 7.2
./stacker services install apache 2.4

# List installed services
./stacker services list

# Start a service
./stacker services start mariadb-11.2

# Stop a service
./stacker services stop mariadb-11.2

# Restart a service
./stacker services restart mariadb-11.2

# Uninstall a service
./stacker services uninstall mariadb-11.2

# Start all services
./stacker services start-all

# Stop all services
./stacker services stop-all
```

### PHP Management
```bash
# List PHP versions
./stacker php list

# Install PHP
./stacker php install 8.3

# Set default PHP version
./stacker php set 8.3
```

### Node.js Management
```bash
# List Node.js versions
./stacker node list

# Set default Node.js version
./stacker node set 18.0

# Install a Node.js version
./stacker node install 18.0
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

## Supported Services

| Service | Versions | Install Type |
|---------|----------|--------------|
| MariaDB | 11.2, 10.11, 10.6 | Source → Compile |
| MySQL | 8.0, 5.7 | Source → Compile |
| Nginx | 1.25, 1.24 | Source → Compile |
| Apache | 2.4 | Source → Compile |
| Redis | 7.2, 7.0 | Source → Compile |

## Configuration

### stacker.yml

Create a `stacker.yml` file in your project root:

```yaml
php: "8.3"
services:
  - type: mariadb
    version: "11.2"
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

## Data Directory

All services and data are stored independently:

```
~/Library/Application Support/Stacker/    # macOS
~/.stacker/                               # Linux
%APPDATA%/Stacker/                      # Windows

├── bin/              # Service binaries
│   ├── mariadb/     # Compiled MariaDB
│   ├── mysql/       # Compiled MySQL
│   ├── nginx/       # Compiled Nginx
│   ├── apache/      # Compiled Apache
│   └── redis/       # Compiled Redis
├── conf/             # Configuration files
├── data/             # Data files (databases, cache, etc)
├── logs/             # Log files
├── pids/             # Process IDs
├── sites.json        # Site configuration
└── services.json     # Service status
```

## Environment Variables

- `STACKER_CONFIG` - Path to config file (default: `$HOME/.stacker-app/config.yaml`)
- `FORGE_API_KEY` - Laravel Forge API key for deployment

## Requirements

- Go 1.19+ (if building from source)
- PHP 7.4-8.4 (installed on system)
- mkcert (for HTTPS support)
- NVM (optional, for Node.js management)
- cmake (required for MariaDB/MySQL compilation)
- make (required for service compilation)

## Cross-Platform Support

- ✅ macOS (Apple Silicon & Intel)
- ✅ Linux
- ✅ Windows

## License

MIT License

## Contributing

Contributions are welcome! Please open an issue or submit a pull request.

