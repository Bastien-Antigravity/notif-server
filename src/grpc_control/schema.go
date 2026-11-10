package grpc_control

import _ "embed"

// NotifServerProtoSchema embeds the canonical NotifControlService gRPC definition.
//
//go:embed notif_server.proto
var NotifServerProtoSchema string

