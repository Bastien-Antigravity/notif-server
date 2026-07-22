package grpc_control

import (
	"context"
	"fmt"
	"net"

	notif_core "github.com/Bastien-Antigravity/notif-server/src/core"
	unilog_interfaces "github.com/Bastien-Antigravity/universal-logger/src/interfaces"

	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
)

// -----------------------------------------------------------------------------
// GRPCService handles gRPC server lifecycle
// -----------------------------------------------------------------------------

type GRPCService struct {
	server     *grpc.Server
	listener   net.Listener
	logger     unilog_interfaces.Logger
	controller notif_core.NotifController
	running    bool

	ControlService *ControlServiceImpl
}

// -----------------------------------------------------------------------------

// NewGRPCService creates a new GRPCService instance
func NewGRPCService(controller notif_core.NotifController, logger unilog_interfaces.Logger, host string, port int) (*GRPCService, error) {
	// Create listener
	address := fmt.Sprintf("%s:%d", host, port)

	listener, err := net.Listen("tcp", address)
	if err != nil {
		return nil, fmt.Errorf("failed to listen on %s: %w", address, err)
	}

	// Create gRPC server with options
	serverOptions := []grpc.ServerOption{
		grpc.MaxRecvMsgSize(10 * 1024 * 1024), // 10MB
		grpc.MaxSendMsgSize(10 * 1024 * 1024), // 10MB
	}

	server := grpc.NewServer(serverOptions...)

	return &GRPCService{
		server:     server,
		listener:   listener,
		logger:     logger,
		controller: controller,
		running:    false,
	}, nil
}

// -----------------------------------------------------------------------------

// Start starts the gRPC server (Non-blocking)
func (g *GRPCService) Start() error {
	g.logger.Info("Starting gRPC management service on %s", g.listener.Addr().String())

	// Register services
	g.ControlService = NewControlService(g.controller, g.logger)
	RegisterNotifControlServiceServer(g.server, g.ControlService)

	// Register health service
	healthServer := health.NewServer()
	grpc_health_v1.RegisterHealthServer(g.server, healthServer)
	healthServer.SetServingStatus("grpc_control.NotifControlService", grpc_health_v1.HealthCheckResponse_SERVING)

	// Start server in goroutine
	go func() {
		g.running = true
		if err := g.server.Serve(g.listener); err != nil && err != grpc.ErrServerStopped {
			g.logger.Error("gRPC management server failed: %v", err)
		}
		g.running = false
	}()

	g.logger.Info("gRPC management service initialized successfully on %s", g.listener.Addr().String())
	return nil
}

// -----------------------------------------------------------------------------

// Stop gracefully stops the gRPC server
func (g *GRPCService) Stop(ctx context.Context) error {
	g.logger.Info("Stopping gRPC management service...")

	if g.server != nil {
		// Graceful stop
		done := make(chan struct{})
		go func() {
			g.server.GracefulStop()
			close(done)
		}()

		select {
		case <-ctx.Done():
			g.logger.Warning("gRPC graceful shutdown timeout, forcing stop...")
			g.server.Stop()
		case <-done:
			g.logger.Info("gRPC management service stopped gracefully")
		}
	}

	if g.listener != nil {
		g.listener.Close()
	}

	g.running = false
	return nil
}

// -----------------------------------------------------------------------------

// IsRunning returns whether the gRPC server is running
func (g *GRPCService) IsRunning() bool {
	return g.running
}
