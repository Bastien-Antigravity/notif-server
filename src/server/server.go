package server

import (
	"os"
	"sync"

	notifier "github.com/Bastien-Antigravity/notif-server/src/core"
	proto_msg "github.com/Bastien-Antigravity/notif-server/src/schemas/protobuf"

	factory "github.com/Bastien-Antigravity/safe-socket"
	socket_interfaces "github.com/Bastien-Antigravity/safe-socket/src/interfaces"
	toolbox_config "github.com/Bastien-Antigravity/microservice-toolbox/go/pkg/config"
	"github.com/Bastien-Antigravity/microservice-toolbox/go/pkg/network"
	"github.com/Bastien-Antigravity/universal-logger/src/interfaces"
)

type Server struct {
	Logger        interfaces.Logger
	AppConfig     *toolbox_config.AppConfig
	Notifier      *notifier.Notifier
	listeners     map[string]socket_interfaces.TransportConnection
	listenersLock sync.RWMutex
	shutdown      chan struct{}
	serverSock    socket_interfaces.Socket // Store the listener socket
}

// -----------------------------------------------------------------------------

// NewServer creates a new Notification Server.
func NewServer(ac *toolbox_config.AppConfig, logger interfaces.Logger, notif *notifier.Notifier) *Server {
	return &Server{
		AppConfig: ac,
		Logger:    logger,
		Notifier:  notif,
		listeners: make(map[string]socket_interfaces.TransportConnection),
		shutdown:  make(chan struct{}),
	}
}

// -----------------------------------------------------------------------------

// Stop shuts down the server.
func (s *Server) Stop() {
	close(s.shutdown)
}

// -----------------------------------------------------------------------------

// Start listens for incoming TCP and gRPC connections.
func (s *Server) Start() error {
	// 1. Resolve TCP address from config using Toolbox
	tcpAddr, err := s.AppConfig.GetListenAddr("notif_server")
	if err != nil {
		s.Logger.Error("Failed to resolve bind address: " + err.Error())
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
		gSrv := network.NewGRPCServerWithLogger(grpcAddr, s.Logger)
		proto_msg.RegisterNotifServiceServer(gSrv.Server, s.Notifier)
		
		if err := gSrv.Start(); err != nil {
			s.Logger.Error("gRPC server failed: %v", err)
		}
	}()

	// 3. Start TCP Server
	s.serverSock, err = factory.Create("tcp-hello", tcpAddr, "127.0.0.1", "server", true)
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
				s.Logger.Error("Accept error: " + err.Error())
				continue
			}
		}
		go s.handleConnection(conn)
	}
}

// -----------------------------------------------------------------------------
