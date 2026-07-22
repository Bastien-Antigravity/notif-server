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

The `notif-server` maintains a 100% success rate on its core test suite.

## 🛠️ Running Tests
```bash
go test -v ./...
```

## 🏗️ Test Architecture
1.  **Core Tests** (`src/core/`): Validates worker pool isolation, message routing, and load-shedding logic.
2.  **Server Tests** (`src/server/`): Verifies protocol listeners, idle timeouts, and framing robustness.
3.  **Integration Tests** (`cmd/test/`): End-to-end verification of the full ingestion-to-delivery flow using mock senders.

## 🧱 Mocking Standards
To test new features, use `nt.RegisterMockSender(ms)`. This helper ensures that the mock platform is correctly registered in the routing table and has its own worker pool initialized.
