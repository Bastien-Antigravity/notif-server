# Notif Server

**Notif Server** is a high-performance notification server written in Go. It acts as a central hub for dispatching notifications to various platforms such as Telegram, Discord, Matrix, and Gmail.

It is designed to be robust, scalable, and easy to integrate with other services via a TCP-based protocol using `safe-socket`.

## Features

- **Multi-Platform Support**: Send notifications to Telegram, Discord, Matrix, and Gmail.
- **Tag-Based Routing**: Route messages to specific platforms based on tags (e.g., "INFO", "ALERT").
- **High Performance**: Built with Go's concurrency model (goroutines and channels) for efficient message processing.
- **Secure Communication**: Uses `safe-socket` for reliable and secure TCP communication.
- **Distributed Configuration**: Configurations are managed via `distributed-config`.

## Architecture

The server listens for incoming connections, deserializes notification messages (using Cap'n Proto definitions handled by `flexible-logger` models), and dispatches them to the configured notifiers.

## Getting Started

### Prerequisites

- Go 1.25+
- A configuration file compatible with `distributed-config`.

### Installation

```bash
git clone https://github.com/Bastien-Antigravity/notif-server.git
cd notif-server
go mod download
```

### Running the Server

```bash
go run cmd/notif-server/main.go
```

## Configuration

The server relies on `distributed-config` to load capabilities (IP, Port) and notifier credentials. Ensure your configuration backend is set up correctly.

## Dependencies

- [safe-socket](https://github.com/Bastien-Antigravity/safe-socket)
- [distributed-config](https://github.com/Bastien-Antigravity/distributed-config)
- [flexible-logger](https://github.com/Bastien-Antigravity/flexible-logger)

## License

This project is licensed under the MIT License.
