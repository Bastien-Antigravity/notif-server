---
microservice: notif-server
type: architecture
status: active
tags:
- '#service/notif-server'
- '#domain/observability'
- '#domain/networking'
- '#zone/3-fleet'
---

# Architecture Documentation

This document describes the high-level architecture of the `notif-server`.

## Overview

The `notif-server` is responsible for receiving notification requests from clients (likely via the `flexible-logger` library) and forwarding them to external platforms (Telegram, Discord, etc.).

## Core Components

### 1. Server (`src/server`)
- **Role**: The entry point of the application.
- **Function**:
    - Initializes the `distributed-config` and `Notifier` core.
    - Listens for incoming TCP connections using `safe-socket` (Cap'n Proto).
    - Listens for incoming gRPC connections using `google.golang.org/grpc` (Protobuf).
    - Spawns a goroutine for each TCP connection (`handleConnection`).
    - **Hardened Ingestion**: Uses `ReadMessage()` for robust framing and enforces a 10-minute `IdleTimeout` to prune zombie connections.
    - **Stable Identity**: Strips dynamic ports from client addresses to maintain consistent identity tracking.

### 2. Notifier (`src/core`)
- **Role**: The central logic hub and dispatcher.
- **Function**:
    - **Worker Pool Pattern**: Manages dedicated worker pools (5 workers per platform) and buffered queues (1000 messages) for each external sender.
    - **Zero Backpressure**: Asynchronous dispatch ensures that slow external APIs do not block the ingestion layer.
    - **Context-Aware**: Uses `context.Context` with a 30-second timeout for all external API calls to prevent hanging workers.

### 3. Notifiers (`src/notifiers`)
- **Role**: Implementations of external service integrations.
- **Supported Services**: Telegram, Discord, Matrix, Gmail.
- **Interface**: Each notifier implements the `NotifSenderInterface` with `context.Context` support.

### 4. Schemas (`src/schemas`)
- **Role**: Serialization contracts (Cap'n Proto and Protobuf).

## Data Flow

1.  **Client** sends a serialized message via **TCP (Cap'n Proto)** or **gRPC (Protobuf)**.
2.  **Server** accepts and identifies the connection (Stable Identity).
3.  **Ingestion**: Message is deserialized and pushed to the unified `Notifier.NotifChan`.
4.  **Dispatch**: `Notifier` looks up the target platforms based on message **Tags**.
5.  **Queueing**: Message is pushed to the platform-specific **Worker Queue**.
6.  **Execution**: A dedicated worker picks up the message and performs the HTTP/SMTP call with a 30s timeout.

## Diagram

```mermaid
graph TD
    Client_TCP[Client (Capnp)] -->|TCP| Server_TCP(Server Listener)
    Client_GRPC[Client (gRPC)] -->|Protobuf| Server_GRPC(gRPC Server)
    
    Server_TCP -->|ReadMessage| DeSer[Capnp Deserializer]
    DeSer -->|NotifMessage| NotifChan(NotifChan)
    
    Server_GRPC -->|NotifRequest| NotifChan
    
    NotifChan --> Router[Notifier Router]
    Router -- Tag: ALERT --> Queue_TG[Telegram Queue]
    Router -- Tag: INFO --> Queue_DS[Discord Queue]
    
    subgraph "Worker Pools"
        Queue_TG --> Workers_TG[Telegram Workers x5]
        Queue_DS --> Workers_DS[Discord Workers x5]
    end
    
    Workers_TG --> API_TG[Telegram API]
    Workers_DS --> API_DS[Discord API]
```
