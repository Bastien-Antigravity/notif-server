# === BUILD STAGE ===
FROM golang:1.25-alpine AS builder

LABEL org.opencontainers.image.source="https://github.com/Bastien-Antigravity/notif-server"

# Install build dependencies
RUN apk add --no-cache git gcc musl-dev ca-certificates tzdata

WORKDIR /workspace

# Clone shared library modules for replace directives in builder stage
RUN git clone --depth 1 -b develop https://github.com/Bastien-Antigravity/microservice-toolbox.git /workspace/microservice-toolbox && \
    git clone --depth 1 -b develop https://github.com/Bastien-Antigravity/distributed-config.git /workspace/distributed-config && \
    git clone --depth 1 -b develop https://github.com/Bastien-Antigravity/safe-socket.git /workspace/safe-socket && \
    git clone --depth 1 -b develop https://github.com/Bastien-Antigravity/universal-logger.git /workspace/universal-logger && \
    git clone --depth 1 -b develop https://github.com/Bastien-Antigravity/flexible-logger.git /workspace/flexible-logger

# Copy notif-server source
WORKDIR /workspace/notif-server
COPY . .

# Ensure dependencies are tidy and build binary
RUN go mod tidy && \
    CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /notif-server-bin ./cmd/notif-server

# === RUNTIME STAGE ===
FROM alpine:3.20

# Install runtime dependencies
RUN apk add --no-cache ca-certificates tzdata

WORKDIR /notif-server

# Copy the binary from the build stage
COPY --from=builder /notif-server-bin /notif-server/notif-server

# Set the entrypoint
ENTRYPOINT ["/notif-server/notif-server"]

