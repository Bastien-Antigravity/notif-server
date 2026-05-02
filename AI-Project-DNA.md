# 🧬 Project DNA: notif-server

## 🎯 High-Level Intent (BDD)
- **Goal**: Provide a real-time notification engine for broadcasting system events (trades, alerts, reports) to multiple clients via TCP and gRPC.
- **Key Pattern**: **Fan-Out Distribution** (Single input message broadcasted to N subscribers) and **Message Buffering** for slow consumers.
- **Behavioral Source of Truth**: [[business-bdd-brain/02-Behavior-Specs/notif-server]]

## 🛠️ Role Specifics
- **Architect**: 
    - Ensure low-latency delivery (soft real-time) for critical trade notifications.
    - Maintain protocol consistency between gRPC and raw TCP listeners.
- **QA**: 
    - Stress test with 5k+ concurrent subscribers.
    - Verify that slow subscribers do not block the main distribution loop (Buffer Overflow handling).
- **Developer**:
    - Use `safe-socket` for all raw TCP connections.

## 🚦 Lifecycle & Versioning
- **Primary Branch**: `develop`
- **Protected Branches**: `main`, `master`
- **Versioning Strategy**: Semantic Versioning (vX.Y.Z).
- **Version Source of Truth**: `VERSION.txt`.
