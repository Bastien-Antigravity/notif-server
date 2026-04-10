package server

import (
	"fmt"
	"os"
	"strings"
	"sync"

	notifie "github.com/Bastien-Antigravity/notif-server/src/core"

	factory "github.com/Bastien-Antigravity/safe-socket"
	socket_interfaces "github.com/Bastien-Antigravity/safe-socket/src/interfaces"
	distconf "github.com/Bastien-Antigravity/distributed-config"
	"github.com/Bastien-Antigravity/universal-logger/src/interfaces"
)

type Server struct {
	Logger        interfaces.Logger
	Config        *distconf.Config
	Notifie       *notifie.Notifie
	listeners     map[string]socket_interfaces.TransportConnection
	listenersLock sync.RWMutex
	shutdown      chan struct{}
	serverSock    socket_interfaces.Socket // Store the listener socket
}

// -----------------------------------------------------------------------------

// NewServer creates a new Config Server.
func NewServer(conf *distconf.Config, logger interfaces.Logger, notif *notifie.Notifie) *Server {
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
	cap, ok := s.Config.Capabilities["notif_server"].(map[string]interface{})
	if !ok || cap["ip"] == nil || cap["port"] == nil {
		s.Logger.Error("Config for notif-server capabilities not found or invalid in generic map")
		os.Exit(1)
	}

	ip := strings.Trim(fmt.Sprintf("%v", cap["ip"]), "\"")
	port := strings.Trim(fmt.Sprintf("%v", cap["port"]), "\"")
	addr := fmt.Sprintf("%s:%s", ip, port)

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
