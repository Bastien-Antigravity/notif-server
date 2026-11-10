package rest

/*
ESSENTIAL PROCESS:
Constructs exhaustive, microservice-authored Exchange Contracts for notif-server
across HTTP/REST, gRPC (NotifService + NotifControlService), SafeSocket TCP with
Cap'n Proto (NotifierMsg), and outbound Tele-Remote / Alerting providers.

DATA FLOW:
1. Embeds notif_service.proto, notif_server.proto, and notifier.capnp directly.
2. Enriches contracts with live active notifiers and supported provider types.
3. Serves GET /api/v1/contracts and populates ServiceSpec.Contracts at boot.

KEY PARAMETERS:
- appConfig: Optional AppConfig resolving TCP/gRPC/REST ports dynamically.
- notifiers: Active runtime dispatcher statuses.
- supportedTypes: Supported provider types reported by controller.
*/

import (
	"net/http"
	"strings"

	toolbox_config "github.com/Bastien-Antigravity/microservice-toolbox/go/pkg/config"
	toolbox_spec "github.com/Bastien-Antigravity/microservice-toolbox/go/pkg/spec"
	notif_core "github.com/Bastien-Antigravity/notif-server/src/core"
	"github.com/Bastien-Antigravity/notif-server/src/grpc_control"
	capnp_notifier "github.com/Bastien-Antigravity/notif-server/src/schemas/capnp"
	notif_proto "github.com/Bastien-Antigravity/notif-server/src/schemas/protobuf"
)

// -----------------------------------------------------------------------------

// BuildExchangeContracts constructs the exhaustive contract catalog from notif-server sources.
func BuildExchangeContracts(
	appConfig *toolbox_config.AppConfig,
	notifiers []*grpc_control.NotifierInfo,
	supportedTypes []string,
) []toolbox_spec.ExchangeContract {
	var tcpAddr, grpcAddr, restAddr, teleAddr string
	if appConfig != nil {
		tcpAddr, _ = appConfig.GetListenAddr("notif_server")
		grpcAddr, _ = appConfig.GetGRPCListenAddr("notif_server")
		restAddr, _ = appConfig.GetRESTAddr("notif_server")
		teleAddr, _ = appConfig.GetGRPCListenAddr("tele_remote")
		if teleAddr == "" {
			teleAddr, _ = appConfig.GetListenAddr("tele_remote")
		}
	}

	grpcOps := toolbox_spec.ExtractProtoTextOperations(notif_proto.NotifServiceProtoSchema)
	grpcOps = append(grpcOps, toolbox_spec.ExtractProtoTextOperations(grpc_control.NotifServerProtoSchema)...)

	capnpOps := toolbox_spec.ExtractCapnpOperations(capnp_notifier.NotifierCapnpSchema)
	capnpOps = append([]toolbox_spec.ContractOperation{
		{
			Name:        "HelloHandshake",
			Kind:        "FRAME",
			Target:      "SafeSocket HelloMsg (tcp-hello)",
			Input:       "fromName, fromHost, fromAddress, toAddress, fromPublicIP",
			Description: "Mandatory unpacked Cap'n Proto handshake identifying source host/service",
		},
	}, capnpOps...)

	combinedProto := strings.TrimSpace(notif_proto.NotifServiceProtoSchema) + "\n\n// -----------------------------------------------------------------------------\n\n" + strings.TrimSpace(grpc_control.NotifServerProtoSchema)

	var providerOps []toolbox_spec.ContractOperation
	providerOps = append(providerOps, toolbox_spec.ContractOperation{
		Name:        "TeleRemoteMenu",
		Kind:        "BIDI_STREAM",
		Target:      "teleremote.TeleRemoteService/Connect",
		Input:       "ComponentMessage (\"Notification Server\")",
		Output:      "SendTestNotification / ReloadSenders / Alerting Config Editor",
		Description: "Interactive Telegram remote-control menu pushed on config changes",
	})
	for _, pType := range supportedTypes {
		providerOps = append(providerOps, toolbox_spec.ContractOperation{
			Name:        pType,
			Kind:        "OUTBOUND",
			Target:      "provider://" + strings.ToLower(pType),
			Input:       "NotifMessage { message, level, tags, attachment }",
			Output:      "Delivery status / retry telemetry",
			Description: "Supported outbound alerting backend dispatcher",
		})
	}
	for _, n := range notifiers {
		statusLabel := "unhealthy"
		if n.Healthy {
			statusLabel = "healthy"
		}
		providerOps = append(providerOps, toolbox_spec.ContractOperation{
			Name:        n.Name,
			Kind:        "WORKER",
			Target:      "provider://" + strings.ToLower(n.Type) + "/" + n.Name,
			Output:      statusLabel,
			Description: "Configured active notifier worker instance",
		})
	}

	return []toolbox_spec.ExchangeContract{
		{
			ID:        "rest",
			Type:      "HTTP/REST API",
			TabLabel:  "HTTP/REST API",
			Icon:      "fa-globe",
			Direction: "inbound",
			Transport: "HTTP/1.1 + CORS JSON",
			Address:   toolbox_spec.DisplayAddress(restAddr),
			Port:      toolbox_spec.ResolveContractPort(restAddr),
			Framing:   "HTTP JSON request/response with CORS headers",
			Auth:      "Internal mesh REST management",
			Summary:   "Manages active alerting workers, dispatches test notifications, updates alerting configuration, and serves <notif-server-mfe>.",
			Source:    "notif-server/src/rest/rest_handler.go",
			Peers: []toolbox_spec.ContractPeer{
				{Service: "web_interface", Role: "consumer", Description: "Hosts <notif-server-mfe> and dispatches alerting configuration REST calls"},
			},
			Operations: []toolbox_spec.ContractOperation{
				{Name: "Status", Kind: "GET", Target: "/api/v1/status", Output: "{ healthy, status, version, timestamp, details }", Description: "Live server status & worker health summary"},
				{Name: "ListNotifiers", Kind: "GET", Target: "/api/v1/notifiers/list", Output: "{ success, notifiers[], timestamp }", Description: "Lists configured alerting senders and health"},
				{Name: "SendTest", Kind: "POST", Target: "/api/v1/notif/test", Input: "{ level, title, message }", Output: "ControlResponse JSON", Description: "Dispatches test alert across routed providers"},
				{Name: "ReloadConfig", Kind: "POST", Target: "/api/v1/config/reload", Output: "ControlResponse JSON", Description: "Reloads alerting configuration & reinitializes senders"},
				{Name: "GetAlertingConfig", Kind: "GET", Target: "/api/v1/config/alerting/get", Output: "{ success, config, timestamp }", Description: "Returns provider config with masked secrets"},
				{Name: "SetAlertingConfig", Kind: "POST", Target: "/api/v1/config/alerting/set", Input: "{ platform, key, value }", Output: "ControlResponse JSON", Description: "Updates a provider setting dynamically"},
				{Name: "AddProvider", Kind: "POST", Target: "/api/v1/config/alerting/add", Input: "{ tag, type }", Output: "ControlResponse JSON", Description: "Adds a new alerting provider section"},
				{Name: "RemoveProvider", Kind: "POST", Target: "/api/v1/config/alerting/remove", Input: "{ tag }", Output: "ControlResponse JSON", Description: "Removes an alerting provider configuration"},
				{Name: "SupportedTypes", Kind: "GET", Target: "/api/v1/config/alerting/types", Output: "{ success, types[] }", Description: "Returns supported alerting backend types"},
				{Name: "Contracts", Kind: "GET", Target: "/api/v1/contracts", Output: "ContractsEnvelope JSON", Description: "Self-describing microservice exchange contracts"},
				{Name: "MFEScript", Kind: "GET", Target: "/static/mfe.js", Output: "application/javascript", Description: "Embedded OpenMFE <notif-server-mfe> custom element"},
			},
			SchemaFormat: "http",
			Schema: strings.Join([]string{
				"GET  /api/v1/status",
				"GET  /api/v1/contracts",
				"GET  /api/v1/notifiers/list",
				"POST /api/v1/notif/test            body: {\"level\", \"title\", \"message\"}",
				"POST /api/v1/config/reload",
				"GET  /api/v1/config/alerting/get",
				"POST /api/v1/config/alerting/set     body: {\"platform\", \"key\", \"value\"}",
				"POST /api/v1/config/alerting/add     body: {\"tag\", \"type\"}",
				"POST /api/v1/config/alerting/remove  body: {\"tag\"}",
				"GET  /api/v1/config/alerting/types",
				"GET  /static/mfe.js                  (alias: /static/js/mfe-loader.js)",
			}, "\n"),
		},
		{
			ID:        "grpc",
			Type:      "gRPC / Protobuf",
			TabLabel:  "gRPC Protobuf",
			Icon:      "fa-code",
			Direction: "inbound",
			Transport: "HTTP/2 + Protobuf",
			Address:   toolbox_spec.DisplayAddress(grpcAddr),
			Port:      toolbox_spec.ResolveContractPort(grpcAddr),
			Framing:   "Length-prefixed HTTP/2 DATA frames (NotifService & NotifControlService)",
			Auth:      "Internal gRPC insecure/TLS transport",
			Summary:   "Dual gRPC services on the same listener: high-speed NotifService alert ingestion plus NotifControlService management RPCs.",
			Source:    "notif-server/src/schemas/protobuf/notif_service.proto + grpc_control/notif_server.proto (go:embed)",
			Peers: []toolbox_spec.ContractPeer{
				{Service: "watchdog_agent", Role: "client", Description: "Critical supervisor failure alerts & recovery events"},
				{Service: "universal-logger / fleet clients", Role: "client", Description: "Dispatches CRITICAL/TRADE/LOGON notifications over gRPC"},
				{Service: "web_interface", Role: "consumer", Description: "Control & health inspection via NotifControlService"},
			},
			Operations:   grpcOps,
			SchemaFormat: "proto3",
			Schema:       combinedProto,
		},
		{
			ID:        "safesocket-capnp",
			Type:      "SafeSocket TCP + Cap'n Proto",
			TabLabel:  "Cap'n Proto TCP",
			Icon:      "fa-file-code-o",
			Direction: "inbound",
			Transport: "TCP + SafeSocket Framing + Packed Cap'n Proto",
			Address:   toolbox_spec.DisplayAddress(tcpAddr),
			Port:      toolbox_spec.ResolveContractPort(tcpAddr),
			Framing:   "4-byte BE length header + HelloMsg Cap'n Proto handshake + packed NotifierMsg",
			Auth:      "SafeSocket tcp-hello handshake identity",
			Summary:   "Persistent zero-copy Cap'n Proto ingestion stream routing incoming alerts into worker pools by explicit tag or log-level fallback.",
			Source:    "notif-server/src/schemas/capnp/notifier.capnp (go:embed) + server/connection.go",
			Peers: []toolbox_spec.ContractPeer{
				{Service: "microservice-toolbox / safe-socket clients", Role: "producer", Description: "Streams serialized NotifierMsg frames over persistent TCP sockets"},
			},
			Operations:   capnpOps,
			SchemaFormat: "capnp",
			Schema:       capnp_notifier.NotifierCapnpSchema,
		},
		{
			ID:        "teleremote-providers",
			Type:      "Tele-Remote & Outbound Alerting",
			TabLabel:  "Outbound Sinks & Bot",
			Icon:      "fa-paper-plane",
			Direction: "outbound",
			Transport: "gRPC Stream + Provider HTTPS/SMTP APIs",
			Address:   toolbox_spec.DisplayAddress(teleAddr),
			Port:      toolbox_spec.ResolveContractPort(teleAddr),
			Framing:   "teleremote.TeleRemoteService/Connect + HTTPS Webhook/Bot JSON / SMTP",
			Auth:      "Encrypted tokens from AppConfig DecryptSecret",
			Summary:   "Connects to Tele-Remote for interactive menu control and dispatches formatted alerts to configured notification providers.",
			Source:    "notif-server/src/telegram/manager.go + notif-server/src/notifiers/",
			Peers: []toolbox_spec.ContractPeer{
				{Service: "tele_remote", Role: "server", Description: "Interactive bot control menus for sending tests and editing providers"},
				{Service: "External Alerting Gateways", Role: "external_sink", Description: "Telegram Bot API, Discord Webhooks, Gmail SMTP, Matrix Client API"},
			},
			Operations:   providerOps,
			SchemaFormat: "wire",
			Schema:       "Outbound Dispatch Adapters:\n- Telegram Bot API (bot token + chat_id)\n- Discord Webhook URL (HTTP POST embed payload)\n- Gmail SMTP Relay (TLS STARTTLS)\n- Matrix Homeserver API (PUT _matrix/client/v3/rooms/{roomId}/send/m.room.message)",
		},
	}
}

// -----------------------------------------------------------------------------

func (h *RESTHandler) handleContracts(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var notifiers []*grpc_control.NotifierInfo
	if listResp, err := h.control.ListNotifiers(r.Context(), &grpc_control.ListNotifiersRequest{}); err == nil && listResp != nil {
		notifiers = listResp.Notifiers
	}

	var supportedTypes []string
	if typesResp, err := h.control.GetSupportedTypes(r.Context(), &grpc_control.GetSupportedTypesRequest{}); err == nil && typesResp != nil {
		supportedTypes = typesResp.Types
	}

	version := notif_core.ServerVersion
	if statusResp, err := h.control.GetStatus(r.Context(), &grpc_control.GetStatusRequest{}); err == nil && statusResp != nil && statusResp.Version != "" {
		version = statusResp.Version
	}

	contracts := BuildExchangeContracts(h.appConfig, notifiers, supportedTypes)
	h.sendJSON(w, toolbox_spec.NewContractsEnvelope("notif_server", version, "notif-server (live controller + embedded schemas)", contracts))
}

