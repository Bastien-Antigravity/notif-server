package protobuf

import _ "embed"

// NotifServiceProtoSchema embeds the canonical NotifService gRPC ingestion definition.
//
//go:embed notif_service.proto
var NotifServiceProtoSchema string

