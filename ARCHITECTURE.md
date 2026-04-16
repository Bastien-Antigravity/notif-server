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
    - Initializes the `distributed-config` and `Notifie` core.
    - Listens for incoming TCP connections using `safe-socket`.
    - Spawns a goroutine for each connection (`handleConnection`).
    - Reads raw bytes from the socket and sends them to the `Notifie`'s raw channel.

### 2. Notifie (`src/core`)
- **Role**: The central logic hub.
- **Function**:
    - Manages a map of `TagToSenderMap` which routes notification tags to specific senders.
    - `consumeRawMessages()`: Listens on `RawNotifChan`, deserializes messages (using `models.NotifMessage`), and forwards them to `NotifChan`.
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

### 4. Models (`external`)
- **Role**: Data definitions.
- **Source**: `github.com/Bastien-Antigravity/flexible-logger/src/models`.
- **Key Struct**: `NotifMessage` (contains Message, Attachment, Tags).

## Data Flow

1.  **Client** sends a serialized notification message via TCP.
2.  **Server** accepts the connection and reads the raw data.
3.  **Server** pushes raw data to `Notifie.RawNotifChan`.
4.  **Notifie** consumes raw data, deserializes it into `NotifMessage`.
5.  **Notifie** pushes `NotifMessage` to `Notifie.NotifChan`.
6.  **Notifie** processes the message, checks its **Tags**.
7.  **Notifie** finds the corresponding **Notifier** (e.g., Telegram) for the tag.
8.  **Notifier** executes the API call to the external service.

## Diagram

```mermaid
graph TD
    Client[Client (Logger)] -->|TCP / serialized msg| Server(Server Listener)
    Server -->|Raw Bytes| RawChan(RawNotifChan)
    RawChan --> DeSer[Deserializer Worker]
    DeSer -->|NotifMessage| NotifChan(NotifChan)
    NotifChan --> Router[Notifie Router]
    Router -- Tag: ALERT --> Telegram[Telegram Sender]
    Router -- Tag: INFO --> Discord[Discord Sender]
    Telegram --> API_TG[Telegram API]
    Discord --> API_DS[Discord API]
```
