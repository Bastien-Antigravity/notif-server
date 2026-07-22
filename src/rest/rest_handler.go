package rest

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	notif_core "github.com/Bastien-Antigravity/notif-server/src/core"
	"github.com/Bastien-Antigravity/notif-server/src/grpc_control"
	unilog_interfaces "github.com/Bastien-Antigravity/universal-logger/src/interfaces"
)

//go:embed mfe.js
var mfeJS string

// -----------------------------------------------------------------------------
// RESTHandler handles HTTP management requests
// -----------------------------------------------------------------------------

type RESTHandler struct {
	logger   unilog_interfaces.Logger
	control  *grpc_control.ControlServiceImpl
}

// -----------------------------------------------------------------------------

// NewRESTHandler creates a new RESTHandler instance
func NewRESTHandler(controller notif_core.NotifController, logger unilog_interfaces.Logger) *RESTHandler {
	return &RESTHandler{
		logger:   logger,
		control:  grpc_control.NewControlService(controller, logger),
	}
}

// -----------------------------------------------------------------------------

// RegisterRoutes registers the REST routes to the provided mux
func (h *RESTHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/v1/notifiers/list", h.handleListNotifiers)
	mux.HandleFunc("/api/v1/notif/test", h.handleSendTest)
	mux.HandleFunc("/api/v1/config/reload", h.handleReloadConfig)
	mux.HandleFunc("/api/v1/status", h.handleStatus)
	mux.HandleFunc("/api/v1/config/alerting/get", h.handleGetAlertingConfig)
	mux.HandleFunc("/api/v1/config/alerting/set", h.handleSetAlertingConfig)
	mux.HandleFunc("/api/v1/config/alerting/add", h.handleAddProvider)
	mux.HandleFunc("/api/v1/config/alerting/remove", h.handleRemoveProvider)
	mux.HandleFunc("/api/v1/config/alerting/types", h.handleGetSupportedTypes)

	// Serve embedded static files (OpenMFE Web Component bundles)
	mfeHandler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/javascript")
		w.Write([]byte(mfeJS))
	}
	mux.HandleFunc("/static/js/mfe-loader.js", mfeHandler)
	mux.HandleFunc("/static/mfe.js", mfeHandler)
}

// -----------------------------------------------------------------------------

func (h *RESTHandler) handleListNotifiers(w http.ResponseWriter, r *http.Request) {
	resp, _ := h.control.ListNotifiers(r.Context(), &grpc_control.ListNotifiersRequest{})
	h.sendJSON(w, resp)
}

func (h *RESTHandler) handleSendTest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req grpc_control.SendTestNotificationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	resp, _ := h.control.SendTestNotification(r.Context(), &req)
	h.sendJSON(w, resp)
}

func (h *RESTHandler) handleReloadConfig(w http.ResponseWriter, r *http.Request) {
	resp, _ := h.control.ReloadConfig(r.Context(), &grpc_control.ReloadConfigRequest{})
	h.sendJSON(w, resp)
}

func (h *RESTHandler) handleStatus(w http.ResponseWriter, r *http.Request) {
	resp, _ := h.control.GetStatus(r.Context(), &grpc_control.GetStatusRequest{})
	h.sendJSON(w, resp)
}

func (h *RESTHandler) handleGetAlertingConfig(w http.ResponseWriter, r *http.Request) {
	resp, _ := h.control.GetAlertingConfig(r.Context(), &grpc_control.GetAlertingConfigRequest{})
	h.sendJSON(w, resp)
}

func (h *RESTHandler) handleSetAlertingConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req grpc_control.SetAlertingConfigRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	resp, _ := h.control.SetAlertingConfig(r.Context(), &req)
	h.sendJSON(w, resp)
}

func (h *RESTHandler) handleAddProvider(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req grpc_control.AddProviderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	resp, _ := h.control.AddProvider(r.Context(), &req)
	h.sendJSON(w, resp)
}

func (h *RESTHandler) handleRemoveProvider(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req grpc_control.RemoveProviderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	resp, _ := h.control.RemoveProvider(r.Context(), &req)
	h.sendJSON(w, resp)
}

func (h *RESTHandler) handleGetSupportedTypes(w http.ResponseWriter, r *http.Request) {
	resp, _ := h.control.GetSupportedTypes(r.Context(), &grpc_control.GetSupportedTypesRequest{})
	h.sendJSON(w, resp)
}

// -----------------------------------------------------------------------------

func (h *RESTHandler) sendJSON(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

// -----------------------------------------------------------------------------

// Handler returns the REST API handler with route registration and CORS wrapping.
func (h *RESTHandler) Handler() http.Handler {
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS, PUT, DELETE")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}
		mux.ServeHTTP(w, r)
	})
}

// StartServer starts a simple HTTP server for the REST API
func (h *RESTHandler) StartServer(port int) error {
	addr := fmt.Sprintf(":%d", port)
	h.logger.Info("Starting REST management server on %s", addr)

	server := &http.Server{
		Addr:         addr,
		Handler:      h.Handler(),
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	return server.ListenAndServe()
}
