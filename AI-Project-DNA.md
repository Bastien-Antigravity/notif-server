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
- **Key Pattern**: **Aggregator / Isolated Sequential Dispatch** (1 worker & cap-100 queue per provider).

## 🛠 Technical Constraints
- **Language**: Go
- **Ingress Protocols**: gRPC (Protobuf), TCP (Cap'n Proto), HTTP REST & OpenMFE.
- **Hardening**: Isolated sequential worker per provider; 10m IdleTimeout; ReadMessage framing; instantiation-time credential decryption (immutable senders); zero-loss live hot-reload & idempotent draining `Stop()`; deny-by-default secret masking & `src/sanitizer` scrubbing; `os.OpenRoot` Gmail attachment confinement.
- **Architecture & Service Standard**: Adheres to [`./AGENTS.md`](AGENTS.md), workspace root [`../AGENTS.md`](../AGENTS.md), and `../obsidian-brain/03-Tech-Stack/02-Project-Architecture/Global-Architecture-Rules.md`.

## 👥 Roles & Responsibilities
- **Architect**: 
    - Ensure zero-backpressure dispatch logic.
    - Implement stable host-based identity resolution.
- **Developer**:
    - Use Go coding standards and universal-logger.
    - Maintain context-aware notifier implementations.
