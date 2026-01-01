package utils

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

var logMu sync.Mutex

// Log writes a message to the stacker.log file
func Log(level, message string) {
	logMu.Lock()
	defer logMu.Unlock()

	logsDir := filepath.Join(GetStackerDir(), "logs")
	os.MkdirAll(logsDir, 0755)

	logFile := filepath.Join(logsDir, "stacker.log")

	f, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Printf("Failed to open log file: %v\n", err)
		return
	}
	defer f.Close()

	timestamp := time.Now().Format("2006-01-02 15:04:05")
	logLine := fmt.Sprintf("[%s] [%s] %s\n", timestamp, level, message)

	f.WriteString(logLine)

	// Also print to console
	fmt.Print(logLine)
}

// LogInfo logs an info message
func LogInfo(message string) {
	Log("INFO", message)
}

// LogError logs an error message
func LogError(message string) {
	Log("ERROR", message)
}

// LogWarn logs a warning message
func LogWarn(message string) {
	Log("WARN", message)
}

// LogService logs a service-related message
func LogService(serviceName, action, status string) {
	Log("SERVICE", fmt.Sprintf("%s: %s - %s", serviceName, action, status))
}
