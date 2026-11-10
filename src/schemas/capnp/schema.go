package capnp_notifier

import _ "embed"

// NotifierCapnpSchema embeds the canonical Cap'n Proto binary schema for SafeSocket ingestion.
//
//go:embed notifier.capnp
var NotifierCapnpSchema string

