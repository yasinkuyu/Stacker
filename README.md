# 🚀 Stacker

**Modern, High-Performance Local Development Environment for PHP, Nginx, Apache, and MySQL.**

Stacker is a powerful, cross-platform, standalone development environment designed to be the modern successor to traditional tools like XAMPP, MAMP, and Laragon. Built from the ground up in **Go**, it offers lightning-fast performance and a sleek, intuitive Web UI to manage your local sites and services.

[![Go Version](https://img.shields.io/github/go-mod/go-version/yasinkuyu/Stacker)](https://github.com/yasinkuyu/Stacker)
[![License](https://img.shields.io/github/license/yasinkuyu/Stacker)](LICENSE)
[![GitHub Stars](https://img.shields.io/github/stars/yasinkuyu/Stacker)](https://github.com/yasinkuyu/Stacker/stargazers)

---

## 🌟 Why Stacker?

Most local development tools are either too heavy, platform-locked, or outdated. Stacker solves this by providing a unified experience that is:

*   **⚡ Faster**: Powered by Go, the management layer is virtually instant.
*   **📦 Portable**: All services are self-contained in a dedicated data directory. No system-wide pollution.
*   **🌐 All-in-One**: Built-in Mail Catcher, Log Viewer, and Dump Interceptor.
*   **🛠️ Flexible**: Switch PHP versions, enable/disable XDebug, and manage multiple services with one click.

---

## 🚀 Key Features

### 🌐 Site & Server Management
*   **Instant Sites**: Add, list, and remove local sites in seconds.
*   **Automatic HTTPS**: Built-in SSL certificate management via `mkcert`.
*   **Hosts Management**: Automatically handles your `/etc/hosts` for `.test` domains.
*   **Modern Web UI**: A beautiful, responsive dark-themed dashboard to control everything.

### 🐘 PHP & Node.js Power
*   **Multi-PHP**: Easily install and switch between PHP versions (7.4 to 8.4).
*   **XDebug Pro**: One-click XDebug management for effortless debugging.
*   **Node.js Integration**: Built-in support for managing Node.js versions.

### ⚙️ Standalone Services
Stacker downloads and compiles services directly on your machine for maximum compatibility:
*   **Databases**: MariaDB (10.x, 11.x) and MySQL (5.7, 8.0).
*   **Web Servers**: Nginx and Apache.
*   **Cache**: Redis.

### 🛠️ Developer Tooling ("Pro" Features Included)
*   **📧 Mail Catcher**: Local SMTP server and viewer—never send a test email to a real user again.
*   **📦 Dump Interceptor**: Intercept and view `dump()` and `dd()` output in a clean UI.
*   **📄 Log Viewer**: Advanced log management with search and real-time tailing.
*   **🔗 Forge Integration**: Deploy your local projects to Laravel Forge directly from Stacker.

---

## 💻 Installation

### 🍏 macOS (Recommended)
1. Download the latest `.app` bundle from [Releases](https://github.com/yasinkuyu/Stacker/releases).
2. Move it to your `Applications` folder.
3. Run `Stacker.app` to use the **System Tray** application (best experience).

### 🐧 Linux & 🪟 Windows
1. Download the binary for your platform from [Releases](https://github.com/yasinkuyu/Stacker/releases).
2. Open your terminal and run:
   ```bash
   ./stacker ui
   ```
3. Open your browser to `http://localhost:9999`.

---

## 🛠️ Build from Source
If you prefer to build it yourself, ensure you have Go 1.19+ installed:
```bash
git clone https://github.com/yasinkuyu/Stacker.git
cd Stacker
./build.sh
```

---

## 📂 Data Directory Structure
Stacker keeps everything organized in a single location:
*   **macOS**: `~/Library/Application Support/Stacker/`
*   **Linux**: `~/.stacker/`
*   **Windows**: `%APPDATA%/Stacker/`

---

## 🛠️ Requirements & Dependencies
*   **Xcode Command Line Tools** (macOS) or **build-essential** (Linux) for service compilation.
*   **cmake** and **make** (for building MariaDB/MySQL).
*   **mkcert** (for SSL/HTTPS support).

---

## 🤝 Contributing
Stacker is open-source and we love contributions!
1. Fork the Project.
2. Create your Feature Branch (`git checkout -b feature/AmazingFeature`).
3. Commit your Changes (`git commit -m 'Add some AmazingFeature'`).
4. Push to the Branch (`git push origin feature/AmazingFeature`).
5. Open a Pull Request.

---

## 📄 License
Distributed under the MIT License. See `LICENSE` for more information.

---

**Developed with ❤️ by [Insya](https://github.com/insya)**

