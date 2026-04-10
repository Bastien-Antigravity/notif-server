package server

import (
	"fmt"
	"os"
	"strings"
	"sync"

	notifie "github.com/Bastien-Antigravity/notif-server/src/core"
	pb "github.com/Bastien-Antigravity/notif-server/src/schemas/notif_msg"

	factory "github.com/Bastien-Antigravity/safe-socket"
	socket_interfaces "github.com/Bastien-Antigravity/safe-socket/src/interfaces"
	distributed_config "github.com/Bastien-Antigravity/distributed-config"
	toolbox_config "github.com/Bastien-Antigravity/microservice-toolbox/go/pkg/config"
	"github.com/Bastien-Antigravity/microservice-toolbox/go/pkg/network"
	"github.com/Bastien-Antigravity/universal-logger/src/interfaces"
)

type Server struct {
	Logger        interfaces.Logger
	Config        *distributed_config.Config
	Notifie       *notifie.Notifie
	listeners     map[string]socket_interfaces.TransportConnection
	listenersLock sync.RWMutex
	shutdown      chan struct{}
	serverSock    socket_interfaces.Socket // Store the listener socket
}

// -----------------------------------------------------------------------------

// NewServer creates a new Config Server.
func NewServer(conf *distributed_config.Config, logger interfaces.Logger, notif *notifie.Notifie) *Server {
	return &Server{
		Config:    conf,
		Logger:    logger,
		Notifie:   notif,
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
	// 1. Resolve TCP address from config
	cap, ok := s.Config.Capabilities["notif_server"].(map[string]interface{})
	if !ok || cap["ip"] == nil || cap["port"] == nil {
		s.Logger.Error("Config for notif-server capabilities not found or invalid in generic map")
		os.Exit(1)
	}

	ip := strings.Trim(fmt.Sprintf("%v", cap["ip"]), "\"")
	port := strings.Trim(fmt.Sprintf("%v", cap["port"]), "\"")
	tcpAddr := fmt.Sprintf("%s:%s", ip, port)

	// 2. Start gRPC Server in background
	go func() {
		// Use toolbox convention for gRPC port
		ac := &toolbox_config.AppConfig{Config: s.Config}
		grpcAddr, err := ac.GetGRPCListenAddr("notif_server")
		if err != nil {
			s.Logger.Error("Failed to resolve gRPC address: %v", err)
			return
		}

		s.Logger.Info("Notification Server gRPC listening on " + grpcAddr)
		gSrv := network.NewGRPCServer(grpcAddr)
		pb.RegisterNotifServiceServer(gSrv.Server, s.Notifie)
		
		if err := gSrv.Start(); err != nil {
			s.Logger.Error("gRPC server failed: %v", err)
		}
	}()

	// 3. Start TCP Server
	var err error
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
