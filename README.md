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

**Notif Server** is a high-performance notification server written in Go. It acts as a central hub for dispatching notifications to various platforms such as Telegram, Discord, Matrix, and Gmail.

It is designed to be robust, scalable, and stable under high load through architectural optimizations and hardened ingestion.

## Features

- **Multi-Protocol Ingress**: Dual-protocol support via **gRPC (Protobuf)** and **Hardened TCP (Cap'n Proto)**.
- **Worker Pool Dispatch**: Offloads external API calls (Telegram, Discord, etc.) to platform-specific worker pools to ensure zero backpressure on microservices.
- **Resilient Messaging**: Uses buffered queues and context-aware timeouts (30s) to handle slow or unresponsive external notification platforms.
- **Stable Host Identity**: Automatically strips dynamic ports for consistent client identification and tracking.
- **Tag-Based Routing**: Conditions dispatch based on message tags (e.g., "ALERT", "TRADING").
- **Resource Protection**: Enforces a 10-minute `IdleTimeout` to automatically prune zombie connections.
- **Full Observability**: Integrated with `universal-logger` for structured, leveled logging across all dispatch stages.

## Architecture

For a technical deep-dive into the system design, components, and data flow, please refer to [ARCHITECTURE.md](ARCHITECTURE.md).

## Getting Started

### Prerequisites

- Go 1.25+
- External platform credentials (API tokens, chat IDs) configured via `distributed-config`.

### Build

```bash
go build -o notif-server cmd/notif-server/main.go
```

## API Protocol

The server enforces a strict ingestion protocol:

1.  **Handshake**: Mandatory identity exchange (Safe-Socket `tcp-hello` profile).
2.  **Framing**: Handled natively via `ReadMessage()`.
3.  **Timeouts**: 10-minute idle pruning and 30-second API dispatch limits.

## Project Structure

- `src/server/`: Hardened connection handling and dual-loop protocol orchestration.
- `src/core/`: Notifier hub with worker pools and dispatch routing.
- `src/notifiers/`: Context-aware platform-specific sender implementations.
- `src/schemas/`: Protobuf and Cap'n Proto definitions.

## 🛡️ Testing & Verification
```bash
go test -v ./src/...
```
Behavioral integrity is verified via the **Spec-First Protocol**.
