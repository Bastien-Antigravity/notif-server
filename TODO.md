---
microservice: notif-server
type: task
status: active
tags:
- '#service/notif-server'
- '#type/task'
- '#state/active'
- '#zone/3-fleet'
---

# TODO List: notif-server

- [x] Initial service structure and configuration
- [x] Main Notification Provider Standard (FEAT-001 & FEAT-002: DefaultSender Telegram fallback, default level routing, transparent message banner)
- [x] CGO Shared Library Bridge (`libnotif`) and polyglot packages (`cpp`, `vba`, `python`, `rust`)
- [x] 3-Tier Testing Consolidation: Cleaned up redundant client test folders; client facade tests consolidated in microservice-toolbox and E2E in sandbox-testing
- [ ] Maintenance and continuous compliance checks
