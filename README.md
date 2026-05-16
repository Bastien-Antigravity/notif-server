---
microservice: notif-server
type: repository
status: active
language: go
tags:
- \'#service/notif-server\'
  - '#domain/observability'
  - '#domain/networking'
---

# Notif Server

**Notif Server** is a high-performance notification server written in Go. It acts as a central hub for dispatching notifications to various platforms such as Telegram, Discord, Matrix, and Gmail.

It is designed to be robust, scalable, and easy to integrate with other services via a TCP-based protocol using `safe-socket`.

## Features

- **Multi-Protocol Support**: Dual-protocol ingress via **gRPC (Protobuf)** and **TCP (Cap'n Proto)**.
- **Multi-Platform Dispatch**: Send notifications to Telegram, Discord, Matrix, and Gmail.
- **Tag-Based Routing**: Route messages to specific platforms based on tags (e.g., "INFO", "ALERT").
- **High Performance**: Built with Go's concurrency model (goroutines and channels) for efficient message processing.
- **Secure Communication**: Uses `safe-socket` for reliable and secure profile-based TCP communication.
- **Distributed Configuration**: Configurations and endpoints are managed via `distributed-config`.

## Architecture

The server listens for incoming connections on both TCP and gRPC ports. It deserializes notification messages (using Cap'n Proto or Protobuf), and dispatches them to the configured notifiers. All schemas are centrally managed in `src/schemas`.

## 🛡️ Feature Specs & Governance (BDD)
The behavior of this microservice is governed by strict specifications in the **[[business-bdd-brain|Business-Specs Brain]]**:
- **Tag-Based Routing**: [[FEAT-001-Tag-Based-Routing|FEAT-001: Conditional Dispatch]]
- **Sender Integrations**: [[FEAT-002-Sender-Integrations|FEAT-002: Multi-Platform Support]]
- **Unified Ingestion**: [[FEAT-003-Unified-Ingestion|FEAT-003: Dual Protocol Support]]

## Getting Started

### Prerequisites

- Go 1.25+
- A configuration file compatible with `distributed-config`.

### Installation

```bash
git clone https://github.com/Bastien-Antigravity/notif-server.git
cd notif-server
go mod download
```

### Running the Server

```bash
go run cmd/notif-server/main.go
```

## Configuration

The server relies on `distributed-config` to load capabilities (IP, Port) and notifier credentials. Ensure your configuration backend is set up correctly.

## Dependencies

- [safe-socket](https://github.com/Bastien-Antigravity/safe-socket)
- [distributed-config](https://github.com/Bastien-Antigravity/distributed-config)
- [flexible-logger](https://github.com/Bastien-Antigravity/flexible-logger)

## License

This project is licensed under the MIT License.
