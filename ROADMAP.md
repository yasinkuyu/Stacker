# Stacker Project Roadmap

Current development status and stability goals.

## 🚀 Current Status

| Feature | Progress | Status | Description |
| :--- | :---: | :---: | :--- |
| **User Interface (UI)** | 100% | ✅ | Modern Glassmorphism layout fully implemented. |
| **Localization (i18n)** | 99% | ✅ | Full support for 13 languages with dynamic updates. |
| **Theme System** | 100% | ✅ | Light/Dark mode fully functional with cross-component persistence. |
| **Service Management** | 95% | ✅ | Cross-platform, persistence, and dependency boot ready. |
| **PHP Management** | 75% | 🔧 | Smart port detection and auto-FPM configuration ready. |
| **Site Management** | 70% | 🔧 | Dynamic domain extensions and hosts CRUD ready. |
| **Mail Catcher** | 60% | 🏗️ | SMTP interceptor and UI viewer functional. |
| **Logs Viewer** | 80% | ✅ | Advanced real-time tailing and filtered search. |

## 🗺️ Focus Areas (Stability First)

### ✅ Phase 1: Core Reliability (v1.1.0) - COMPLETED
- [x] **Service Manager Refactor:** Improved PID tracking, cross-platform signal management (SIGKILL/taskkill).
- [x] **Port Conflict Resolution:** Intelligent detection via `lsof`/`netstat` and proactive port clearing.
- [x] **State Persistence:** Restore previously active services on restart.
- [x] **Unified Settings:** Centralized preference management between Web and Backend.

### ⚡ Phase 2: UX Improvements (v1.2.0)
- [ ] **Service Auto-Recovery:** Attempt to restart services if they crash unexpectedly.
- [ ] **Site Configuration Wizard:** Guided setup for common CMS (WP, Laravel).
- [ ] **PHP Extensions:** Simple UI to enable/disable common PHP extensions.
- [ ] **Unified Search:** Search across sites, services, and logs from one place.

### 🚀 Phase 3: Extension (v1.3.0)
- [ ] **Custom Service Additions:** Ability to add custom binaries as managed services.
- [ ] **Import/Export Settings:** Easily migration Stacker settings between machines.
- [ ] **Health Monitoring:** Basic CPU/RAM monitoring per managed service.

---
*Last updated: January 7, 2026*
