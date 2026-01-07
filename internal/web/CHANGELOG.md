# Changelog

## v1.1.0 (Jan 07, 2026)
- [feat] **MySQL Security:** Added Root Password configuration during installation and credential persistence.
- [feat] **Service Control:** Added "Start All" and "Stop All" buttons for global service management.
- [fix] **Dashboard Loading:** Resolved "No sites yet" issue by ensuring cache update on load.
- [fix] **Preferences:** Fixed zero-port issue in `preferences.json` by enforcing defaults (8080/80/3306).
- [improve] **UI Experience:** Added "Connection Lost" modal, status dots, and softer hover aesthetics.
- [feat] **Cross-Platform Power:** Full support for Windows, macOS, and Linux with platform-specific process and port management.
- [feat] **Service State Persistence:** Automatically remembers and restores active services upon application restart.
- [feat] **Smart Dependency Auto-start:** Intelligent service startup ordering (Database → PHP → Web Server) for reliable environment boot.
- [feat] **Unified Architecture:** Merged preferences between Web UI and Backend to ensure 100% data consistency.
- [feat] **Proactive 503 Prevention:** Smart PHP port detection to resolve Apache/Nginx connectivity issues automatically.
- [feat] **Dynamic Domain Extension:** System-wide support for configurable domain extensions, defaulting to `.local`.
- [improve] Windows support with `APPDATA` path handling and `taskkill` process termination.
- [improve] Optimized PHP-FPM configuration generation and binary detection logic.
- [improve] Added i18n support for "Auto-start Services" settings in 13 languages.

## v1.0.1 (Jan 07, 2026)
- [feat] Advanced Hosts File Manager with CRUD, backup/restore, import/export, and group filtering
- [feat] Hosts search bar and bulk delete with checkboxes  
- [feat] Custom confirmation modal for hosts operations
- [feat] Permission error handling with terminal command display
- [fix] Fixed domain extension handling for sites with dots (e.g., `local.instock`)
- [fix] Prevented double domain extension appending in Nginx/Apache configs
- [improve] Changed default domain extension to `.local`
- [improve] Site name hint now correctly previews custom domains
- [improve] System host entries protected from deletion

## v1.0.0 (Jan 02, 2026)
- [feat] Complete UI Redesign with 3-column layout and Glassmorphism
- [feat] Multi-language support (13 languages) with RTL for Arabic
- [feat] Service Management (Nginx, MySQL, MariaDB, Redis, PHP-FPM)
- [feat] Built-in SMTP Mail Catcher and Webmail Viewer
- [feat] Dynamic Dashboard with Real-time Service Stats
- [feat] Advanced Settings with Dark/Light Theme Persistence
- [fix] Improved Service Process Management and Logging
- [fix] Resolved Navigation and Active State issues
