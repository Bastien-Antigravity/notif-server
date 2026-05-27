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
---

# Notif Server

**Notif Server** is a high-performance notification gateway written in Go. It acts as the centralized hub for the Bastien-Antigravity fleet, dispatching critical alerts and logs to platforms like Telegram, Discord, Matrix, and Gmail.

## 🚀 Key Features

- **Multi-Protocol Ingress**: Simultaneous support for **gRPC (Protobuf)** and **Hardened TCP (Cap'n Proto)**.
- **Resilient Dispatch**:
    - **Worker Pools**: 5 dedicated workers per platform to ensure isolation.
    - **Exponential Backoff**: Smart retries for transient network and API errors.
    - **Zero Backpressure**: Asynchronous architecture ensures slow APIs never block ingestion.
- **Architectural Hardening**:
    - **Stable Identity**: Host-based tracking (strips dynamic ports).
    - **Resource Protection**: 10-minute idle pruning and Load-Shedding during spikes.
- **Universal Standards**: Strictly adheres to the ecosystem-wide **I/M Naming Prefixes** and governance headers.

## 🏗️ Architecture

For a deep-dive into the components and data flow, refer to [ARCHITECTURE.md](ARCHITECTURE.md).

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
go test -v ./...
```

## 📂 Project Structure

- `src/server/`: Hardened protocol orchestration and identity resolution.
- `src/core/`: The `INotifier` hub, router, and worker pool manager.
- `src/notifiers/`: Resilience-aware integrations (Telegram, Discord, etc.).
- `src/interfaces/`: Universal contract definitions (`INotifier`, `INotifSender`).
- `src/schemas/`: Serialization contracts (Capnp/Proto).

## 📜 Governance
This project follows the **Squad Protocol** for Go Systems. Every file includes mandatory **Triple-Block Headers** and organized **4-Block Imports**.
