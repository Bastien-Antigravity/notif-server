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

`notif-server` strictly adheres to the ecosystem-wide **Squad Protocol** standard (`02-Go-Memory-and-Concurrency.md`, `11-Unified-Comment-Standards.md`, `01-General-Naming-Conventions.md`). Every Go file implements:

### 1. Triple-Block Headers & Section Dividers
Every Go source file begins with a structured comment block defining `ESSENTIAL PROCESS`, `DATA FLOW`, and `KEY PARAMETERS`, and separates exported functions/major blocks with `// -----------------------------------------------------------------------------`.

### 2. 4-Block Imports
Imports are grouped into four distinct blocks separated by a blank line:
1. **Standard Library**: e.g., `"context"`, `"mime"`, `"time"`
2. **Local Packages**: e.g., `"github.com/Bastien-Antigravity/notif-server/src/sanitizer"`
3. **Ecosystem Libraries** (explicitly aliased): e.g., `toolbox_config "github.com/Bastien-Antigravity/microservice-toolbox/go/pkg/config"`
4. **External Third-Party**: e.g., `"github.com/stretchr/testify/assert"`

### 3. Naming Conventions
- New Go interfaces use descriptive nouns without `I`-prefix (per `01-General-Naming-Conventions.md`, e.g., `NotifController`); legacy `INotifier` and `INotifSender` are preserved for backwards compatibility.

---

## 📂 Directory Layout

```text
notif-server/
├── cmd/
│   ├── libnotif/           # CGO shared library bridge (libnotif.{dylib,so,h})
│   ├── notif-server/       # Standalone daemon entrypoint (main.go + verifyTelegramConfig)
│   └── test/               # E2E integration test suite
├── notif/                  # Polyglot client bindings (cpp, python, rust, vba, libnotif)
├── src/
│   ├── core/               # Notifier engine, router, controller, live hot-reload & metrics
│   ├── grpc_control/       # NotifControlService gRPC handlers & Protobuf definitions
│   ├── interfaces/         # Core interface contracts (INotifier, INotifSender)
│   ├── notifiers/          # Immutable provider senders (Telegram, Discord, Matrix, Gmail)
│   ├── rest/               # HTTP REST API, /api/v1/contracts & embedded OpenMFE (mfe.js)
│   ├── sanitizer/          # Deny-by-default masking, header/filename sanitizer & redaction
│   ├── schemas/            # Cap'n Proto (.capnp) & Protobuf (.proto) ingestion schemas
│   ├── server/             # Dual-protocol TCP & gRPC server orchestration
│   └── telegram/           # Tele-Remote interactive control menu manager
├── quick-overview/         # Concise architecture, features, and testing playbooks
├── AGENTS.md               # Authoritative AI development & integration guidelines
└── VERSION.txt             # Semantic version single source of truth
```

---

## ⚙️ Configuration & Version Setup

- **Profiles**: `standalone.yaml` links to central ecosystem configuration (`../docker-deployment/modes/local/config/native.yaml`), loaded via `microservice-toolbox` and `distributed-config`.
- **Credentials**: Both plaintext and `ENC(...)` values are supported natively by `appConfig.DecryptSecret()` and decrypted once at sender instantiation.
- **Version Reference**: Centralized strictly in [`VERSION.txt`](file:///Users/imac/Desktop/Bastien-Antigravity/notif-server/VERSION.txt) and resolved via `toolbox_version.Get()`.
