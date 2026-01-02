# Stacker Project Roadmap

Current development status and stability goals.

## 🚀 Current Status

| Feature | Progress | Status | Description |

| Feature | Progress | Status | Description |
| :--- | :---: | :---: | :--- |
| **User Interface (UI)** | 99% | ✅ | Modern layout implemented, nearly complete. |
| **Localization (i18n)** | 50% | 🏗️ | Implementation in progress (Core ready, need more languages). |
| **Theme System** | 99% | ✅ | Light/Dark mode fully functional with persistence. |
| **Service Management** | 80% | 🔧 | Major features implemented, focus on reliability. |
| **PHP Management** | 30% | 🏗️ | Basic listing only. Setup needs work. |
| **Site Management** | 40% | 🏗️ | Basic listing. Creation logic needs stabilization. |
| **Mail Catcher** | 50% | 🏗️ | Basic functionality implemented. |
| **Database Dumps** | 50% | 🏗️ | Core export/import logic in development. |
| **Logs Viewer** | 70% | 🏗️ | Functional log tailing and filtered search. |

## 🗺️ Focus Areas (Stability First)

### 🛠️ Phase 1: Core Reliability (v1.1.0)
- [ ] **Service Manager Refactor:** Improve PID tracking and process state detection (current pain point).
- [ ] **Port Conflict Resolution:** Better detection of ports already in use by system services.
- [ ] **Error Handling:** Detailed error messages for failed service starts in the UI.
- [ ] **Config Validation:** Check syntax of config files before applying changes.

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
*Last updated: January 2, 2026*
