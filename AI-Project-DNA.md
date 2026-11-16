# Metadata
- Version: 0.0.1
- Classification: Level 1 Microservice

---
microservice: notif-server
type: note
status: active
tags:
- '#service/notif-server'
- '#type/note'
- '#state/active'
- '#zone/3-fleet'
---
# 🧬 Project DNA: notif-server

## 🎯 High-Level Intent (BDD)
- **Goal**: Centralized notification gateway for the microservice fleet.
- **Key Pattern**: **Aggregator / Worker-Pool Dispatch**.

## 🛠 Technical Constraints
- **Language**: Go
- **Ingress Protocols**: gRPC (Protobuf), TCP (Cap'n Proto).
- **Hardening**: Non-blocking worker pools per platform; 10m IdleTimeout; ReadMessage framing.
- **Architecture & Service Standard**: Adheres to [`./AGENTS.md`](AGENTS.md), workspace root [`../AGENTS.md`](../AGENTS.md), and `../obsidian-brain/03-Tech-Stack/02-Project-Architecture/Global-Architecture-Rules.md`.

## 👥 Roles & Responsibilities
- **Architect**: 
    - Ensure zero-backpressure dispatch logic.
    - Implement stable host-based identity resolution.
- **Developer**:
    - Use Go coding standards and universal-logger.
    - Maintain context-aware notifier implementations.
