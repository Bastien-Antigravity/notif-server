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
# 🚀 Features & Behavior

## 🛠️ Multi-Platform Support
- **Telegram**: Bot API integration with retry logic.
- **Discord**: Webhook-based messaging.
- **Matrix**: Unified integration support.
- **Gmail**: Hardened SMTP flow with STARTTLS and Implicit TLS (465) support.

## 🛡️ Reliability Features
- **Load-Shedding**: If a platform's worker queue is full (1000 messages), the server drops new notifications for that platform to prevent Out-Of-Memory (OOM) failures.
- **Pruning**: Zombie TCP connections are automatically closed after 10 minutes of inactivity.
- **Exponential Backoff**: doubling wait time between retries: 500ms ➡️ 1s ➡️ 2s.

## 📦 Ingestion Protocol
- **Framing**: Robust message delimitation via `safe-socket` framing.
- **Concurrency**: Verified to handle bursts of **5,000 notifications in < 1 second**.
