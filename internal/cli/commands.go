package cli

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/yasinkuyu/Stacker/internal/config"
	"github.com/yasinkuyu/Stacker/internal/dumps"
	"github.com/yasinkuyu/Stacker/internal/forge"
	"github.com/yasinkuyu/Stacker/internal/logs"
	"github.com/yasinkuyu/Stacker/internal/mail"
	"github.com/yasinkuyu/Stacker/internal/node"
	"github.com/yasinkuyu/Stacker/internal/php"
	"github.com/yasinkuyu/Stacker/internal/server"
	"github.com/yasinkuyu/Stacker/internal/services"
	"github.com/yasinkuyu/Stacker/internal/tray"
	"github.com/yasinkuyu/Stacker/internal/utils"
	"github.com/yasinkuyu/Stacker/internal/web"
	"github.com/yasinkuyu/Stacker/internal/xdebug"

	"github.com/spf13/cobra"
)

var cfgFile string

var rootCmd = &cobra.Command{
	Use:   "stacker",
	Short: "A full-stack development environment for PHP",
	Long:  `A cross-platform local development environment for PHP applications with all advanced features.`,
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) == 0 {
			// No command specified, run as tray app by default
			trayAppCmd.Run(trayAppCmd, args)
		}
	},
}

var trayAppCmd = &cobra.Command{
	Use:   "tray",
	Short: "Start as menu bar (tray) application - no dock icon",
	Run: func(cmd *cobra.Command, args []string) {
		cfg := config.Load(cfgFile)
		ws := web.NewWebServer(cfg)

		// Start web server in background
		go func() {
			if err := ws.Start(); err != nil {
				fmt.Printf("Error starting server: %v\n", err)
				os.Exit(1)
			}
		}()

		// Run tray manager (this blocks until quit)
		tm := tray.NewTrayManager()
		tm.SetWebURL("http://localhost:8080")
		tm.Run()
	},
}

var uiCmd = &cobra.Command{
	Use:   "ui",
	Short: "Start web UI (opens in browser, shows in dock)",
	Run: func(cmd *cobra.Command, args []string) {
		cfg := config.Load(cfgFile)
		ws := web.NewWebServer(cfg)

		// Start web server in background
		go func() {
			if err := ws.Start(); err != nil {
				fmt.Printf("Error starting server: %v\n", err)
				os.Exit(1)
			}
		}()

		// Open browser
		go tray.OpenBrowser("http://localhost:8080")

		// Run tray manager (blocks)
		tm := tray.NewTrayManager()
		tm.SetWebURL("http://localhost:8080")
		tm.Run()
	},
}

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the development server",
	Run: func(cmd *cobra.Command, args []string) {
		cfg := config.Load(cfgFile)
		srv := server.NewServer(cfg)
		go srv.Start()
		fmt.Printf("🚀 Server started on https://localhost:443\n")
		fmt.Println("Press Ctrl+C to stop")
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
		<-sigChan
		fmt.Println("\n🛑 Stopping server...")
		srv.Stop()
	},
}

var addCmd = &cobra.Command{
	Use:   "add [name] [path]",
	Short: "Add a new site",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		name := args[0]
		path := args[1]
		cfg := config.Load(cfgFile)
		cfg.AddSite(name, path)
		stackerConfig, _ := config.LoadStackerYaml(path)
		if stackerConfig.PHP != "" {
			pm := php.NewPHPManager()
			pm.PinSite(name, stackerConfig.PHP)
		}
		if err := cfg.Save(); err != nil {
			fmt.Printf("❌ Failed to add site: %v\n", err)
			return
		}
		utils.AddToHosts(name + ".test")
		fmt.Printf("✅ Site added: %s -> %s\n", name, path)
	},
}

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all sites",
	Run: func(cmd *cobra.Command, args []string) {
		cfg := config.Load(cfgFile)
		if len(cfg.Sites) == 0 {
			fmt.Println("No sites configured")
			return
		}
		fmt.Println("Configured sites:")
		for _, site := range cfg.Sites {
			fmt.Printf("  🌐 %s.test -> %s\n", site.Name, site.Path)
		}
	},
}

var removeCmd = &cobra.Command{
	Use:   "remove [name]",
	Short: "Remove a site",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		name := args[0]
		cfg := config.Load(cfgFile)
		if !cfg.RemoveSite(name) {
			fmt.Printf("❌ Site not found: %s\n", name)
			return
		}
		if err := cfg.Save(); err != nil {
			fmt.Printf("❌ Failed to remove site: %v\n", err)
			return
		}
		utils.RemoveFromHosts(name + ".test")
		fmt.Printf("✅ Site removed: %s\n", name)
	},
}

var dumpsCmd = &cobra.Command{
	Use:   "dumps",
	Short: "View and manage dumps",
}

var dumpsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all dumps",
	Run: func(cmd *cobra.Command, args []string) {
		cfg := config.Load(cfgFile)
		dm := dumps.NewDumpManager(cfg)
		allDumps := dm.GetDumps()
		if len(allDumps) == 0 {
			fmt.Println("No dumps recorded")
			return
		}
		for _, dump := range allDumps {
			fmt.Printf("📦 [%s] %s:%d\n", dump.Type, dump.File, dump.Line)
			fmt.Printf("   %s\n", dump.Timestamp.Format("15:04:05"))
		}
	},
}

var dumpsClearCmd = &cobra.Command{
	Use:   "clear",
	Short: "Clear all dumps",
	Run: func(cmd *cobra.Command, args []string) {
		cfg := config.Load(cfgFile)
		dm := dumps.NewDumpManager(cfg)
		dm.ClearDumps()
		fmt.Println("🗑️  All dumps cleared")
	},
}

var mailCmd = &cobra.Command{
	Use:   "mail",
	Short: "View and manage emails",
}

var mailListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all emails",
	Run: func(cmd *cobra.Command, args []string) {
		cfg := config.Load(cfgFile)
		mm := mail.NewMailManager(cfg)
		emails := mm.LoadEmails()
		if len(emails) == 0 {
			fmt.Println("📭 No emails received")
			return
		}
		fmt.Printf("📬 %d emails (%d unread)\n\n", mm.GetEmailCount(), mm.GetUnreadCount())
		fmt.Println(mm.FormatEmailList())
	},
}

var mailClearCmd = &cobra.Command{
	Use:   "clear",
	Short: "Clear all emails",
	Run: func(cmd *cobra.Command, args []string) {
		cfg := config.Load(cfgFile)
		mm := mail.NewMailManager(cfg)
		mm.ClearEmails()
		fmt.Println("🗑️  All emails cleared")
	},
}

var logsCmd = &cobra.Command{
	Use:   "logs",
	Short: "View and search logs",
}

var logsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all log files",
	Run: func(cmd *cobra.Command, args []string) {
		lm := logs.NewLogManager()
		logFiles := lm.GetLogFiles()
		if len(logFiles) == 0 {
			fmt.Println("No log files found")
			return
		}
		fmt.Println("Log files:")
		for _, file := range logFiles {
			fmt.Printf("  📄 %s (%s) - %s\n", file.Name, file.Site, file.Modified.Format("15:04:05"))
		}
	},
}

var logsTailCmd = &cobra.Command{
	Use:   "tail [file]",
	Short: "Tail a log file",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		logPath := args[0]
		lm := logs.NewLogManager()
		fmt.Printf("📋 Tailing %s...\n", logPath)
		lm.TailLog(logPath, func(entry logs.LogEntry) {
			fmt.Printf("[%s] %s: %s\n", entry.Timestamp.Format("15:04:05"), entry.Level, entry.Message)
		})
	},
}

var logsSearchCmd = &cobra.Command{
	Use:   "search [query]",
	Short: "Search logs",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		query := args[0]
		lm := logs.NewLogManager()
		results := lm.SearchLogs(query)
		if len(results) == 0 {
			fmt.Printf("No results for: %s\n", query)
			return
		}
		fmt.Printf("🔍 Found %d results for '%s':\n\n", len(results), query)
		fmt.Println(lm.FormatLogs(results))
	},
}

var servicesCmd = &cobra.Command{
	Use:   "services",
	Short: "Manage services",
}

var servicesListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all services",
	Run: func(cmd *cobra.Command, args []string) {
		sm := services.NewServiceManager()
		fmt.Println(sm.FormatStatus())
	},
}

var servicesAddCmd = &cobra.Command{
	Use:   "add [name] [type] [port]",
	Short: "Add a service",
	Args:  cobra.ExactArgs(3),
	Run: func(cmd *cobra.Command, args []string) {
		name := args[0]
		svcType := args[1]
		port := 0
		fmt.Sscanf(args[2], "%d", &port)
		sm := services.NewServiceManager()
		sm.AddService(&services.Service{
			Name: name,
			Type: svcType,
			Port: port,
		})
		fmt.Printf("✅ Service added: %s (%s)\n", name, svcType)
	},
}

var servicesStartCmd = &cobra.Command{
	Use:   "start [name]",
	Short: "Start a service",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		name := args[0]
		sm := services.NewServiceManager()
		if err := sm.StartService(name); err != nil {
			fmt.Printf("❌ Failed to start service: %v\n", err)
			return
		}
		fmt.Printf("✅ Service started: %s\n", name)
	},
}

var servicesStopCmd = &cobra.Command{
	Use:   "stop [name]",
	Short: "Stop a service",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		name := args[0]
		sm := services.NewServiceManager()
		if err := sm.StopService(name); err != nil {
			fmt.Printf("❌ Failed to stop service: %v\n", err)
			return
		}
		fmt.Printf("⏹️  Service stopped: %s\n", name)
	},
}

var servicesStopAllCmd = &cobra.Command{
	Use:   "stop-all",
	Short: "Stop all services",
	Run: func(cmd *cobra.Command, args []string) {
		sm := services.NewServiceManager()
		sm.StopAll()
		fmt.Println("⏹️  All services stopped")
	},
}

var phpCmd = &cobra.Command{
	Use:   "php",
	Short: "Manage PHP versions",
}

var phpListCmd = &cobra.Command{
	Use:   "list",
	Short: "List PHP versions",
	Run: func(cmd *cobra.Command, args []string) {
		pm := php.NewPHPManager()
		pm.DetectPHPVersions()
		fmt.Println(pm.FormatVersions())
	},
}

var phpSetCmd = &cobra.Command{
	Use:   "set [version]",
	Short: "Set default PHP version",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		version := args[0]
		pm := php.NewPHPManager()
		if err := pm.SetDefault(version); err != nil {
			fmt.Printf("❌ Failed to set PHP version: %v\n", err)
			return
		}
		fmt.Printf("✅ Default PHP version set to %s\n", version)
	},
}

var xdebugCmd = &cobra.Command{
	Use:   "xdebug",
	Short: "Manage XDebug",
}

var xdebugEnableCmd = &cobra.Command{
	Use:   "enable",
	Short: "Enable XDebug",
	Run: func(cmd *cobra.Command, args []string) {
		xm := xdebug.NewXDebugManager()
		xm.SetEnabled(true)
		fmt.Println("✅ XDebug enabled")
	},
}

var xdebugDisableCmd = &cobra.Command{
	Use:   "disable",
	Short: "Disable XDebug",
	Run: func(cmd *cobra.Command, args []string) {
		xm := xdebug.NewXDebugManager()
		xm.SetEnabled(false)
		fmt.Println("⏹️  XDebug disabled")
	},
}

var forgeCmd = &cobra.Command{
	Use:   "forge",
	Short: "Manage Laravel Forge integration",
}

var forgeServersCmd = &cobra.Command{
	Use:   "servers",
	Short: "List Forge servers",
	Run: func(cmd *cobra.Command, args []string) {
		apiKey := os.Getenv("FORGE_API_KEY")
		if apiKey == "" {
			fmt.Println("❌ FORGE_API_KEY environment variable not set")
			return
		}
		fc := forge.NewForgeClient(apiKey)
		servers, err := fc.GetServers()
		if err != nil {
			fmt.Printf("❌ Failed to get servers: %v\n", err)
			return
		}
		fmt.Println("Forge Servers:")
		for _, server := range servers {
			fmt.Printf("  • %s (%s) - %s\n", server.Name, server.IP, server.Status)
		}
	},
}

var forgeDeployCmd = &cobra.Command{
	Use:   "deploy [server-id] [site-id]",
	Short: "Deploy a site via Forge",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		apiKey := os.Getenv("FORGE_API_KEY")
		if apiKey == "" {
			fmt.Println("❌ FORGE_API_KEY environment variable not set")
			return
		}
		serverID := args[0]
		siteID := args[1]
		fc := forge.NewForgeClient(apiKey)
		if err := fc.DeploySite(serverID, siteID); err != nil {
			fmt.Printf("❌ Deployment failed: %v\n", err)
			return
		}
		fmt.Println("✅ Deployment started")
	},
}

var nodeCmd = &cobra.Command{
	Use:   "node",
	Short: "Manage Node.js versions",
}

var nodeListCmd = &cobra.Command{
	Use:   "list",
	Short: "List Node.js versions",
	Run: func(cmd *cobra.Command, args []string) {
		nm := node.NewNodeManager()
		fmt.Println(nm.FormatVersions())
	},
}

var nodeSetCmd = &cobra.Command{
	Use:   "set [version]",
	Short: "Set default Node.js version",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		version := args[0]
		nm := node.NewNodeManager()
		if err := nm.SetDefault(version); err != nil {
			fmt.Printf("❌ Failed to set Node version: %v\n", err)
			return
		}
		fmt.Printf("✅ Default Node version set to %s\n", version)
	},
}

var nodeInstallCmd = &cobra.Command{
	Use:   "install [version]",
	Short: "Install a Node.js version",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		version := args[0]
		nm := node.NewNodeManager()
		fmt.Printf("Installing Node.js %s...\n", version)
		if err := nm.InstallVersion(version); err != nil {
			fmt.Printf("❌ Installation failed: %v\n", err)
			return
		}
		fmt.Printf("✅ Node.js %s installed\n", version)
	},
}

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show system status",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Stacker Status")
		fmt.Println("═══════════════")
		cfg := config.Load(cfgFile)
		if len(cfg.Sites) > 0 {
			fmt.Printf("\n🌐 Sites (%d):\n", len(cfg.Sites))
			for _, site := range cfg.Sites {
				fmt.Printf("   • %s.test\n", site.Name)
			}
		}
		sm := services.NewServiceManager()
		fmt.Println(sm.FormatStatus())
		pm := php.NewPHPManager()
		pm.DetectPHPVersions()
		defaultPHP := pm.GetDefault()
		if defaultPHP != nil {
			fmt.Printf("\n🐘 PHP: %s (default)\n", defaultPHP.Version)
		}
		nm := node.NewNodeManager()
		if nm.NVMInstalled() {
			fmt.Printf("\n📦 Node.js: %s\n", nm.GetCurrentNVMVersion())
		}
	},
}

func init() {
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.stacker-app/config.yaml)")

	rootCmd.AddCommand(serveCmd)
	rootCmd.AddCommand(uiCmd)
	rootCmd.AddCommand(trayAppCmd)
	rootCmd.AddCommand(addCmd)
	rootCmd.AddCommand(listCmd)
	rootCmd.AddCommand(removeCmd)
	rootCmd.AddCommand(statusCmd)

	rootCmd.AddCommand(dumpsCmd)
	dumpsCmd.AddCommand(dumpsListCmd)
	dumpsCmd.AddCommand(dumpsClearCmd)

	rootCmd.AddCommand(mailCmd)
	mailCmd.AddCommand(mailListCmd)
	mailCmd.AddCommand(mailClearCmd)

	rootCmd.AddCommand(logsCmd)
	logsCmd.AddCommand(logsListCmd)
	logsCmd.AddCommand(logsTailCmd)
	logsCmd.AddCommand(logsSearchCmd)

	rootCmd.AddCommand(servicesCmd)
	servicesCmd.AddCommand(servicesListCmd)
	servicesCmd.AddCommand(servicesAddCmd)
	servicesCmd.AddCommand(servicesStartCmd)
	servicesCmd.AddCommand(servicesStopCmd)
	servicesCmd.AddCommand(servicesStopAllCmd)

	rootCmd.AddCommand(phpCmd)
	phpCmd.AddCommand(phpListCmd)
	phpCmd.AddCommand(phpSetCmd)

	rootCmd.AddCommand(xdebugCmd)
	xdebugCmd.AddCommand(xdebugEnableCmd)
	xdebugCmd.AddCommand(xdebugDisableCmd)

	rootCmd.AddCommand(forgeCmd)
	forgeCmd.AddCommand(forgeServersCmd)
	forgeCmd.AddCommand(forgeDeployCmd)

	rootCmd.AddCommand(nodeCmd)
	nodeCmd.AddCommand(nodeListCmd)
	nodeCmd.AddCommand(nodeSetCmd)
	nodeCmd.AddCommand(nodeInstallCmd)
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
