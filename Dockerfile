# === BUILD STAGE ===
FROM golang:1.25-alpine AS builder

LABEL org.opencontainers.image.source="https://github.com/Bastien-Antigravity/notif-server"

# Install build dependencies
RUN apk add --no-cache git gcc musl-dev ca-certificates tzdata

WORKDIR /workspace

# Copy required local modules for 'replace' directives
COPY microservice-toolbox ./microservice-toolbox
COPY universal-logger ./universal-logger
COPY distributed-config ./distributed-config
COPY safe-socket ./safe-socket
COPY flexible-logger ./flexible-logger

# Copy the target service
COPY notif-server ./notif-server

WORKDIR /workspace/notif-server

# Ensure dependencies are tidy for linux build
RUN go mod tidy && go mod download

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /notif-server-bin ./cmd/notif-server

# === RUNTIME STAGE ===
FROM alpine:3.20

# Install runtime dependencies
RUN apk add --no-cache ca-certificates tzdata

WORKDIR /notif-server

# Copy the binary from the build stage
COPY --from=builder /notif-server-bin /notif-server/notif-server

# Set the entrypoint
ENTRYPOINT ["/notif-server/notif-server"]
