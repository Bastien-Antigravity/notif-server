# libnotif

This directory contains the compiled CGO shared library and C ABI headers for the `notif-server` polyglot client ecosystem.

## Artifacts

- **`libnotif.h`**: The C header containing all function declarations.
- **`libnotif.dylib`**: Shared library for macOS systems.
- **`libnotif.so`**: Shared library for Linux/Unix systems.

## API Mapping & Parity Reference

| C ABI Function | Wire/Go Equivalent | Description |
| :--- | :--- | :--- |
| `Notif_Send(addr, lvl, msg, att, tags)` | `SendNotification(req)` | Dispatches notification with default 5.0s timeout. |
| `Notif_SendWithTimeout(addr, lvl, msg, att, tags, timeout)` | `SendNotification(req)` with context timeout | Dispatches notification with custom timeout. |
| `Notif_GetLastError()` | Error string | Retrieves the last error message from the CGO bridge. |
| `Notif_FreeString(ptr)` | `free(ptr)` | Mandatory helper to free Go-allocated error strings. |
| `Notif_Close()` | `grpc.ClientConn.Close()` | Cleans up active gRPC channel and resets state. |
