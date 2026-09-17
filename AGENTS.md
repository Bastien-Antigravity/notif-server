# AGENTS.md: notif-server

## Service Mission & Architecture Role
`notif-server` is the multi-channel notification and alerting hub for the Bastien-Antigravity fleet. It consumes alert requests from microservices and dispatches them across Telegram, Discord, Email, and Webhook sinks. It also exports an OpenMFE micro-frontend that dynamically registers with `web-interface`.

- **Exposed Capability**: `notif_server` (Port: `8095` REST management / OpenMFE host)
- **Downstream Integrations**: `web-interface` (`127.0.0.1:5000`), Telegram, Discord APIs
- **Libraries**: `microservice-toolbox`, `universal-logger`, `distributed-config`
- **Configuration Link**: `standalone.yaml -> ../docker-deployment/modes/local/config/native.yaml`

## Key Build & Test Commands
```bash
# Build binary
go build -o bin/notif-server ./cmd/notif-server

# Run tests
go test -v ./...

# Run service
./bin/notif-server
```

## AI Development & Integration Guidelines
1. **Dynamic Web Registration**: OpenMFE UI registers with `web-interface` at `http://<webAddr>/api/v1/register`. Ensure fallback address defaults to `127.0.0.1:5000` (never legacy 8080).
2. **Encrypted Credentials**: Telegram bot tokens and Discord webhook URLs must be loaded as encrypted strings (`ENC(...)`) and decrypted via `appConfig.DecryptSecret()`.
3. **Resilient Dispatch**: Outgoing HTTP calls to notification providers must use timeout-guarded clients and circuit breakers from `microservice-toolbox/go/pkg/resilience`.
4. **Header Ritual**: All Go source files MUST begin with the Triple-Block header (`ESSENTIAL PROCESS`, `DATA FLOW`, `KEY PARAMETERS`).
5. **Section Dividers**: Use `// -----------------------------------------------------------------------------` between exported methods and major blocks.
