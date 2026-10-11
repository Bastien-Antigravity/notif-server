# AGENTS.md: notif-server

## Service Mission & Architecture Role
`notif-server` is the multi-channel notification and alerting hub for the Bastien-Antigravity fleet. It consumes alert requests from microservices and dispatches them across Telegram, Discord, Email, and Webhook sinks. It also exports an OpenMFE micro-frontend that dynamically registers with `web-interface`.

- **Exposed Capability**: `notif_server`
  - **TCP Ingestion Protocol**: (`appConfig.GetListenAddr("notif_server")`) — framed TCP ingestion via `safe-socket`
  - **gRPC Service & Control**: (`appConfig.GetGRPCListenAddr("notif_server")`) — Protobuf ingestion and management
  - **REST & OpenMFE**: (`appConfig.GetRESTAddr("notif_server")`) — HTTP REST management and OpenMFE host
- **Downstream Integrations**: `web-interface`, Telegram, Discord APIs
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
6. **Main Notification Provider Standard**:
   - **Mandatory Default**: Telegram (`capabilities.notif_server`) is the authoritative primary notification provider (`DefaultSender` / `DefaultTag`), initialized on startup.
   - **4 Invariant Roles of the Main Provider**:
     1. **Default Ingestion Sink** (BDD `FEAT-001`): Untagged messages, unmapped tags, or unmapped levels route directly to `DefaultSender`. To make fallback transparent, the message content is prefixed with `⚠️ [Default Fallback - <Reason>]` so it is clearly visible in the delivered alert. Never broadcast untagged alerts to all providers.
     2. **System Error Sink**: Internal operational logs and daemon errors route exclusively to `DefaultSender`, never to secondary domain channels (e.g. Discord, client emails).
     3. **Universal Fallback Sink** (BDD `FEAT-002`): When an auxiliary provider (Discord, Matrix, Gmail) fails to deliver, worker logs standard `Logger.Error(...)` and dispatches the fallback alert directly to `DefaultSender`.
     4. **Terminal Sink (Loop-Breaker)**: The main provider is terminal. If it fails, it logs to disk/stderr and halts fallback. A provider's own dispatch failure log is never routed back into its own queue.
     5. **Default Severity Routing**: On startup, only `CRITICAL`, `ERROR`, and `WARNING` map implicitly to `DefaultSender` (`telegram`). `INFO` and lower levels are not routed to Telegram unless explicitly configured or tagged.
   - **No Unnecessary Fallback Wrapping**: Providers are registered directly into `TagToSenderMap`. Do not wrap providers in redundant nested fallback decorators.
7. **Pre-flight Bot Token Verification & Fail-Fast**:
   - On daemon startup, `verifyTelegramConfig()` queries Telegram `getMe` to validate the bot token strictly in `capabilities.notif_server` (no fallback to other services like `tele_remote`).
   - If the token is missing, un-decryptable, or rejected by the Telegram API (`ok == false`), the daemon fails fast immediately (`Logger.Critical(...)` and `os.Exit(1)`).
   - Authoritative primary Telegram configuration parameters (`chat_id`, `token`, `url`) belong directly inside the `notif_server:` capability block in ecosystem manifests (`native.yaml`, `notif-server.yaml`). Separate `primary: "true"` attributes are strictly unnecessary and deprecated.
8. **Polyglot Client & CGO Shared Library (`libnotif`)**:
   - `notif-server` provides the authoritative CGO shared library bridge (`cmd/libnotif/main.go` -> `notif/libnotif/libnotif.{dylib,so,h}`), exporting C ABI functions (`Notif_Send`, `Notif_SendWithTimeout`, `Notif_GetLastError`, `Notif_FreeString`, `Notif_Close`).
   - Polyglot bindings and client wrappers reside under `notif/` (`notif/libnotif/`, `notif/cpp/`, `notif/vba/`, `notif/python/`, `notif/rust/`), adhering to the exact same architectural pattern as `universal-logger` (`libunilog` / `unilog/`) and `distributed-config` (`libdistconf` / `distconf/`).
   - Downstream facades across the fleet (such as `microservice-toolbox`) bind to `libnotif` to dispatch direct gRPC alerts without redundant gRPC client compilation overhead.
   - **Testing Boundary Standard**: In accordance with the 3-Tier Testing Pyramid, `notif-server` tests its internal Go daemon (`go test ./...`) and compiles `libnotif`. Comprehensive end-user polyglot client testing is centrally owned by `microservice-toolbox` (`tests/` across Go, Python, Rust, C++), and multi-service E2E scenario validation is owned by `sandbox-testing`. Redundant sub-tests are not maintained inside `notif-server/notif/`.

