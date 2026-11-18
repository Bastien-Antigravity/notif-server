---
microservice: notif-server
type: overview
status: active
tags:
- '#service/notif-server'
- '#type/overview'
- '#state/active'
- '#ai/ignore'
---
# 🚀 Features & Behavior

## 🛠️ Multi-Platform Support (Multiple Instances per Type Supported)
- **Telegram**: Primary authoritative sink (`DefaultSender`), Bot API with exponential retry and token redaction; verified at startup via `getMe`.
- **Discord**: Webhook-based JSON delivery with reusable HTTP client and webhook URL redaction.
- **Matrix**: Webhook/integration endpoint delivery with reusable HTTP client and URL redaction.
- **Gmail**: Hardened SMTP (STARTTLS on 587 & Implicit TLS on 465), kernel-confined attachments via `os.OpenRoot` (`ATTACHMENT_DIR`, 10 MB cap), RFC-formatted MIME headers (`mime.FormatMediaType`), and control-character stripping.

## 🛡️ Reliability & Security Features
- **Tracked Load-Shedding**: Both `NotifChan` (cap 100) and each provider queue (cap 100) shed load without blocking ingestion; drops are explicitly counted in `dropped_ingestion` and per-provider `DroppedCount` / `total_dropped`.
- **Dynamic Health States**: `Initialized` → `Operational` → `Degraded` (1–2 consecutive failures or queue >= 80%) → `Unavailable` (3+ consecutive failures) → automatic recovery on next success, plus `Stopped` during shutdown.
- **Deny-by-Default Secret Isolation (`src/sanitizer`)**: `web-interface` / OpenMFE receives only allowlisted public config fields; all credentials, webhook URLs, `ENC(...)` blobs, and unknown keys are masked (`********`) and write-only.
- **Targeted Single-Provider Testing**: OpenMFE and `POST /api/v1/notif/test` support an optional `"tag"` parameter to test any single provider exclusively without triggering level broadcasts.
- **Rate-Throttled UI Toasts**: OpenMFE inline toast banners cap visible items at 3 and auto-pause non-error toasts for 15s during high-rate bursts (>5 in 10s).
- **Idle Pruning**: Zombie TCP connections are automatically closed after 10 minutes of inactivity.
- **Exponential Backoff**: Doubling wait time between retries: `500ms → 1s → 2s`.
