package main

// -----------------------------------------------------------------------------
// ESSENTIAL PROCESS:
// CGO shared library bridge exporting C ABI functions for dispatching direct,
// multi-channel notifications to notif-server over gRPC (Port 1027).
//
// DATA FLOW:
// Polyglot Caller (C++, VBA, Python, Rust) -> C ABI -> NotifService.SendNotification -> notif-server
//
// KEY PARAMETERS:
// - targetAddr: Host:port string of notif-server gRPC listener (canonical: 1027).
// - level: Alert severity level (TRADE, CRITICAL, WARNING, REPORT, INFO).
// - message: Notification body content.
// - attachment: Optional path or URI to attached asset.
// - tagsCSV: Comma-separated destination routing tags (e.g. "telegram,discord").
// - timeoutSec: RPC timeout in seconds.
// -----------------------------------------------------------------------------

/*
#include <stdlib.h>
*/
import "C"

import (
	"context"
	"strings"
	"sync"
	"time"
	"unsafe"

	pb "github.com/Bastien-Antigravity/notif-server/src/schemas/protobuf"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// -----------------------------------------------------------------------------

var (
	connMu  sync.Mutex
	conns   = make(map[string]*grpc.ClientConn)
	clients = make(map[string]pb.NotifServiceClient)

	errMu     sync.RWMutex
	lastError string
)

func setLastError(err string) {
	errMu.Lock()
	lastError = err
	errMu.Unlock()
}

func getOrInitClient(targetAddr string) (pb.NotifServiceClient, error) {
	connMu.Lock()
	defer connMu.Unlock()

	if client, ok := clients[targetAddr]; ok {
		return client, nil
	}

	conn, err := grpc.NewClient(
		targetAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, err
	}

	conns[targetAddr] = conn
	client := pb.NewNotifServiceClient(conn)
	clients[targetAddr] = client
	return client, nil
}

// -----------------------------------------------------------------------------

//export Notif_Send
func Notif_Send(targetAddr, level, message, attachment, tagsCSV *C.char) C.int {
	return Notif_SendWithTimeout(targetAddr, level, message, attachment, tagsCSV, 5.0)
}

// -----------------------------------------------------------------------------

//export Notif_SendWithTimeout
func Notif_SendWithTimeout(targetAddr, level, message, attachment, tagsCSV *C.char, timeoutSec C.double) C.int {
	target := C.GoString(targetAddr)
	if target == "" {
		setLastError("targetAddr cannot be empty")
		return 1
	}

	client, err := getOrInitClient(target)
	if err != nil {
		setLastError("failed to connect to " + target + ": " + err.Error())
		return 2
	}

	sec := float64(timeoutSec)
	if sec <= 0 {
		sec = 5.0
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(sec*float64(time.Second)))
	defer cancel()

	var tags []string
	if tagsCSV != nil {
		rawTags := C.GoString(tagsCSV)
		if rawTags != "" {
			for _, t := range strings.Split(rawTags, ",") {
				t = strings.TrimSpace(t)
				if t != "" {
					tags = append(tags, t)
				}
			}
		}
	}

	req := &pb.NotifRequest{
		Level:      C.GoString(level),
		Message:    C.GoString(message),
		Attachment: C.GoString(attachment),
		Tags:       tags,
	}

	resp, err := client.SendNotification(ctx, req)
	if err != nil {
		setLastError("rpc SendNotification failed: " + err.Error())
		return 3
	}

	if !resp.Success {
		setLastError("notification server rejected message")
		return 4
	}

	setLastError("")
	return 0
}

// -----------------------------------------------------------------------------

//export Notif_GetLastError
func Notif_GetLastError() *C.char {
	errMu.RLock()
	defer errMu.RUnlock()
	if lastError == "" {
		return nil
	}
	return C.CString(lastError)
}

// -----------------------------------------------------------------------------

//export Notif_FreeString
func Notif_FreeString(ptr *C.char) {
	if ptr != nil {
		C.free(unsafe.Pointer(ptr))
	}
}

// -----------------------------------------------------------------------------

//export Notif_Close
func Notif_Close() {
	connMu.Lock()
	defer connMu.Unlock()
	for addr, conn := range conns {
		if conn != nil {
			_ = conn.Close()
		}
		delete(conns, addr)
		delete(clients, addr)
	}
	setLastError("")
}

// -----------------------------------------------------------------------------

func main() {}
