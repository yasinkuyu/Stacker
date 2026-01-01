---
description: How to install and manage PHP versions
---

# PHP Management

## Install PHP via Web UI

1. Open http://localhost:9999
2. Click "PHP" in sidebar
3. Click "Install PHP" button
4. Select version (8.3, 8.2, 8.1, 8.0, 7.4)
5. Toggle extensions (XDebug, Redis, ImageMagick)
6. Click "Save"

## Install PHP via API

```bash
curl -X POST http://localhost:9999/api/php/install \
  -H "Content-Type: application/json" \
  -d '{"version":"8.3","xdebug":true}'
```

## Set Default PHP Version

```bash
curl -X PUT http://localhost:9999/api/php/default \
  -H "Content-Type: application/json" \
  -d '{"version":"8.3"}'
```

## PHP Files Location

- Binaries: `~/Library/Application Support/Stacker/bin/php{version}/`
- Config: `~/Library/Application Support/Stacker/conf/php/php{version}.ini`
- Default: `~/Library/Application Support/Stacker/php_default.txt`

## Switch PHP for Specific Site

Edit the site and select a PHP version in the dropdown.