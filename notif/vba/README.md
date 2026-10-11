# notif-server - VBA Module

VBA client class and FFI bridge bindings for `notif-server`, enabling Excel and Access applications to dispatch direct multi-channel notifications via `libnotif`.

## Usage

```vba
Dim client As New NotificationClient
client.Init "127.0.0.1:1027"

' 1-line general alert
client.Send "Model rebalanced successfully", "telegram"

' Categorized alert with severity level
client.SendAlert "CRITICAL", "Risk threshold exceeded: -4.5%", "risk_team,telegram"

' Alert with attachment
client.SendWithAttachment "REPORT", "Daily portfolio report", "portfolio.xlsx", "reports"
```
