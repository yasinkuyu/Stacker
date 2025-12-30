package dumps

import (
	"encoding/json"
	"fmt"
	"github.com/yasinkuyu/Stacker/internal/config"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type Dump struct {
	ID        string      `json:"id"`
	Site      string      `json:"site"`
	Type      string      `json:"type"` // dump, query, job, view, request, log
	File      string      `json:"file"`
	Line      int         `json:"line"`
	Timestamp time.Time   `json:"timestamp"`
	Data      interface{} `json:"data"`
}

type DumpManager struct {
	cfg       *config.Config
	dumps     []Dump
	mu        sync.RWMutex
	dumpDir   string
	wsClients map[chan Dump]bool
	wsMu      sync.Mutex
}

func NewDumpManager(cfg *config.Config) *DumpManager {
	home, _ := os.UserHomeDir()
	dumpDir := filepath.Join(home, ".stacker-app", "dumps")
	os.MkdirAll(dumpDir, 0755)

	return &DumpManager{
		cfg:       cfg,
		dumpDir:   dumpDir,
		wsClients: make(map[chan Dump]bool),
	}
}

func (dm *DumpManager) AddDump(dump Dump) {
	dump.ID = generateID()
	dump.Timestamp = time.Now()

	dm.mu.Lock()
	dm.dumps = append(dm.dumps, dump)
	dm.mu.Unlock()

	dm.saveDump(dump)
	dm.broadcastDump(dump)

	fmt.Printf("📦 [%s] %s:%d - %v\n", dump.Type, dump.File, dump.Line, dump.Data)
}

func (dm *DumpManager) GetDumps() []Dump {
	dm.mu.RLock()
	defer dm.mu.RUnlock()
	return dm.dumps
}

func (dm *DumpManager) GetDumpsByType(dumpType string) []Dump {
	dm.mu.RLock()
	defer dm.mu.RUnlock()

	var result []Dump
	for _, dump := range dm.dumps {
		if dump.Type == dumpType {
			result = append(result, dump)
		}
	}
	return result
}

func (dm *DumpManager) ClearDumps() {
	dm.mu.Lock()
	dm.dumps = []Dump{}
	dm.mu.Unlock()

	os.RemoveAll(dm.dumpDir)
	os.MkdirAll(dm.dumpDir, 0755)
}

func (dm *DumpManager) saveDump(dump Dump) {
	data, err := json.MarshalIndent(dump, "", "  ")
	if err != nil {
		return
	}

	filename := filepath.Join(dm.dumpDir, fmt.Sprintf("%s.json", dump.ID))
	os.WriteFile(filename, data, 0644)
}

func (dm *DumpManager) broadcastDump(dump Dump) {
	dm.wsMu.Lock()
	defer dm.wsMu.Unlock()

	for client := range dm.wsClients {
		client <- dump
	}
}

func (dm *DumpManager) Subscribe() chan Dump {
	ch := make(chan Dump, 100)
	dm.wsMu.Lock()
	dm.wsClients[ch] = true
	dm.wsMu.Unlock()
	return ch
}

func (dm *DumpManager) Unsubscribe(ch chan Dump) {
	dm.wsMu.Lock()
	defer dm.wsMu.Unlock()
	delete(dm.wsClients, ch)
	close(ch)
}

func generateID() string {
	b := make([]byte, 8)
	for i := range b {
		b[i] = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"[time.Now().UnixNano()%62]
	}
	return string(b)
}

func (dm *DumpManager) ParseLaravelDump(data string, siteName string) {
	var dumpData interface{}

	if err := json.Unmarshal([]byte(data), &dumpData); err != nil {
		dumpData = data
	}

	dump := Dump{
		Type: "dump",
		Site: siteName,
		Data: dumpData,
	}

	dm.AddDump(dump)
}

func (dm *DumpManager) HandleLaravelDumpRequest(body []byte, siteName string) error {
	var dump struct {
		Data    interface{} `json:"dump"`
		Context struct {
			File string `json:"file"`
			Line int    `json:"line"`
		} `json:"context"`
		Type string `json:"type"`
	}

	if err := json.Unmarshal(body, &dump); err != nil {
		return err
	}

	dm.AddDump(Dump{
		Type: "dump",
		Site: siteName,
		File: dump.Context.File,
		Line: dump.Context.Line,
		Data: dump.Data,
	})

	return nil
}
