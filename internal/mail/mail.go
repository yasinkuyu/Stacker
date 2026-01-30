package mail

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/yasinkuyu/Stacker/internal/config"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Email struct {
	ID        string    `json:"id"`
	Site      string    `json:"site"`
	From      string    `json:"from"`
	To        []string  `json:"to"`
	Subject   string    `json:"subject"`
	Body      string    `json:"body"`
	HTML      string    `json:"html"`
	Timestamp time.Time `json:"timestamp"`
	Read      bool      `json:"read"`
}

type MailManager struct {
	cfg     *config.Config
	emails  []Email
	mailDir string
	port    int
	server  *MailServer
}

type MailServer struct {
	smtpPort int
	pop3Port int
}

func NewMailManager(cfg *config.Config) *MailManager {
	home, _ := os.UserHomeDir()
	mailDir := filepath.Join(home, ".stacker-app", "mail")
	os.MkdirAll(mailDir, 0755)

	mm := &MailManager{
		cfg:     cfg,
		mailDir: mailDir,
		port:    1025,
		server:  &MailServer{smtpPort: 1025, pop3Port: 1100},
	}

	mm.loadEmails()
	return mm
}

func (mm *MailManager) LoadEmails() []Email {
	return mm.emails
}

func (mm *MailManager) GetEmailsBySite(site string) []Email {
	var result []Email
	for _, email := range mm.emails {
		if email.Site == site {
			result = append(result, email)
		}
	}
	return result
}

func (mm *MailManager) GetEmail(id string) *Email {
	for i := range mm.emails {
		if mm.emails[i].ID == id {
			return &mm.emails[i]
		}
	}
	return nil
}

func (mm *MailManager) AddEmail(email Email) error {
	email.ID = generateID()
	email.Timestamp = time.Now()
	email.Read = false

	mm.emails = append(mm.emails, email)
	if err := mm.saveEmail(email); err != nil {
		return fmt.Errorf("failed to save email: %w", err)
	}

	return nil
}

func (mm *MailManager) DeleteEmail(id string) {
	for i, email := range mm.emails {
		if email.ID == id {
			mm.emails = append(mm.emails[:i], mm.emails[i+1:]...)
			os.Remove(filepath.Join(mm.mailDir, id+".json"))
			return
		}
	}
}

func (mm *MailManager) MarkAsRead(id string) {
	for i := range mm.emails {
		if mm.emails[i].ID == id {
			mm.emails[i].Read = true
			if err := mm.saveEmail(mm.emails[i]); err != nil {
				// Log error but don't fail - email is marked as read in memory
				fmt.Printf("Warning: failed to save email %s: %v\n", id, err)
			}
			return
		}
	}
}

func (mm *MailManager) ClearEmails() {
	for _, email := range mm.emails {
		os.Remove(filepath.Join(mm.mailDir, email.ID+".json"))
	}
	mm.emails = []Email{}
}

func (mm *MailManager) loadEmails() {
	files, err := os.ReadDir(mm.mailDir)
	if err != nil {
		// Directory might not exist yet, which is fine
		return
	}
	
	for _, file := range files {
		if strings.HasSuffix(file.Name(), ".json") {
			data, err := os.ReadFile(filepath.Join(mm.mailDir, file.Name()))
			if err != nil {
				continue // Skip files that can't be read
			}
			var email Email
			if err := json.Unmarshal(data, &email); err != nil {
				continue // Skip malformed JSON files
			}
			mm.emails = append(mm.emails, email)
		}
	}
}

func (mm *MailManager) saveEmail(email Email) error {
	// Use json.Marshal for safe JSON serialization
	data, err := json.MarshalIndent(email, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal email: %w", err)
	}

	if err := os.WriteFile(filepath.Join(mm.mailDir, email.ID+".json"), data, 0644); err != nil {
		return fmt.Errorf("failed to write email file: %w", err)
	}
	return nil
}

func generateID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

func (mm *MailManager) GetEmailCount() int {
	return len(mm.emails)
}

func (mm *MailManager) GetUnreadCount() int {
	count := 0
	for _, email := range mm.emails {
		if !email.Read {
			count++
		}
	}
	return count
}

func (mm *MailManager) FormatEmailList() string {
	var buf bytes.Buffer

	for _, email := range mm.emails {
		status := "📬"
		if email.Read {
			status = "📭"
		}
		buf.WriteString(fmt.Sprintf("%s [%s] %s\n", status, email.Site, email.Subject))
		buf.WriteString(fmt.Sprintf("   From: %s | %s\n\n", email.From, email.Timestamp.Format("15:04:05")))
	}

	return buf.String()
}
