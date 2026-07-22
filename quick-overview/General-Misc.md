---
tags:
- '#ai/ignore'
- '#zone/3-fleet'
- '#service/notif-server'
- '#type/overview'
- '#state/active'
microservice: notif-server
type: overview
status: active
---
# 📚 General & Misc: `notif-server`

This document details the code standards, naming conventions, project layout, configuration structure, and version tracking mechanism governing the `notif-server` service.

---

## 👥 Ecosystem Governance & Code Standards

`notif-server` strictly adheres to the ecosystem-wide **Squad Protocol** standard. Every Go file is required to implement the following formatting structures:

### 1. Triple-Block Headers
Every source file must begin with a structured comment block defining its:
*   **Essential Process**: High-level explanation of the file's purpose.
*   **Data Flow**: A numeric trace showing incoming and outgoing data transitions.
*   **Key Parameters**: Definitions of the primary state variables or parameters used.

Example:
```go
/*
ESSENTIAL PROCESS:
Defines the standard interface for notification senders (e.g., Telegram, Discord).
Ensures that all notification platforms implement a unified dispatch method.

DATA FLOW:
1. Core dispatcher receives a notification.
2. Identifies the target sender implementation.
3. Calls SendMessage with context and payload.

KEY PARAMETERS:
- ctx: Execution context with timeout.
- msg: The primary notification text.
- to: Target recipient or room (platform-specific).
- subject: Optional metadata or header.
*/
```

### 2. 4-Block Imports
Imports must be grouped into four distinct, alphabetical blocks separated by whitespace:
1.  **Standard Library**: e.g., `"context"`, `"time"`
2.  **Local Packages**: e.g., `"github.com/Bastien-Antigravity/notif-server/src/interfaces"`
3.  **Ecosystem Libraries**: e.g., `"github.com/Bastien-Antigravity/distributed-config"`
4.  **External Third-Party**: e.g., `"capnproto.org/go/capnp/v3"`

### 3. Naming Conventions
To maintain structural consistency, the following prefix standards are enforced:
*   `I` Prefix: Reserved strictly for Interfaces (e.g., `INotifier`, `INotifSender`).
*   `M` Prefix: Reserved for data-only Model structs (e.g., `MNotifMessage`).
*   `Base` Prefix: Reserved for base structures designed to be extended (e.g., `BaseSender`).

---

## 📂 Directory Layout

```text
notif-server/
├── cmd/
│   ├── notif-server/       # Standalone entrypoint (main.go)
│   └── test/               # E2E Integration test suite
├── src/
│   ├── core/               # Routing, worker pool orchestration, & handlers
│   ├── interfaces/         # Universal interface definitions (INotifier, INotifSender)
│   ├── notifiers/          # Vendor-specific sender logic (Telegram, Gmail, Matrix, Discord)
│   ├── schemas/            # Protocols: Cap'n Proto (.capnp) & Protobuf (.proto)
│   └── server/             # Network listeners (TCP & gRPC server orchestration)
├── VERSION.txt             # Semantic version source (e.g., 1.2.0)
└── ARCHITECTURE.md         # Comprehensive architectural guidelines
```

---

## ⚙️ Configuration Setup

The application is bootstrapped using YAML profiles managed by `distributed-config` and `microservice-toolbox`.

*   **Profiles**: Configured profiles like `standalone` or `test` load respective YAML files:
    *   [`src/core/core.yaml`](file:///Users/imac/Desktop/Bastien-Antigravity/notif-server/src/core/core.yaml) configures default routing behaviors.
    *   [`src/server/test.yaml`](file:///Users/imac/Desktop/Bastien-Antigravity/notif-server/src/server/test.yaml) controls capability endpoints during verification.
*   **Version Reference**: Version information is centralized in [`VERSION.txt`](file:///Users/imac/Desktop/Bastien-Antigravity/notif-server/VERSION.txt) (currently `1.2.0`) to ensure consistency across automated build packaging pipelines.
