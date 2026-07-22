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
	"os"
	"time"

	notifier "github.com/Bastien-Antigravity/notif-server/src/core"
	"github.com/Bastien-Antigravity/notif-server/src/grpc_control"
	proto_msg "github.com/Bastien-Antigravity/notif-server/src/schemas/protobuf"

	factory "github.com/Bastien-Antigravity/safe-socket"
	socket_interfaces "github.com/Bastien-Antigravity/safe-socket/src/interfaces"
	toolbox_config "github.com/Bastien-Antigravity/microservice-toolbox/go/pkg/config"
	"github.com/Bastien-Antigravity/microservice-toolbox/go/pkg/network"
	"github.com/Bastien-Antigravity/universal-logger/src/interfaces"
)

type Server struct {
	Logger     interfaces.Logger
	AppConfig  *toolbox_config.AppConfig
	Notifier   *notifier.Notifier
	Controller notifier.NotifController
	shutdown   chan struct{}
	serverSock socket_interfaces.Socket // Store the listener socket
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
	close(s.shutdown)
	if s.grpcSrv != nil {
		s.grpcSrv.Stop()
	}
}

// -----------------------------------------------------------------------------

// Start listens for incoming TCP and gRPC connections.
func (s *Server) Start() error {
	// 1. Resolve TCP address from config using Toolbox
	tcpAddr, err := s.AppConfig.GetListenAddr("notif_server")
	if err != nil {
		s.Logger.Error("Failed to resolve bind address: %v", err)
		os.Exit(1)
	}

	// 2. Start gRPC Server in background
	go func() {
		// Use toolbox convention for gRPC port
		grpcAddr, err := s.AppConfig.GetGRPCListenAddr("notif_server")
		if err != nil {
			s.Logger.Error("Failed to resolve gRPC address: %v", err)
			return
		}

		s.Logger.Info("Notification Server gRPC listening on " + grpcAddr)
		s.grpcSrv = network.NewGRPCServerWithLogger(grpcAddr, s.Logger)
		proto_msg.RegisterNotifServiceServer(s.grpcSrv.Server, s.Notifier)

		// Register NotifControlServiceServer on the same server
		controlImpl := grpc_control.NewControlService(s.Controller, s.Logger)
		grpc_control.RegisterNotifControlServiceServer(s.grpcSrv.Server, controlImpl)

		if err := s.grpcSrv.Start(); err != nil {
			s.Logger.Error("gRPC server failed: %v", err)
		}
	}()

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
