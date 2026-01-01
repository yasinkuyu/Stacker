package mail

import (
	"bufio"
	"fmt"
	"net"
	"strings"
)

func (mm *MailManager) Start() {
	go mm.startSMTP()
}

func (mm *MailManager) startSMTP() {
	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", mm.server.smtpPort))
	if err != nil {
		fmt.Printf("Failed to start SMTP server: %v\n", err)
		return
	}
	defer ln.Close()

	fmt.Printf("📧 SMTP Server listening on port %d\n", mm.server.smtpPort)

	for {
		conn, err := ln.Accept()
		if err != nil {
			continue
		}
		go mm.handleSMTPConnection(conn)
	}
}

func (mm *MailManager) handleSMTPConnection(conn net.Conn) {
	defer conn.Close()
	reader := bufio.NewReader(conn)
	writer := bufio.NewWriter(conn)

	// Greet
	writer.WriteString("220 localhost Stacker SMTP\r\n")
	writer.Flush()

	var from string
	var to []string
	var dataMode bool
	var bodyBuilder strings.Builder

	for {
		if dataMode {
			line, err := reader.ReadString('\n')
			if err != nil {
				return
			}

			// Handle end of data "."
			if strings.TrimSpace(line) == "." {
				dataMode = false
				writer.WriteString("250 OK\r\n")
				writer.Flush()

				// Process email
				mm.processEmail(from, to, bodyBuilder.String())

				// Reset for next message in same session
				from = ""
				to = []string{}
				bodyBuilder.Reset()
				continue
			}
			bodyBuilder.WriteString(line)
			continue
		}

		line, err := reader.ReadString('\n')
		if err != nil {
			return
		}
		line = strings.TrimSpace(line)
		cmd := strings.ToUpper(line)

		if strings.HasPrefix(cmd, "HELO") || strings.HasPrefix(cmd, "EHLO") {
			writer.WriteString("250 Hello\r\n")
		} else if strings.HasPrefix(cmd, "MAIL FROM:") {
			from = strings.Trim(strings.TrimPrefix(cmd, "MAIL FROM:"), "<> ")
			writer.WriteString("250 OK\r\n")
		} else if strings.HasPrefix(cmd, "RCPT TO:") {
			rcpt := strings.Trim(strings.TrimPrefix(cmd, "RCPT TO:"), "<> ")
			to = append(to, rcpt)
			writer.WriteString("250 OK\r\n")
		} else if cmd == "DATA" {
			dataMode = true
			writer.WriteString("354 End data with <CR><LF>.<CR><LF>\r\n")
		} else if cmd == "QUIT" {
			writer.WriteString("221 Bye\r\n")
			writer.Flush()
			return
		} else {
			// Ignore other commands or errors for now
			writer.WriteString("500 Command not recognized\r\n")
		}
		writer.Flush()
	}
}

func (mm *MailManager) processEmail(from string, to []string, rawBody string) {
	// Simple parsing
	subject := "No Subject"
	lines := strings.Split(rawBody, "\n")
	for _, l := range lines {
		if strings.HasPrefix(strings.ToUpper(l), "SUBJECT: ") {
			subject = strings.TrimSpace(l[9:])
			break
		}
	}

	email := Email{
		Site:    "Local", // Could potentially map IP to site
		From:    from,
		To:      to,
		Subject: subject,
		Body:    rawBody,
		HTML:    rawBody, // For now, treat raw as HTML/Text
	}
	mm.AddEmail(email)
}
