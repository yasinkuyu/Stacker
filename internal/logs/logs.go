package logs

import (
	"bufio"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

type LogEntry struct {
	Timestamp time.Time `json:"timestamp"`
	Level     string    `json:"level"`
	Message   string    `json:"message"`
	Context   string    `json:"context"`
	File      string    `json:"file"`
	Line      int       `json:"line"`
	FullText  string    `json:"full_text"`
}

type LogFile struct {
	Name     string    `json:"name"`
	Path     string    `json:"path"`
	Size     int64     `json:"size"`
	Modified time.Time `json:"modified"`
	Site     string    `json:"site"`
}

type LogManager struct {
	logDirs map[string]string
	mu      sync.RWMutex
	cache   map[string][]LogEntry
}

func NewLogManager() *LogManager {
	return &LogManager{
		logDirs: make(map[string]string),
		cache:   make(map[string][]LogEntry),
	}
}

func (lm *LogManager) AddLogDir(site, path string) {
	lm.mu.Lock()
	defer lm.mu.Unlock()
	lm.logDirs[site] = path
}

func (lm *LogManager) GetLogFiles() []LogFile {
	lm.mu.RLock()
	defer lm.mu.RUnlock()

	var files []LogFile

	for site, dir := range lm.logDirs {
		filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return nil
			}

			if !d.IsDir() && strings.HasSuffix(d.Name(), ".log") {
				info, _ := d.Info()
				files = append(files, LogFile{
					Name:     d.Name(),
					Path:     path,
					Size:     info.Size(),
					Modified: info.ModTime(),
					Site:     site,
				})
			}
			return nil
		})
	}

	sort.Slice(files, func(i, j int) bool {
		return files[i].Modified.After(files[j].Modified)
	})

	return files
}

func (lm *LogManager) GetLogs(logPath string, limit int) []LogEntry {
	if cached, ok := lm.cache[logPath]; ok {
		if limit > 0 && len(cached) > limit {
			return cached[len(cached)-limit:]
		}
		return cached
	}

	lm.parseLogFile(logPath)
	return lm.GetLogs(logPath, limit)
}

func (lm *LogManager) GetLogsBySite(site string) []LogEntry {
	lm.mu.RLock()
	defer lm.mu.RUnlock()

	var allLogs []LogEntry
	for path, logs := range lm.cache {
		if strings.Contains(path, site) {
			allLogs = append(allLogs, logs...)
		}
	}
	return allLogs
}

func (lm *LogManager) SearchLogs(query string) []LogEntry {
	var results []LogEntry

	lm.mu.RLock()
	defer lm.mu.RUnlock()

	for _, logs := range lm.cache {
		for _, entry := range logs {
			if strings.Contains(strings.ToLower(entry.Message), strings.ToLower(query)) ||
				strings.Contains(strings.ToLower(entry.FullText), strings.ToLower(query)) {
				results = append(results, entry)
			}
		}
	}

	return results
}

func (lm *LogManager) SearchLogsByRegex(pattern string) []LogEntry {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return []LogEntry{}
	}

	var results []LogEntry

	lm.mu.RLock()
	defer lm.mu.RUnlock()

	for _, logs := range lm.cache {
		for _, entry := range logs {
			if re.MatchString(entry.Message) || re.MatchString(entry.FullText) {
				results = append(results, entry)
			}
		}
	}

	return results
}

func (lm *LogManager) TailLog(logPath string, callback func(LogEntry)) error {
	file, err := os.Open(logPath)
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		entry := lm.parseLine(scanner.Text())
		if entry.Timestamp.IsZero() {
			continue
		}
		callback(entry)
	}

	return scanner.Err()
}

func (lm *LogManager) ClearCache() {
	lm.mu.Lock()
	defer lm.mu.Unlock()
	lm.cache = make(map[string][]LogEntry)
}

func (lm *LogManager) parseLogFile(logPath string) []LogEntry {
	file, err := os.Open(logPath)
	if err != nil {
		return []LogEntry{}
	}
	defer file.Close()

	var entries []LogEntry
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		entry := lm.parseLine(scanner.Text())
		if !entry.Timestamp.IsZero() {
			entries = append(entries, entry)
		}
	}

	lm.mu.Lock()
	lm.cache[logPath] = entries
	lm.mu.Unlock()

	return entries
}

func (lm *LogManager) parseLine(line string) LogEntry {
	// Laravel log format: [2024-01-01 12:00:00] local.ERROR: message
	re := regexp.MustCompile(`\[(\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2})\] (\w+)\.(\w+): (.+)`)
	matches := re.FindStringSubmatch(line)

	if len(matches) == 5 {
		timestamp, _ := time.Parse("2006-01-02 15:04:05", matches[1])
		return LogEntry{
			Timestamp: timestamp,
			Level:     matches[3],
			Message:   matches[4],
			FullText:  line,
		}
	}

	// Generic log format
	re = regexp.MustCompile(`(\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2})`)
	if re.MatchString(line) {
		return LogEntry{
			Timestamp: time.Now(),
			Message:   line,
			FullText:  line,
		}
	}

	return LogEntry{}
}

func (lm *LogManager) FormatLogs(entries []LogEntry) string {
	var buf strings.Builder

	for _, entry := range entries {
		levelIcon := getLevelIcon(entry.Level)
		buf.WriteString(fmt.Sprintf("%s [%s] %s\n", levelIcon, entry.Timestamp.Format("15:04:05"), entry.Level))
		buf.WriteString(fmt.Sprintf("   %s\n\n", entry.Message))
	}

	return buf.String()
}

func getLevelIcon(level string) string {
	switch strings.ToUpper(level) {
	case "EMERGENCY":
		return "🚨"
	case "ALERT":
		return "⚠️"
	case "CRITICAL":
		return "💥"
	case "ERROR":
		return "❌"
	case "WARNING":
		return "⚡"
	case "NOTICE":
		return "📌"
	case "INFO":
		return "ℹ️"
	case "DEBUG":
		return "🔍"
	default:
		return "📝"
	}
}
