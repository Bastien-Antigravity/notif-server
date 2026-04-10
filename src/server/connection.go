package server

import (
	"fmt"
	"io"
	"time"

	"github.com/Bastien-Antigravity/safe-socket/src/facade"
	socket_interfaces "github.com/Bastien-Antigravity/safe-socket/src/interfaces"
)

// -----------------------------------------------------------------------------
func (s *Server) findHandshakeConnection(sock socket_interfaces.TransportConnection) *facade.HandshakeConnection {
	if sock == nil {
		return nil
	}

	// Try direct type assertion
	if hc, ok := sock.(*facade.HandshakeConnection); ok {
		return hc
	}

	// If it's a HeartbeatConnection, look inside
	if hb, ok := sock.(*facade.HeartbeatConnection); ok {
		return s.findHandshakeConnection(hb.TransportConnection)
	}

	return nil
}

// -----------------------------------------------------------------------------
func (s *Server) handleConnection(sock socket_interfaces.TransportConnection) {
	defer sock.Close()

	// 1. Extract Client Identity from Handshake (Peeling wrappers if needed)
	hc := s.findHandshakeConnection(sock)
	if hc == nil || hc.Identity == nil {
		s.Logger.Error("Connection does not have a Handshake identity")
		return
	}

	name, _ := hc.Identity.FromName()
	address, _ := hc.Identity.FromAddress()
	clientName := fmt.Sprintf("%s-%s", name, address)

	s.Logger.Info(fmt.Sprintf("Client identified: %s", clientName))

	// 2. Message Loop
	// Allocation Optimization: Reuse buffer
	// Start with 64KB (typical max UDP, reasonable for TCP config messages)
	buf := make([]byte, 65535)

	for {
		// Refresh Deadline to prevent idle timeout (default 5s in safe-socket profiles)
		// We use 30s as a safe default for notification streams.
		_ = sock.SetReadDeadline(time.Now().Add(30 * time.Second))

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

		// Send to Notifie raw channel
		s.Notifie.RawNotifChan <- rawMsg
	}
}
