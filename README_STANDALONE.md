# Stacker - Tam Bağımsız Local Geliştirme Ortamı

Stacker, PHP geliştirme için tam bağımsız, dış bağımlılıkları olmayan bir local sunucu yöneticisidir.

## Özellikler

### 🗄️ Servis Yönetimi
Tüm servisler tam bağımsız çalışır ve Stacker'ın data klasöründe yönetilir:

- **MariaDB** (11.2, 10.11, 10.6)
- **MySQL** (8.0, 5.7)
- **Nginx** (1.25, 1.24)
- **Apache** (2.4)
- **Redis** (7.2, 7.0)

### 📦 PHP Yönetimi
- Çoklu PHP versiyon desteği (8.3, 8.2, 8.1, 8.0, 7.4)
- Otomatik kurulum ve yapılandırma
- XDebug entegrasyonu

### 🌐 Site Yönetimi
- Site ekleme, silme, düzenleme
- Otomatik SSL sertifikası
- Hosts dosyası yönetimi
- Custom domain desteği

### 🔧 Araçlar
- Laravel Dumps viewer
- Email catcher
- Log viewer ve arama
- Node.js versiyon yönetimi

## Kurulum

Programı çalıştırmak için **sistem bağımlılığı gerekmez**. Tüm servisler programın data klasörüne indirilir:

```bash
# Programı çalıştır
./stacker ui
```

## Kullanım

### CLI Komutları

```bash
# Servisleri listele
stacker services list

# Mevcut versiyonları göster
stacker services versions

# Servis kurulumu (tam bağımsız)
stacker services install mariadb 11.2
stacker services install nginx 1.25
stacker services install redis 7.2

# Servisleri yönet
stacker services start mariadb-11.2
stacker services stop mariadb-11.2
stacker services restart mariadb-11.2

# Servisi kaldır
stacker services uninstall mariadb-11.2

# Tüm servisleri başlat/durdur
stacker services start-all
stacker services stop-all

# Site ekle
stacker add myproject /path/to/project

# PHP versiyonları
stacker php list
stacker php install 8.3
stacker php set 8.3

# System durumu
stacker status
```

### Web UI

```bash
# Web UI başlat
./stacker ui

# Tray uygulaması olarak başlat
./stacker tray
```

## Veri Dizini

Tüm servisler ve veriler tamamen bağımsız çalışır:

```
~/Library/Application Support/Stacker/
├── bin/              # Servis binary dosyaları
│   ├── mariadb/
│   ├── nginx/
│   ├── apache/
│   └── redis/
├── conf/             # Konfigürasyon dosyaları
│   ├── mariadb/
│   ├── nginx/
│   ├── apache/
│   └── redis/
├── data/             # Veri dosyaları
│   ├── mariadb/
│   ├── nginx/
│   ├── apache/
│   └── redis/
├── logs/             # Log dosyaları
├── pids/             # PID dosyaları
├── sites.json        # Site konfigürasyonu
└── services.json     # Servis durumları
```

## API Endpoint'leri

### Servisler
- `GET /api/services` - Tüm servisleri listele
- `GET /api/services/versions?type=mariadb` - Mevcut versiyonlar
- `POST /api/services/install` - Servis kur
- `POST /api/services/uninstall` - Servis kaldır
- `POST /api/services/start/{name}` - Servis başlat
- `POST /api/services/stop/{name}` - Servis durdur
- `POST /api/services/restart/{name}` - Servis yeniden başlat

### PHP
- `GET /api/php` - PHP versiyonları
- `POST /api/php/install` - PHP kur
- `PUT /api/php/default` - Default PHP ayarla
- `GET /api/php/install-status?version=8.3` - Kurulum durumu

### Siteler
- `GET /api/sites` - Siteleri listele
- `POST /api/sites` - Site ekle
- `PUT /api/sites/{name}` - Site güncelle
- `DELETE /api/sites/{name}` - Site sil

## Avantajlar

✅ **Tam Bağımsız** - Sistem bağımlılığı yok (Homebrew, apt, vs gerekmez)
✅ **Çoklu Versiyon** - Her servisten birden fazla versiyon çalıştırabilir
✅ **Hafif** - Yalnızca ihtiyaç duyulan servisleri kurun
✅ **İzole** - Her servis kendi klasöründe çalışır, çakışma yok
✅ **Kolay** - Tek komutla kur, yönet
✅ **Portable** - Data klasörü taşınabilir

## Geliştirme

```bash
# Derle
go build -o stacker main.go

# Çalıştır
./stacker ui

# Tray uygulaması
./stacker tray
```

## Lisans

MIT
