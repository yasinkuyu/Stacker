# Stacker Geliştirme Planı

> Bu doküman, Stacker projesinde tespit edilen bug'ların, güvenlik açıklarının ve iyileştirmelerin düzeltilmesi için hazırlanmıştır.
> 
> 📅 Oluşturulma: 2026-01-30
> 🎯 Hedef: Stabil ve güvenli bir local development ortamı

---

## 📋 İçindekiler

1. [Kritik Sorunlar (P0)](#kritik-sorunlar-p0)
2. [Yüksek Öncelikli Sorunlar (P1)](#yüksek-öncelikli-sorunlar-p1)
3. [Orta Öncelikli İyileştirmeler (P2)](#orta-öncelikli-iyileştirmeler-p2)
4. [Düşük Öncelikli İyileştirmeler (P3)](#düşük-öncelikli-iyileştirmeler-p3)
5. [Test Stratejisi](#test-stratejisi)
6. [Sprint Planlaması](#sprint-planlaması)

---

## 🚨 Kritik Sorunlar (P0)

> **Bu sorunlar çözülmeden production kullanımı önerilmez!**

### P0-1: Race Condition - ID Üretimi
**Dosya:** `internal/dumps/dumps.go:127`

```go
// ❌ SORUNLU KOD
func generateID() string {
    b := make([]byte, 8)
    for i := range b {
        b[i] = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"[time.Now().UnixNano()%62]
    }
    return string(b)
}
```

**Sorun:** 
- `time.Now().UnixNano()` her iterasyonda farklı değer dönebilir
- Concurrent erişimde race condition riski
- Tutarsız ID üretimi

**Çözüm:**
```go
// ✅ DÜZELTİLMİŞ KOD
func generateID() string {
    const chars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
    b := make([]byte, 8)
    // crypto/rand kullanarak güvenli ID üretimi
    if _, err := rand.Read(b); err != nil {
        // Fallback: timestamp + counter
        return fmt.Sprintf("%d-%d", time.Now().UnixNano(), atomic.AddInt64(&counter, 1))
    }
    for i := range b {
        b[i] = chars[int(b[i])%len(chars)]
    }
    return string(b)
}
```

**Tahmini Süre:** 2 saat
**Test:** Race detector ile test

---

### P0-2: Command Injection Riski
**Dosya:** `internal/web/web.go:872-878`

**Sorun:** Kullanıcı girdisi doğrudan shell komutuna gömülüyor

**Çözüm:**
- Input sanitization
- Whitelist-based validation
- Prepared command pattern

**Tahmini Süre:** 3 saat
**Test:** Security fuzzing testleri

---

### P0-3: Path Traversal Güvenlik Açığı
**Dosya:** `internal/web/web.go:818-836`

**Sorun:** Kullanıcı path'i validasyon yapılmadan kullanılıyor

**Çözüm:**
```go
func sanitizePath(baseDir, userPath string) (string, error) {
    // Path traversal kontrolü
    cleanPath := filepath.Clean(userPath)
    if strings.Contains(cleanPath, "..") {
        return "", fmt.Errorf("invalid path")
    }
    fullPath := filepath.Join(baseDir, cleanPath)
    // Ensure path is within base directory
    if !strings.HasPrefix(filepath.Clean(fullPath), filepath.Clean(baseDir)) {
        return "", fmt.Errorf("path outside allowed directory")
    }
    return fullPath, nil
}
```

**Tahmini Süre:** 2 saat
**Test:** Path traversal test cases

---

### P0-4: JSON Injection Riski
**Dosya:** `internal/mail/mail.go:129-142`

**Sorun:** Manuel JSON string oluşturma

**Çözüm:** `json.Marshal()` kullanımına geçiş

**Tahmini Süre:** 1 saat

---

### P0-5: Concurrent Map Access Panic Riski
**Dosya:** `internal/services/services.go`

**Sorun:** `loadInstalledServices()` fonksiyonunda map'e concurrent write

**Çözüm:** Mutex lock scope'unu genişlet

**Tahmini Süre:** 2 saat
**Test:** Concurrent access testleri

---

## 🔶 Yüksek Öncelikli Sorunlar (P1)

### P1-1: Port Çakışması Mantığı
**Dosya:** `internal/php/fpm.go:59-69`

**Sorun:** PHP 10.0 için port 100 atanacak (privileged), PHP 7.4 ve 74.x çakışacak

**Çözüm:**
```go
func GetPort(version string) int {
    // Semver parsing
    parts := strings.Split(version, ".")
    if len(parts) >= 2 {
        major := parts[0]
        minor := parts[1]
        // 9000 + major*100 + minor
        // PHP 7.4 -> 9074
        // PHP 8.3 -> 9083
        // PHP 10.0 -> 9100 (hala > 1024)
    }
    return 9000
}
```

**Tahmini Süre:** 3 saat

---

### P1-2: Resource Leak - Log Dosyası
**Dosya:** `internal/php/fpm.go:99-104`

**Sorun:** Başarılı başlatmada log dosyası handle'ı kapanmıyor

**Çözüm:** `cmd.Wait()` sonrası close veya process monitoring

**Tahmini Süre:** 2 saat

---

### P1-3: Insecure HTTP Download
**Dosya:** `internal/ssl/ssl.go:54`

**Sorun:** Mkcert binary indirme HTTP üzerinden yapılabilir

**Çözüm:** HTTPS-only download, checksum verification

**Tahmini Süre:** 2 saat

---

### P1-4: Hardcoded Credentials
**Dosya:** `internal/services/services.go:1035`

**Sorun:** Sabit root şifresi

**Çözüm:** Rastgele şifre üretimi, kullanıcıdan isteme opsiyonu

**Tahmini Süre:** 3 saat

---

### P1-5: Unbounded Memory Cache
**Dosya:** `internal/logs/logs.go:36-38`

**Sorun:** Cache boyutu sınırsız

**Çözüm:** LRU cache implementasyonu

**Tahmini Süre:** 4 saat

---

## 🔸 Orta Öncelikli İyileştirmeler (P2)

### P2-1: Error Handling Eksiklikleri
**Dosyalar:** Birden fazla dosyada

**Sorun:** `err` değişkenleri ignore edilmiş

**Çözüm:** 
- `errcheck` tool kullanımı
- Structured error handling

**Tahmini Süre:** 6 saat (tüm proje için)

---

### P2-2: Goroutine Leak
**Dosya:** `internal/tray/tray_cgo.go:251`

**Sorun:** Ticker stop edilmiyor

**Çözüm:** `defer ticker.Stop()`

**Tahmini Süre:** 30 dk

---

### P2-3: Memory Leak - WebSocket
**Dosya:** `internal/dumps/dumps.go:100-107`

**Sorun:** Buffer dolunca block, goroutine leak

**Çözüm:** Non-blocking send veya timeout

**Tahmini Süre:** 3 saat

---

### P2-4: Hosts File Race Condition
**Dosya:** `internal/utils/hosts.go`

**Sorun:** Singleton pattern eksik

**Çözüm:** Single instance with file locking

**Tahmini Süre:** 4 saat

---

### P2-5: Magic Numbers
**Dosya:** `internal/config/preferences.go:66-70`

**Çözüm:** Constants tanımlama

**Tahmini Süre:** 1 saat

---

## 🔹 Düşük Öncelikli İyileştirmeler (P3)

### P3-1: String Concatenation → filepath.Join
**Dosya:** `internal/services/services.go`

**Tahmini Süre:** 2 saat

---

### P3-2: Context Usage
**Sorun:** HTTP handler'larda context timeout yok

**Çözüm:** `context.WithTimeout()` implementasyonu

**Tahmini Süre:** 4 saat

---

### P3-3: Structured Logging
**Sorun:** `fmt.Printf` kullanımı

**Çözüm:** `log/slog` veya `zap` entegrasyonu

**Tahmini Süre:** 6 saat

---

### P3-4: Configuration Validation
**Sorun:** Config dosyası validasyonu yetersiz

**Çözüm:** JSON Schema validation

**Tahmini Süre:** 4 saat

---

## 🧪 Test Stratejisi

### Birim Testleri
```
coverage hedefi: >80%
- internal/config/*
- internal/utils/*
- internal/php/*
```

### Entegrasyon Testleri
- Servis kurulum/başlatma/durdurma akışı
- Web UI API testleri
- Hosts file yönetimi

### Güvenlik Testleri
```bash
# Gosec ile güvenlik taraması
gosec -fmt sarif -out security.sarif ./...

# Fuzzing testleri
go test -fuzz=FuzzParseVersion

# Race condition testleri
go test -race ./...
```

### Load Testleri
- 100+ eşzamanlı bağlantı
- Bellek kullanımı monitoring
- Goroutine leak tespiti

---

## 📅 Sprint Planlaması

### Sprint 1: Kritik Güvenlik ve Stabilite (Hafta 1-2)
**Hedef:** Production-ready stabilite

| Gün | Görev | Tahmini Süre |
|-----|-------|--------------|
| 1 | P0-1: Race Condition fix | 2s |
| 1 | P0-2: Command Injection fix | 3s |
| 2 | P0-3: Path Traversal fix | 2s |
| 2 | P0-4: JSON Injection fix | 1s |
| 3 | P0-5: Concurrent map fix | 2s |
| 3 | Test yazımı | 4s |
| 4-5 | Review ve bugfix | 8s |

**Toplam:** 22 saat (~3 gün)

---

### Sprint 2: Servis Yönetimi İyileştirmeleri (Hafta 3)
**Hedef:** Güvenilir servis yönetimi

| Görev | Tahmini Süre |
|-------|--------------|
| P1-1: Port çakışması fix | 3s |
| P1-2: Resource leak fix | 2s |
| P1-4: Hardcoded credentials | 3s |
| Servis health check | 4s |
| Retry mekanizması | 4s |

**Toplam:** 16 saat (~2 gün)

---

### Sprint 3: Performans ve Memory (Hafta 4)
**Hedef:** Ölçeklenebilirlik

| Görev | Tahmini Süre |
|-------|--------------|
| P1-5: Bounded cache | 4s |
| P2-3: WebSocket leak | 3s |
| P2-2: Goroutine leak | 0.5s |
| Memory profiling | 4s |
| Optimizasyon | 4s |

**Toplam:** 15.5 saat (~2 gün)

---

### Sprint 4: Kod Kalitesi ve Altyapı (Hafta 5-6)
**Hedef:** Maintainable codebase

| Görev | Tahmini Süre |
|-------|--------------|
| P2-1: Error handling | 6s |
| P3-3: Structured logging | 6s |
| P3-2: Context usage | 4s |
| Linting kuralları | 2s |
| CI/CD pipeline | 4s |

**Toplam:** 22 saat (~3 gün)

---

## 📊 Özet Zaman Planı

| Sprint | Süre | Başlangıç | Bitiş |
|--------|------|-----------|-------|
| Sprint 1 | 3 gün | Day 1 | Day 3 |
| Sprint 2 | 2 gün | Day 4 | Day 5 |
| Sprint 3 | 2 gün | Day 6 | Day 7 |
| Sprint 4 | 3 gün | Day 8 | Day 10 |
| Buffer | 2 gün | Day 11 | Day 12 |

**Toplam:** ~12 iş günü (2-3 hafta)

---

## ✅ Her Sprint Sonrası Yapılacaklar

1. **Code Review**
   - En az 1 reviewer onayı
   - Güvenlik review'ı

2. **Testler**
   - Tüm testler geçmeli
   - Coverage %80+ olmalı
   - Race detector temiz olmalı

3. **Dokümantasyon**
   - CHANGELOG güncellemesi
   - API dokümantasyonu (varsa)

4. **Release Notes**
   - Düzeltilen bug'lar
   - Yeni özellikler
   - Breaking changes

---

## 🛠️ Geliştirme Ortamı Kurulumu

```bash
# Gerekli araçlar
make install-tools

# Pre-commit hooks
./scripts/setup-dev.sh

# Docker ile development
docker-compose -f docker-compose.dev.yml up

# Local development
make dev  # Air hot reload
```

---

## 📈 Başarı Kriterleri

### Sprint 1 Sonrası
- [ ] Race condition tespit edilmiyor (`go test -race`)
- [ ] Gosec high/critical issue kalmadı
- [ ] Path traversal testleri geçiyor

### Sprint 2 Sonrası
- [ ] Port çakışması testleri geçiyor
- [ ] Memory leak tespit edilmiyor
- [ ] Servis restart testleri stabil

### Sprint 3 Sonrası
- [ ] 1000+ site ile test edilebiliyor
- [ ] Memory kullanımı sabit (growth yok)
- [ ] Response time < 100ms

### Sprint 4 Sonrası
- [ ] Linting hatası yok
- [ ] Test coverage > 80%
- [ ] CI/CD pipeline çalışıyor

---

## 📝 Notlar

- Her değişiklik için ayrı branch: `fix/P0-1-race-condition`
- Commit mesaj formatı: `fix(P0-1): resolve race condition in ID generation`
- Her P0 fix'i için regression test yazılmalı
- Performance değişiklikleri için benchmark eklenebilir

---

**Hazırlayan:** Kimi Code CLI  
**Son Güncelleme:** 2026-01-30
