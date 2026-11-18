---
microservice: notif-server
type: repository
status: active
language: go
tags:
- '#service/notif-server'
- '#domain/observability'
- '#domain/networking'
- '#zone/3-fleet'
- '#type/repository'
- '#state/active'
---

# Notif Server

**Notif Server** is a high-performance notification gateway written in Go. It acts as the centralized hub for the Bastien-Antigravity fleet, dispatching critical alerts and logs to platforms like Telegram, Discord, Matrix, and Gmail.

## 🚀 Key Features

- **Multi-Protocol Ingress**: Simultaneous support for **gRPC (Protobuf)** and **Hardened TCP (Cap'n Proto)**.
- **Resilient Dispatch**:
    - **Isolated Provider Workers**: 1 dedicated sequential worker and buffered queue per configured provider (preserves strict message ordering, avoids API rate-limit violations, and isolates slow endpoints).
    - **Exponential Backoff**: Smart retries for transient network and API errors.
    - **Zero Backpressure**: Asynchronous architecture with tracked load-shedding ensures slow APIs never block ingestion.
    - **Real-Time Telemetry**: Dynamic provider health states (`Initialized`, `Operational`, `Degraded`, `Unavailable`, `Stopped`), queue depth, delivery/failure counts, and drop counters via `/api/v1/status` and gRPC `GetStatus`.
- **Architectural Hardening**:
    - **Stable Identity**: Host-based tracking (strips dynamic ports).
    - **Graceful Lifecycle**: Live hot-reload with sequential queue draining and idempotent shutdown.
    - **Secret Isolation & Sanitization**: Instantiation-time credential decryption (immutable senders), deny-by-default config masking for `web-interface`/OpenMFE, kernel-confined Gmail attachments (`os.OpenRoot`), and centralized secret redaction (`src/sanitizer`).
    - **Resource Protection**: 10-minute idle pruning and tracked load-shedding during spikes.
- **Universal Standards**: Adheres strictly to ecosystem Triple-Block headers, 4-block imports, and `VERSION.txt` single-source versioning.

## 🏗️ Architecture

For architecture details, behavior, and testing guides, refer to [quick-overview/Architecture-Overview.md](quick-overview/Architecture-Overview.md) and [AGENTS.md](AGENTS.md).

## 🛠️ Usage

### Prerequisites
- Go 1.25+
- External API credentials configured in `distributed-config`.

### Run
```bash
go run cmd/notif-server/main.go
```

### Test
```bash
go test -race -v ./...
```

## 📂 Project Structure

- `cmd/notif-server/`: Main daemon entrypoint and Telegram pre-flight verification.
- `cmd/libnotif/`: CGO shared library bridge (`libnotif`) for polyglot fleet bindings under `notif/`.
- `src/server/`: Hardened dual-protocol (TCP & gRPC) server orchestration and identity resolution.
- `src/core/`: The `Notifier` hub, tag/level router, controller, graceful hot-reload, and live metrics.
- `src/notifiers/`: Immutable, timeout-guarded provider integrations (Telegram, Discord, Matrix, Gmail).
- `src/sanitizer/`: Deny-by-default secret masking, MIME header/filename sanitization, and error redaction.
- `src/rest/`: HTTP REST management API, `/api/v1/contracts`, and embedded OpenMFE (`mfe.js`).
- `src/grpc_control/`: gRPC control plane (`NotifControlService`) with masked config exposure.
- `src/telegram/`: Interactive Tele-Remote bot control menus.
- `src/interfaces/`: Contract definitions (`INotifier`, `INotifSender`).
- `src/schemas/`: Serialization contracts (Cap'n Proto & Protobuf).

## 📜 Governance
This project follows the **Squad Protocol** for Go Systems. Every file includes mandatory **Triple-Block Headers** and organized **4-Block Imports**.
