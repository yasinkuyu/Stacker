package web

import (
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"runtime"
	"stacker-app/internal/config"
	"stacker-app/internal/dumps"
	"stacker-app/internal/logs"
	"stacker-app/internal/mail"
	"stacker-app/internal/php"
	"stacker-app/internal/services"
	"stacker-app/internal/tray"
	"stacker-app/internal/xdebug"
)

type WebServer struct {
	cfg         *config.Config
	port        int
	dumpManager *dumps.DumpManager
	mailManager *mail.MailManager
	logManager  *logs.LogManager
	svcManager  *services.ServiceManager
	phpManager  *php.PHPManager
	xdebugMgr   *xdebug.XDebugManager
	trayMgr     *tray.TrayManager
}

func NewWebServer(cfg *config.Config) *WebServer {
	return &WebServer{
		cfg:         cfg,
		port:        8080,
		dumpManager: dumps.NewDumpManager(cfg),
		mailManager: mail.NewMailManager(cfg),
		logManager:  logs.NewLogManager(),
		svcManager:  services.NewServiceManager(),
		phpManager:  php.NewPHPManager(),
		xdebugMgr:   xdebug.NewXDebugManager(),
		trayMgr:     tray.NewTrayManager(),
	}
}

func (ws *WebServer) setupRoutes() {
	http.HandleFunc("/static/logo.png", ws.logoHandler)
	http.HandleFunc("/", ws.indexHandler)
	http.HandleFunc("/api/status", ws.statusHandler)
	http.HandleFunc("/api/sites", ws.sitesHandler)
	http.HandleFunc("/api/dumps", ws.dumpsHandler)
	http.HandleFunc("/api/mail", ws.mailHandler)
	http.HandleFunc("/api/services", ws.servicesHandler)
	http.HandleFunc("/api/php", ws.phpHandler)
	http.HandleFunc("/api/xdebug", ws.xdebugHandler)
	http.HandleFunc("/api/logs", ws.logsHandler)
}

func (ws *WebServer) Start() error {
	ws.phpManager.DetectPHPVersions()
	ws.setupRoutes()

	webURL := fmt.Sprintf("http://localhost:%d", ws.port)
	ws.trayMgr.SetWebURL(webURL)

	fmt.Printf("\n📦 Stackr is running\n")
	fmt.Printf("🌐 Web UI: %s\n", webURL)
	fmt.Printf("⏹️  Press Ctrl+C to stop\n\n")

	if runtime.GOOS != "darwin" {
		go ws.trayMgr.Run()
	}
	return http.ListenAndServe(fmt.Sprintf(":%d", ws.port), nil)
}

func (ws *WebServer) Stop() {
}

func (ws *WebServer) indexHandler(w http.ResponseWriter, r *http.Request) {
	html := `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Stackr</title>
    <link rel="icon" type="image/png" href="/static/logo.png">
    <style>
        :root {
            --bg-primary: #0d0d14;
            --bg-secondary: #1a1a24;
            --bg-tertiary: #252532;
            --text-primary: #ffffff;
            --text-secondary: #8a8a9a;
            --accent: #6366f1;
            --success: #34d399;
            --border: #2a2a3a;
        }
        
        * { margin: 0; padding: 0; box-sizing: border-box; }
        
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'SF Pro Display', sans-serif;
            background: var(--bg-primary);
            color: var(--text-primary);
            font-size: 13px;
            min-height: 100vh;
        }
        
        .app-container { display: flex; min-height: 100vh; }
        
        .sidebar {
            width: 220px;
            background: var(--bg-secondary);
            border-right: 1px solid var(--border);
            padding: 16px 12px;
        }
        
        .logo {
            display: flex;
            align-items: center;
            gap: 10px;
            padding: 14px;
            font-size: 15px;
            font-weight: 600;
            color: var(--text-primary);
        }

        .logo-img {
            width: 32px;
            height: 32px;
            border-radius: 8px;
        }
        
        .nav-item {
            display: flex;
            align-items: center;
            gap: 12px;
            padding: 11px 14px;
            border-radius: 8px;
            color: var(--text-secondary);
            cursor: pointer;
            transition: all 0.15s ease;
            font-size: 13px;
            border: none;
            background: none;
            width: 100%;
            text-align: left;
        }
        
        .nav-item:hover { background: var(--bg-tertiary); color: var(--text-primary); }
        .nav-item.active { background: var(--accent); color: white; }
        
        .main-content { flex: 1; padding: 40px; }
        
        .page { display: none; }
        .page.active { display: block; }
        
        .stats-grid {
            display: grid;
            grid-template-columns: repeat(auto-fit, minmax(160px, 1fr));
            gap: 16px;
            margin-bottom: 36px;
        }
        
        .stat-card {
            background: var(--bg-secondary);
            border: 1px solid var(--border);
            border-radius: 12px;
            padding: 20px;
        }
        
        .stat-label {
            font-size: 11px;
            text-transform: uppercase;
            letter-spacing: 0.5px;
            color: var(--text-secondary);
            margin-bottom: 8px;
        }
        
        .stat-value {
            font-size: 32px;
            font-weight: 600;
            color: var(--text-primary);
        }
        
        .card {
            background: var(--bg-secondary);
            border: 1px solid var(--border);
            border-radius: 12px;
            overflow: hidden;
        }
        
        .card-header {
            display: flex;
            justify-content: space-between;
            align-items: center;
            padding: 16px 20px;
            border-bottom: 1px solid var(--border);
        }
        
        .card-title {
            font-size: 14px;
            font-weight: 600;
            color: var(--text-primary);
        }
        
        .list-item {
            display: flex;
            justify-content: space-between;
            align-items: center;
            padding: 16px 20px;
            border-bottom: 1px solid var(--border);
        }
        
        .item-primary { font-size: 13px; font-weight: 500; color: var(--text-primary); }
        .item-secondary { font-size: 12px; color: var(--text-secondary); margin-top: 4px; }
        
        .status-badge {
            display: inline-flex;
            align-items: center;
            gap: 6px;
            padding: 4px 10px;
            border-radius: 20px;
            font-size: 11px;
            font-weight: 500;
        }
        
        .status-running { color: var(--success); background: rgba(52, 211, 153, 0.1); }
        
        .empty-state {
            text-align: center;
            padding: 48px 32px;
            color: var(--text-secondary);
        }
        
        .btn {
            display: inline-flex;
            align-items: center;
            gap: 6px;
            padding: 8px 16px;
            border-radius: 8px;
            font-size: 13px;
            font-weight: 500;
            border: none;
            cursor: pointer;
            transition: all 0.15s ease;
        }
        
        .btn-primary { background: var(--accent); color: white; }
        .btn-danger { background: rgba(248, 113, 113, 0.1); color: #f87171; }
    </style>
</head>
<body>
    <div class="app-container">
        <aside class="sidebar">
            <div class="logo">
                <img src="/static/logo.png" class="logo-img" alt="Stackr">
                <span>Stackr</span>
            </div>
            <nav>
                <button class="nav-item active" data-page="dashboard">📊 Dashboard</button>
                <button class="nav-item" data-page="sites">🌐 Sites</button>
                <button class="nav-item" data-page="services">⚙️ Services</button>
                <button class="nav-item" data-page="dumps">💾 Dumps</button>
                <button class="nav-item" data-page="mail">📧 Mail</button>
                <button class="nav-item" data-page="logs">📄 Logs</button>
                <button class="nav-item" data-page="php">🐘 PHP</button>
            </nav>
        </aside>
        
        <main class="main-content">
            <div id="dashboard" class="page active">
                <h1 style="font-size: 22px; font-weight: 600; margin-bottom: 28px;">Dashboard</h1>
                
                <div class="stats-grid">
                    <div class="stat-card">
                        <div class="stat-label">Sites</div>
                        <div class="stat-value" id="stat-sites">0</div>
                    </div>
                    <div class="stat-card">
                        <div class="stat-label">Running</div>
                        <div class="stat-value" id="stat-services">0</div>
                    </div>
                    <div class="stat-card">
                        <div class="stat-label">Dumps</div>
                        <div class="stat-value" id="stat-dumps">0</div>
                    </div>
                    <div class="stat-card">
                        <div class="stat-label">PHP</div>
                        <div class="stat-value" id="stat-php">-</div>
                    </div>
                </div>
                
                <div class="card">
                    <div class="card-header">
                        <span class="card-title">Quick Actions</span>
                    </div>
                    <div style="padding: 16px 20px; display: flex; gap: 8px;">
                        <button class="btn btn-primary" onclick="location.reload()">Refresh</button>
                    </div>
                </div>
            </div>
            
            <div id="sites" class="page">
                <h1 style="font-size: 22px; font-weight: 600; margin-bottom: 28px;">Sites</h1>
                <div class="card"><div id="sites-list"></div></div>
            </div>
            
            <div id="services" class="page">
                <h1 style="font-size: 22px; font-weight: 600; margin-bottom: 28px;">Services</h1>
                <div class="card"><div id="services-list"></div></div>
            </div>
            
            <div id="dumps" class="page">
                <h1 style="font-size: 22px; font-weight: 600; margin-bottom: 28px;">Dumps</h1>
                <div class="card">
                    <div class="card-header">
                        <span class="card-title">All Dumps</span>
                        <button class="btn btn-danger" onclick="clearDumps()">Clear</button>
                    </div>
                    <div id="dumps-list"></div>
                </div>
            </div>
            
            <div id="mail" class="page">
                <h1 style="font-size: 22px; font-weight: 600; margin-bottom: 28px;">Mail</h1>
                <div class="card">
                    <div class="card-header">
                        <span class="card-title">Inbox</span>
                        <button class="btn btn-danger" onclick="clearMail()">Clear</button>
                    </div>
                    <div id="mail-list"></div>
                </div>
            </div>
            
            <div id="logs" class="page">
                <h1 style="font-size: 22px; font-weight: 600; margin-bottom: 28px;">Logs</h1>
                <div class="card"><div id="logs-list"></div></div>
            </div>
            
            <div id="php" class="page">
                <h1 style="font-size: 22px; font-weight: 600; margin-bottom: 28px;">PHP Versions</h1>
                <div class="card"><div id="php-list"></div></div>
            </div>
        </main>
    </div>
    
    <script>
        document.querySelectorAll('.nav-item').forEach(item => {
            item.addEventListener('click', () => showPage(item.dataset.page));
        });
        
        function showPage(pageId) {
            document.querySelectorAll('.page').forEach(p => p.classList.remove('active'));
            document.querySelectorAll('.nav-item').forEach(n => n.classList.remove('active'));
            document.getElementById(pageId).classList.add('active');
            document.querySelector('[data-page="'+pageId+'"]').classList.add('active');
            loadPage(pageId);
        }
        
        async function api(endpoint) {
            const res = await fetch('/api' + endpoint);
            return res.json();
        }
        
        async function loadPage(pageId) {
            switch(pageId) {
                case 'dashboard': await loadStatus(); break;
                case 'sites': await loadSites(); break;
                case 'services': await loadServices(); break;
                case 'dumps': await loadDumps(); break;
                case 'mail': await loadMail(); break;
                case 'logs': await loadLogs(); break;
                case 'php': await loadPHP(); break;
            }
        }
        
        async function loadStatus() {
            const data = await api('/status');
            document.getElementById('stat-sites').textContent = data.sites || 0;
            document.getElementById('stat-services').textContent = data.services || 0;
            document.getElementById('stat-dumps').textContent = data.dumps || 0;
            document.getElementById('stat-php').textContent = data.php || '-';
        }
        
        async function loadSites() {
            const list = document.getElementById('sites-list');
            const sites = await api('/sites');
            if (!sites.length) {
                list.innerHTML = '<div class="empty-state">No sites configured</div>';
                return;
            }
            list.innerHTML = sites.map(s => '<div class="list-item"><div><div class="item-primary">'+s.name+'.test</div><div class="item-secondary">'+s.path+'</div></div><span class="status-badge status-running">Running</span></div>').join('');
        }
        
        async function loadServices() {
            const list = document.getElementById('services-list');
            const services = await api('/services');
            if (!services.length) {
                list.innerHTML = '<div class="empty-state">No services configured</div>';
                return;
            }
            list.innerHTML = services.map(s => '<div class="list-item"><div><div class="item-primary">'+s.name+'</div><div class="item-secondary">'+s.type+' • Port '+s.port+'</div></div><span class="status-badge status-'+s.status+'">'+s.status+'</span></div>').join('');
        }
        
        async function loadDumps() {
            const list = document.getElementById('dumps-list');
            const data = await api('/dumps');
            if (!data.dumps || !data.dumps.length) {
                list.innerHTML = '<div class="empty-state">No dumps recorded</div>';
                return;
            }
            list.innerHTML = data.dumps.map(d => '<div class="list-item"><div><div class="item-primary">'+d.type+'</div><div class="item-secondary">'+d.file+':'+d.line+'</div></div><span class="item-secondary">'+new Date(d.timestamp).toLocaleTimeString()+'</span></div>').join('');
        }
        
        async function loadMail() {
            const list = document.getElementById('mail-list');
            const emails = await api('/mail');
            if (!emails.length) {
                list.innerHTML = '<div class="empty-state">No emails</div>';
                return;
            }
            list.innerHTML = emails.map(e => '<div class="list-item"><div><div class="item-primary">'+e.subject+'</div><div class="item-secondary">'+e.from+'</div></div><span class="item-secondary">'+new Date(e.timestamp).toLocaleTimeString()+'</span></div>').join('');
        }
        
        async function loadLogs() {
            const list = document.getElementById('logs-list');
            const logs = await api('/logs');
            if (!logs.length) {
                list.innerHTML = '<div class="empty-state">No logs found</div>';
                return;
            }
            list.innerHTML = logs.map(l => '<div class="list-item"><div><div class="item-primary">'+l.name+'</div><div class="item-secondary">'+l.site+'</div></div><span class="item-secondary">'+new Date(l.modified).toLocaleTimeString()+'</span></div>').join('');
        }
        
        async function loadPHP() {
            const list = document.getElementById('php-list');
            const data = await api('/php');
            if (!data.versions || !data.versions.length) {
                list.innerHTML = '<div class="empty-state">No PHP versions found</div>';
                return;
            }
            list.innerHTML = data.versions.map(v => '<div class="list-item"><div><div class="item-primary">'+v.version+'</div><div class="item-secondary">'+v.path+'</div></div>'+(v.default?'<span class="status-badge status-running">Default</span>':'')+'</div>').join('');
        }
        
        async function clearDumps() {
            if (confirm('Clear all dumps?')) {
                await fetch('/api/dumps', { method: 'DELETE' });
                loadDumps();
            }
        }
        
        async function clearMail() {
            if (confirm('Clear all emails?')) {
                await fetch('/api/mail', { method: 'DELETE' });
                loadMail();
            }
        }
        
        loadStatus();
        setInterval(loadStatus, 5000);
    </script>
</body>
</html>`
	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(html))
}

func (ws *WebServer) statusHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"sites":    len(ws.cfg.Sites),
		"services": ws.countRunningServices(),
		"dumps":    len(ws.dumpManager.GetDumps()),
		"php":      ws.getPHPVersion(),
	})
}

func (ws *WebServer) sitesHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(ws.cfg.Sites)
}

func (ws *WebServer) dumpsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == "DELETE" {
		ws.dumpManager.ClearDumps()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"dumps": ws.dumpManager.GetDumps()})
}

func (ws *WebServer) mailHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == "DELETE" {
		ws.mailManager.ClearEmails()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(ws.mailManager.LoadEmails())
}

func (ws *WebServer) servicesHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(ws.svcManager.GetServices())
}

func (ws *WebServer) phpHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"versions": ws.phpManager.GetVersions(),
		"default":  ws.phpManager.GetDefault(),
	})
}

func (ws *WebServer) xdebugHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"enabled": ws.xdebugMgr.IsEnabled(),
		"port":    ws.xdebugMgr.GetPort(),
	})
}

func (ws *WebServer) logsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(ws.logManager.GetLogFiles())
}

func (ws *WebServer) countRunningServices() int {
	count := 0
	for _, svc := range ws.svcManager.GetServices() {
		if svc.Status == "running" {
			count++
		}
	}
	return count
}

func (ws *WebServer) getPHPVersion() string {
	if php := ws.phpManager.GetDefault(); php != nil {
		return php.Version
	}
	return ""
}

func (ws *WebServer) logoHandler(w http.ResponseWriter, r *http.Request) {
	_, filename, _, _ := runtime.Caller(0)
	dir := filepath.Dir(filename)
	logoPath := filepath.Join(dir, "logo.png")

	w.Header().Set("Content-Type", "image/png")
	http.ServeFile(w, r, logoPath)
}
