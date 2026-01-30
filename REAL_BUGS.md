# Stacker - Gerçek Bug ve Sorunlar

> ⚠️ Önceki analizde bazı şeyleri abartmışım. Bu doküman kanıtlanabilir gerçek sorunları içerir.

---

## 🚨 GERÇEK Sorunlar (Kanıtlı)

### 1. Command Injection - KESİN (web.go:862-890)

**Kod:**
```go
func (ws *WebServer) handleRunTerminalCommand(w http.ResponseWriter, r *http.Request) {
    var req struct {
        Command string `json:"command"`
    }
    json.NewDecoder(r.Body).Decode(&req)
    
    // MACOS
    script := fmt.Sprintf(`tell application "Terminal"
        do script "%s"
    end tell`, req.Command)
    cmd = exec.Command("osascript", "-e", script)
    
    // LINUX
    cmd = exec.Command("x-terminal-emulator", "-e", "bash", "-c", req.Command+"; exec bash")
}
```

**Kanıt:**
```bash
curl -X POST http://localhost:9999/api/run-terminal-command \
  -H "Content-Type: application/json" \
  -d '{"command": "\"; rm -rf /tmp/test; echo \"hacked"}'
```

**Sonuç:** Komut enjeksiyonu mümkün. ✅ GERÇEK TEHLİKE

---

### 2. HTTP Client Timeout YOK (ssl.go, remote.go)

**Kod:**
```go
// ssl.go:54
resp, err := http.Get(downloadURL)  // Timeout yok!

// remote.go:103
client := &http.Client{Timeout: 10 * time.Second}  // Bu var ama:
resp, err := http.Get(downloadURL)  // Burada default client kullanılıyor (timeout yok)
```

**Sonuç:** Sonsuz bekleme riski. ✅ GERÇEK SORUN

---

### 3. Path Validation EKSİK (web.go:818-836)

**Kod:**
```go
func (ws *WebServer) handleOpenFolder(w http.ResponseWriter, r *http.Request) {
    var req struct { Path string `json:"path"` }
    json.NewDecoder(r.Body).Decode(&req)
    
    cmd = exec.Command("open", req.Path)  // Validasyon YOK
}
```

**Kanıt:**
```json
{"path": "../../../etc/passwd"}
```

**Sonuç:** Sistem dosyalarına erişim denenebilir. ⚠️ Kısmen risk (macOS sandbox var)

---

### 4. Error Handling EKSİK (mail.go:116-126)

**Kod:**
```go
func (mm *MailManager) loadEmails() {
    files, _ := os.ReadDir(mm.mailDir)  // Hata ignore!
    for _, file := range files {
        data, _ := os.ReadFile(...)     // Hata ignore!
        json.Unmarshal(data, &email)    // Hata ignore!
    }
}
```

**Sonuç:** Sessizce başarısız olur, debugging zor. ✅ GERÇEK SORUN

---

### 5. JSON Malformed (mail.go:130-140)

**Kod:**
```go
data := fmt.Sprintf(`{
    "body": "%s",
}`, email.Body)
```

**Kanıt:**
```go
email.Body = `I said "hello"`
// Sonuç: {"body": "I said "hello""}  // BROKEN JSON
```

**Sonuç:** JSON parse edilemez. ✅ GERÇEK BUG

---

## ❌ ABARTTIĞIM Şeyler

| Sorun | Durum | Gerçek |
|-------|-------|--------|
| `dumps.go:127` Race Condition | ❌ YANLIŞ | Sadece deterministik olmayan ID, race değil |
| `services.go` Concurrent Map | ❌ YANLIŞ | `sm.mu.Lock()` zaten var |
| Port 100 problemi | ❌ ABARTI | PHP 10.0 henüz yok, teorik |
| "JSON Injection" | ❌ ABARTI | Sadece malformed JSON, injection değil |

---

## 🔧 GERÇEKÇİ Fix Öncelikleri

### Öncelik 1 (Acil)
1. **Command Injection** - Input sanitization
2. **HTTP Timeout** - Client timeout ekleme

### Öncelik 2 (Önemli)
3. **JSON Malformed** - `json.Marshal()` kullanma
4. **Error Handling** - Basit error loglama

### Öncelik 3 (İyileştirme)
5. **Path Validation** - Temel path check

---

## 📊 Özet

- **Kesin Bug:** 5
- **Abartılan:** 4
- **Toplam:** 9

**Gerçekçi tahmini fix süresi:** 1-2 gün (basit fix'ler)
