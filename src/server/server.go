package server

import (
	"os"
	"sync"

	notifie "notif-server/src/core"
	"notif-server/src/interfaces"

	distributed_config "github.com/Bastien-Antigravity/distributed-config"
	factory "github.com/Bastien-Antigravity/safe-socket"
	socket_interfaces "github.com/Bastien-Antigravity/safe-socket/src/interfaces"
)

// Server represents the Config Server.
type Server struct {
	Logger        interfaces.Logger
	Config        *distributed_config.Config
	Notifie       *notifie.Notifie
	listeners     map[string]socket_interfaces.TransportConnection
	listenersLock sync.RWMutex
	shutdown      chan struct{}
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

// Start listens for incoming TCP connections.
func (s *Server) Start() error {
	// Resolve address from config capabilities
	if s.Config.Capabilities.Notification == nil || s.Config.Capabilities.Notification.Port == "" || s.Config.Capabilities.Notification.IP == "" {
		s.Logger.Error("Config for Notification capabilities not found or invalid")
		os.Exit(1)
	}

	addr := s.Config.Capabilities.Notification.IP + ":" + s.Config.Capabilities.Notification.Port

	// Create a server socket using safe-socket factory
	// We use "tcp-hello" profile which automatically handles the Handshake
	serverSock, err := factory.Create("tcp-hello", addr, "127.0.0.1", "server", true)
	if err != nil {
		return err // Wrap error in caller if needed, or return raw err
	}
	defer serverSock.Close()

	s.Logger.Info("Notification Server listening on " + addr)

	for {
		conn, err := serverSock.Accept()
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
