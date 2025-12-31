---
description: How to add and manage local development sites
---

# Site Management

## Add a New Site via Web UI

1. Open http://localhost:8080
2. Click "Sites" in sidebar
3. Click "Add Site" button
4. Fill in:
   - **Site Name**: project name (will be accessible at `name.test`)
   - **Project Path**: full path to project root
   - **PHP Version**: optional, uses default if empty
   - **SSL**: enable for HTTPS

## Add Site via API

```bash
curl -X POST http://localhost:8080/api/sites \
  -H "Content-Type: application/json" \
  -d '{"name":"my-project","path":"/path/to/project","ssl":true}'
```

## Update Site

```bash
curl -X PUT http://localhost:8080/api/sites/my-project \
  -H "Content-Type: application/json" \
  -d '{"name":"my-project","path":"/new/path","php":"8.2","ssl":true}'
```

## Delete Site

```bash
curl -X DELETE http://localhost:8080/api/sites/my-project
```

## Site Configuration Files

Sites are stored in:
- Config: `~/Library/Application Support/Stacker/sites.json`
- Nginx: `~/Library/Application Support/Stacker/conf/nginx/{site}.conf`

## Access Sites

Sites are accessible at `https://{name}.test` or `http://{name}.test`
