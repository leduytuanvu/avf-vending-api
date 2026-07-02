> **Relocated from avf-vending-system** — canonical home: `avf-vending-api/docs/reports/postman/`. System copy removed 2026-06-16.

﻿# Admin Login Script Root Cause

Generated: 2026-06-12T09:00:34.6193337Z

## Result
- Status: PASS (in-memory admin auth helper)
- Login HTTP: 200
- Token path: tokens.accessToken
- /v1/auth/me: verified

## Prior failure
- Scripts used curl -d with PowerShell JSON (Windows encoding risk -> HTTP 400 invalid_json)
- Parsed .accessToken instead of tokens.accessToken

## Fix
- UTF-8 body file + curl --data-binary
- Postman-shaped headers
- Shared scripts/lib/admin-auth.ps1

