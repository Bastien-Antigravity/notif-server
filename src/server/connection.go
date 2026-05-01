package server

import (
	"fmt"
	"io"

	"github.com/Bastien-Antigravity/safe-socket"
	socket_interfaces "github.com/Bastien-Antigravity/safe-socket/src/interfaces"
)

// -----------------------------------------------------------------------------
func (s *Server) handleConnection(sock socket_interfaces.TransportConnection) {
	defer sock.Close()

	// 1. Extract Client Identity from Handshake (Peeling wrappers if needed)
	identity := safesocket.GetIdentity(sock)
	if identity == nil {
		s.Logger.Error("Connection does not have a Handshake identity")
		return
	}

	name, _ := identity.FromName()
	address, _ := identity.FromAddress()
	clientName := fmt.Sprintf("%s-%s", name, address)

	s.Logger.Info(fmt.Sprintf("Client identified: %s", clientName))

	// Disable all timeouts to allow the connection to remain open forever.
	_ = sock.SetIdleTimeout(0)

	// 2. Message Loop
	// Allocation Optimization: Reuse buffer
	// Start with 64KB (typical max UDP, reasonable for TCP config messages)
	buf := make([]byte, 65535)

	for {
		// No deadline set here, allowing infinite wait on Read.

		// Use Read(buf) instead of ReadMessage to reuse buffer
		n, err := sock.Read(buf)
		if err != nil {
			if err == io.ErrShortBuffer {
				// Buffer too small. Resize double and retry.
				// Note: FramedTCP uses Peek, so the header is still there. We can safely retry.
				// Safety check: Limit max size to avoid OOM (e.g. 10MB)
				if len(buf) >= 10*1024*1024 {
					s.Logger.Error(fmt.Sprintf("Message too large from %s", clientName))
					return
				}
				newSize := len(buf) * 2
				s.Logger.Info(fmt.Sprintf("Resizing read buffer for %s to %d bytes", clientName, newSize))
				buf = make([]byte, newSize)
				continue
			}

			if err != io.EOF {
				s.Logger.Error(fmt.Sprintf("Read error from %s: %v", clientName, err))
			}
			return
		}

		// Handle NotifMsg via Cap'n Proto (Raw forwarding)
		// We copy the buffer because 'buf' is reused in the loop.
		rawMsg := make([]byte, n)
		copy(rawMsg, buf[:n])

		// Send to Notifier raw channel
		s.Notifier.RawNotifChan <- rawMsg
	}
}
