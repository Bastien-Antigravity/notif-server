package server

/*
ESSENTIAL PROCESS:
Handles individual client connections and stable identity resolution.
Implements the persistent message loop for TCP ingestion.

DATA FLOW:
1. Extract client identity and resolve stable hostname.
2. Enforce idle timeouts to prune zombie connections.
3. Read framed messages using ReadMessage().
4. Forward raw binary data to the Notifier core.

KEY PARAMETERS:
- sock: The active transport connection from safe-socket.
- clientName: Resolved stable identity of the connected source.
*/

import (
	"fmt"
	"io"
	"net"
	"time"

	"github.com/Bastien-Antigravity/safe-socket"
	socket_interfaces "github.com/Bastien-Antigravity/safe-socket/src/interfaces"
)

// -----------------------------------------------------------------------------

func (s *Server) handleConnection(sock socket_interfaces.TransportConnection) {
	defer sock.Close()

	// 1. Extract Client Identity from Handshake
	identity := safesocket.GetIdentity(sock)
	if identity == nil {
		s.Logger.Error("Connection does not have a Handshake identity")
		return
	}

	name, _ := identity.FromName()
	address, _ := identity.FromAddress()

	// Stable Identity Resolution: Strip port from address if present
	host, _, err := net.SplitHostPort(address)
	if err == nil {
		address = host
	}
	clientName := fmt.Sprintf("%s-%s", name, address)

	s.Logger.Info(fmt.Sprintf("Client identified: %s", clientName))

	// Set a reasonable idle timeout to clean up zombie connections.
	_ = sock.SetIdleTimeout(10 * time.Minute)

	// 2. Message Loop
	// Using ReadMessage() which is provided by safe-socket for robust framing.
	for {
		data, err := sock.ReadMessage()
		if err != nil {
			if err != io.EOF {
				s.Logger.Error(fmt.Sprintf("Read error from %s: %v", clientName, err))
			}
			return
		}

		// Handle NotifMsg via Cap'n Proto (Raw forwarding)
		// Send to Notifier raw channel
		s.Notifier.RawNotifChan <- data
	}
}
