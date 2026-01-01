---
description: How to install and manage services (MySQL, Redis, Nginx)
---

# Services Management

## Install Service via Web UI

1. Open http://localhost:9999
2. Click "Services" in sidebar
3. Click "Open Server Configuration"
4. Select tab (Nginx, MySQL, Redis)
5. Configure port and options
6. Click "Save"

## Install Service via API

```bash
# Install MySQL
curl -X POST http://localhost:9999/api/services/install \
  -H "Content-Type: application/json" \
  -d '{"name":"mysql","port":3306}'

# Install Redis
curl -X POST http://localhost:9999/api/services/install \
  -H "Content-Type: application/json" \
  -d '{"name":"redis","port":6379}'

# Install Nginx
curl -X POST http://localhost:9999/api/services/install \
  -H "Content-Type: application/json" \
  -d '{"name":"nginx","port":80}'
```

## Service Directories

All services are stored self-contained:
- Binaries: `~/Library/Application Support/Stacker/bin/{service}/`
- Data: `~/Library/Application Support/Stacker/data/{service}/`
- Config: `~/Library/Application Support/Stacker/conf/{service}/`
- Logs: `~/Library/Application Support/Stacker/logs/`

## Default Ports

| Service     | Port  |
|-------------|-------|
| Nginx HTTP  | 80    |
| Nginx HTTPS | 443   |
| MySQL       | 3306  |
| Redis       | 6379  |
| Meilisearch | 7700  |