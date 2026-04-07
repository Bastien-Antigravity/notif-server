package server

import (
	"fmt"
	"os"
	"sync"

	notifie "github.com/Bastien-Antigravity/notif-server/src/core"

	factory "github.com/Bastien-Antigravity/safe-socket"
	socket_interfaces "github.com/Bastien-Antigravity/safe-socket/src/interfaces"
	"github.com/Bastien-Antigravity/universal-logger/src/config"
	"github.com/Bastien-Antigravity/universal-logger/src/logger"
)

type Server struct {
	Logger        *logger.UniLog
	Config        *config.DistConfig
	Notifie       *notifie.Notifie
	listeners     map[string]socket_interfaces.TransportConnection
	listenersLock sync.RWMutex
	shutdown      chan struct{}
	serverSock    socket_interfaces.Socket // Store the listener socket
}

// -----------------------------------------------------------------------------

// NewServer creates a new Config Server.
func NewServer(conf *config.DistConfig, logger *logger.UniLog, notif *notifie.Notifie) *Server {
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

// Start listens for incoming TCP connections.
func (s *Server) Start() error {
	// Resolve address from config capabilities using the new generic map-based approach
	cap, ok := s.Config.Capabilities["NotifServer"].(map[string]interface{})
	if !ok || cap["IP"] == nil || cap["Port"] == nil {
		s.Logger.Error("Config for NotifServer capabilities not found or invalid in generic map")
		os.Exit(1)
	}

	addr := fmt.Sprintf("%v:%v", cap["IP"], cap["Port"])

	// Create a server socket using safe-socket factory
	// We use "tcp-hello" profile which automatically handles the Handshake
	var err error
	s.serverSock, err = factory.Create("tcp-hello", addr, "127.0.0.1", "server", true)
	if err != nil {
		return err // Wrap error in caller if needed, or return raw err
	}
	defer s.serverSock.Close()

	s.Logger.Info("Notification Server listening on " + addr)

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
