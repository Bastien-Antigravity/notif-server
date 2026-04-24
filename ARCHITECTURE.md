---
microservice: notif-server
type: architecture
status: active
tags:
  - domain/observability
  - domain/networking
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
    - Reads raw bytes from the TCP socket and sends them to the `Notifier`'s raw channel.

### 2. Notifier (`src/core`)
- **Role**: The central logic hub.
- **Function**:
    - Manages a map of `TagToSenderMap` which routes notification tags to specific senders.
    - `ConsumeRawMessages()`: Listens on `RawNotifChan`, deserializes messages (using Cap'n Proto definitions), and forwards them to `NotifChan`.
    - `SendNotification()`: Implements the gRPC service interface, converting Protobuf requests to internal messages.
    - `processMessage()`: Listens on `NotifChan`, looks up the sender based on the message's tags, and triggers the `SendMessage` method implementation.
    - **Initialization**: Loads senders (Telegram, Discord, etc.) based on the provided configuration.

### 3. Notifiers (`src/notifiers`)
- **Role**: Implementations of external service integrations.
- **Supported Services**:
    - Telegram
    - Discord
    - Matrix
    - Gmail
- **Interface**: Each notifier implements the `NotifSenderInterface`.

### 4. Schemas (`src/schemas`)
- **Role**: Serialization contracts.
- **Structure**:
    - `capnp/`: Contains Cap'n Proto definitions for binary streaming.
    - `protobuf/`: Contains Protobuf definitions and generated gRPC service code.

## Data Flow

1.  **Client** sends a serialized message via **TCP (Cap'n Proto)** or **gRPC (Protobuf)**.
2.  **Server** accepts the connection.
3.  **Cap'n Proto path**: Raw data is pushed to `Notifier.RawNotifChan` and deserialized.
4.  **gRPC path**: `SendNotification` RPC directly creates an internal `NotifMessage`.
5.  **Notifier** pushes unified `NotifMessage` to `Notifier.NotifChan`.
6.  **Notifier** processes the message, checks its **Tags**.
7.  **Notifier** finds the corresponding **Notifier** (e.g., Telegram) for the tag.
8.  **Notifier** executes the API call to the external service.

## Diagram

```mermaid
graph TD
    Client_TCP[Client (Capnp)] -->|TCP| Server_TCP(Server Listener)
    Client_GRPC[Client (gRPC)] -->|Protobuf| Server_GRPC(gRPC Server)
    
    Server_TCP -->|Raw Bytes| RawChan(RawNotifChan)
    RawChan --> DeSer[Capnp Deserializer]
    DeSer -->|NotifMessage| NotifChan(NotifChan)
    
    Server_GRPC -->|NotifRequest| NotifChan
    
    NotifChan --> Router[Notifier Router]
    Router -- Tag: ALERT --> Telegram[Telegram Sender]
    Router -- Tag: INFO --> Discord[Discord Sender]
    Telegram --> API_TG[Telegram API]
    Discord --> API_DS[Discord API]
```
