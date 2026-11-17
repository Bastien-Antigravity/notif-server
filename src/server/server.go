package server

/*
ESSENTIAL PROCESS:
Initializes and manages the dual-protocol notification server (TCP & gRPC).
Coordinates between the ingestion layer and the Notifier core.

DATA FLOW:
1. Server starts and binds to configured TCP and gRPC addresses.
2. Accepts incoming TCP connections (Cap'n Proto) or gRPC calls (Protobuf).
3. Routes deserialized messages to the Notifier core.

KEY PARAMETERS:
- AppConfig: Shared microservice configuration.
- Notifier: The core dispatching engine.
*/

import (
	"fmt"
	"sync"
	"time"

	notifier "github.com/Bastien-Antigravity/notif-server/src/core"
	"github.com/Bastien-Antigravity/notif-server/src/grpc_control"
	proto_msg "github.com/Bastien-Antigravity/notif-server/src/schemas/protobuf"

	toolbox_config "github.com/Bastien-Antigravity/microservice-toolbox/go/pkg/config"
	"github.com/Bastien-Antigravity/microservice-toolbox/go/pkg/network"
	factory "github.com/Bastien-Antigravity/safe-socket"
	socket_interfaces "github.com/Bastien-Antigravity/safe-socket/src/interfaces"
	"github.com/Bastien-Antigravity/universal-logger/src/interfaces"
)

type Server struct {
	Logger     interfaces.Logger
	AppConfig  *toolbox_config.AppConfig
	Notifier   *notifier.Notifier
	Controller notifier.NotifController
	shutdown   chan struct{}
	stopOnce   sync.Once
	serverSock socket_interfaces.Socket // Store the listener socket
	grpcMu     sync.Mutex
	grpcSrv    *network.GRPCServer
}

// -----------------------------------------------------------------------------

// NewServer creates a new Notification Server.
func NewServer(ac *toolbox_config.AppConfig, logger interfaces.Logger, notif *notifier.Notifier, controller notifier.NotifController) *Server {
	return &Server{
		AppConfig:  ac,
		Logger:     logger,
		Notifier:   notif,
		Controller: controller,
		shutdown:   make(chan struct{}),
	}
}

// -----------------------------------------------------------------------------

// Stop shuts down the server.
func (s *Server) Stop() {
	s.stopOnce.Do(func() {
		close(s.shutdown)
		s.grpcMu.Lock()
		if s.grpcSrv != nil {
			s.grpcSrv.Stop()
		}
		s.grpcMu.Unlock()
	})
}

// -----------------------------------------------------------------------------

// Start listens for incoming TCP and gRPC connections.
func (s *Server) Start() error {
	// 1. Resolve TCP address from config using Toolbox
	tcpAddr, err := s.AppConfig.GetListenAddr("notif_server")
	if err != nil {
		s.Logger.Error("Failed to resolve bind address: %v", err)
		return fmt.Errorf("failed to resolve bind address: %w", err)
	}

	// 2. Prepare and start gRPC Server if configured (safely guarded before background run)
	if grpcAddr, err := s.AppConfig.GetGRPCListenAddr("notif_server"); err == nil && grpcAddr != "" {
		s.grpcMu.Lock()
		s.grpcSrv = network.NewGRPCServerWithLogger(grpcAddr, s.Logger)
		proto_msg.RegisterNotifServiceServer(s.grpcSrv.Server, s.Notifier)
		controlImpl := grpc_control.NewControlService(s.Controller, s.Logger)
		grpc_control.RegisterNotifControlServiceServer(s.grpcSrv.Server, controlImpl)
		s.grpcMu.Unlock()

		go func() {
			s.Logger.Info("Notification Server gRPC listening on " + grpcAddr)
			if err := s.grpcSrv.Start(); err != nil {
				s.Logger.Error("gRPC server failed: %v", err)
			}
		}()
	} else {
		s.Logger.Debug("gRPC address not configured for notif_server; skipping gRPC listener: %v", err)
	}

	// 3. Start TCP Server with 10-minute idle timeout configuration
	config := factory.SocketConfig{
		Deadline: 10 * time.Minute,
	}
	s.serverSock, err = factory.CreateWithConfig("tcp-hello", tcpAddr, config, "server", true)
	if err != nil {
		return err
	}
	defer s.serverSock.Close()

	s.Logger.Info("Notification Server TCP listening on " + tcpAddr)

	// Background goroutine to handle shutdown signal
	go func() {
		<-s.shutdown
		s.serverSock.Close()
	}()

	for {
		conn, err := s.serverSock.Accept()
		if err != nil {
			select {
			case <-s.shutdown:
				return nil
			default:
				s.Logger.Error("Accept error: %v", err)
				continue
			}
		}
		go s.handleConnection(conn)
	}
}
