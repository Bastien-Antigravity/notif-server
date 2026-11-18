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
# 🧪 Testing Playbook

The `notif-server` maintains a 100% pass rate across all packages under Go's race detector.

## 🛠️ Running Tests
```bash
go test -race -v ./...
```

## 🏗️ Test Architecture
1. **Core Tests (`src/core/`)**: Validates tag/level/fallback routing, worker isolation, zero-loss live hot-reload (including multiple same-type senders and custom `TAG`s), `RegisterSender` deduplication, idempotent draining `Stop()`, dynamic health state transitions, and drop counters under backpressure.
2. **Notifier Tests (`src/notifiers/`)**: Verifies instantiation-time decryption (plaintext & `ENC(...)`, fail-fast on bad ciphertext), concurrent dispatch safety, Gmail `os.OpenRoot` attachment confinement + 10 MB limit + CRLF sanitization, and bot token / webhook URL error redaction.
3. **Sanitizer Tests (`src/sanitizer/`)**: Verifies deny-by-default `IsPublicField`, ASCII/C1/Unicode header & filename sanitization, and exhaustive secret pattern redaction.
4. **REST & OpenMFE Tests (`src/rest/`)**: Verifies `/api/v1/contracts`, zero secret leakage in `/api/v1/config/alerting/get`, masked placeholder rejection & log redaction in `SetAlertingConfig`, and targeted single-provider testing (`POST /api/v1/notif/test`). Validate `src/rest/mfe.js` syntax with `node --check src/rest/mfe.js`.
5. **Server & E2E Tests (`src/server/`, `cmd/notif-server/`, `cmd/test/`)**: Verifies TCP/gRPC listeners, 10m idle timeout refresh, Telegram pre-flight token check, and full E2E dispatch flow.

## 🧱 Mocking Standards
Use `nt.RegisterSender(ms)` with thread-safe mock senders (protect shared assertion state with `sync.RWMutex` so tests stay clean under `go test -race`).
