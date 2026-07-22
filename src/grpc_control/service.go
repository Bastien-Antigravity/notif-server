package grpc_control

import (
	"context"
	"fmt"
	"time"

	notif_core "github.com/Bastien-Antigravity/notif-server/src/core"
	unilog_interfaces "github.com/Bastien-Antigravity/universal-logger/src/interfaces"
)

// -----------------------------------------------------------------------------
// ControlService Implementation
// -----------------------------------------------------------------------------

type ControlServiceImpl struct {
	UnimplementedNotifControlServiceServer
	Name       string
	controller notif_core.NotifController
	logger     unilog_interfaces.Logger
}

// -----------------------------------------------------------------------------

// NewControlService creates a new ControlServiceImpl instance
func NewControlService(controller notif_core.NotifController, logger unilog_interfaces.Logger) *ControlServiceImpl {
	return &ControlServiceImpl{
		Name:       "GRPCControlService",
		controller: controller,
		logger:     logger,
	}
}

// -----------------------------------------------------------------------------
// Notifier Management
// -----------------------------------------------------------------------------

// ListNotifiers returns the list of configured notifiers
func (s *ControlServiceImpl) ListNotifiers(ctx context.Context, req *ListNotifiersRequest) (*ListNotifiersResponse, error) {
	s.logger.Debug("%s : received ListNotifiers request", s.Name)

	notifiers := s.controller.ListNotifiers()
	responseNotifiers := make([]*NotifierInfo, 0, len(notifiers))

	for _, n := range notifiers {
		responseNotifiers = append(responseNotifiers, &NotifierInfo{
			Name:    n.Name,
			Type:    n.Type,
			Healthy: n.Healthy,
		})
	}

	return &ListNotifiersResponse{
		Success:   true,
		Notifiers: responseNotifiers,
		Timestamp: time.Now().Unix(),
	}, nil
}

// SendTestNotification triggers a test notification through the engine
func (s *ControlServiceImpl) SendTestNotification(ctx context.Context, req *SendTestNotificationRequest) (*ControlResponse, error) {
	s.logger.Info("%s : received SendTestNotification request: [%s] %s", s.Name, req.Level, req.Title)

	err := s.controller.SendTestNotification(req.Level, req.Title, req.Message)
	if err != nil {
		return &ControlResponse{
			Success:   false,
			Message:   fmt.Sprintf("Failed to send test notification: %v", err),
			Timestamp: time.Now().Unix(),
			ErrorCode: "SEND_FAILED",
		}, nil
	}

	return &ControlResponse{
		Success:   true,
		Message:   "Test notification queued",
		Timestamp: time.Now().Unix(),
	}, nil
}

// ReloadConfig triggers a manual reload of the configuration
func (s *ControlServiceImpl) ReloadConfig(ctx context.Context, req *ReloadConfigRequest) (*ControlResponse, error) {
	s.logger.Info("%s : received ReloadConfig request", s.Name)

	if err := s.controller.ReloadConfig(); err != nil {
		return &ControlResponse{
			Success:   false,
			Message:   fmt.Sprintf("Failed to reload config: %v", err),
			Timestamp: time.Now().Unix(),
			ErrorCode: "RELOAD_FAILED",
		}, nil
	}

	return &ControlResponse{
		Success:   true,
		Message:   "Configuration reload triggered successfully",
		Timestamp: time.Now().Unix(),
	}, nil
}

// -----------------------------------------------------------------------------
// Status
// -----------------------------------------------------------------------------

// GetStatus returns server health and metadata
func (s *ControlServiceImpl) GetStatus(ctx context.Context, req *GetStatusRequest) (*GetStatusResponse, error) {
	s.logger.Debug("%s : received GetStatus request", s.Name)

	details, _ := s.controller.GetStatus()

	return &GetStatusResponse{
		Healthy:   true,
		Status:    "Operational",
		Version:   "0.1.1", // Standard platform versioning
		Timestamp: time.Now().Unix(),
		Details:   details,
	}, nil
}

// GetAlertingConfig returns the entire configuration state as a map of sections
func (s *ControlServiceImpl) GetAlertingConfig(ctx context.Context, req *GetAlertingConfigRequest) (*GetAlertingConfigResponse, error) {
	s.logger.Debug("%s : received GetAlertingConfig request", s.Name)

	config := s.controller.GetAlertingConfig()
	responseConfig := make(map[string]*AlertingConfigSection)

	for platform, settings := range config {
		responseConfig[platform] = &AlertingConfigSection{
			Settings: settings,
		}
	}

	return &GetAlertingConfigResponse{
		Success:   true,
		Config:    responseConfig,
		Timestamp: time.Now().Unix(),
	}, nil
}

// SetAlertingConfig updates a specific setting for a notification platform
func (s *ControlServiceImpl) SetAlertingConfig(ctx context.Context, req *SetAlertingConfigRequest) (*ControlResponse, error) {
	s.logger.Info("%s : received SetAlertingConfig request for [%s] %s = %s", s.Name, req.Platform, req.Key, req.Value)

	err := s.controller.SetAlertingConfig(req.Platform, req.Key, req.Value)
	if err != nil {
		return &ControlResponse{
			Success:   false,
			Message:   fmt.Sprintf("Failed to update alerting config: %v", err),
			Timestamp: time.Now().Unix(),
			ErrorCode: "UPDATE_FAILED",
		}, nil
	}

	return &ControlResponse{
		Success:   true,
		Message:   "Alerting configuration updated successfully",
		Timestamp: time.Now().Unix(),
	}, nil
}

// AddProvider initializes a new notification provider
func (s *ControlServiceImpl) AddProvider(ctx context.Context, req *AddProviderRequest) (*ControlResponse, error) {
	s.logger.Info("%s : received AddProvider request: [%s] %s", s.Name, req.Type, req.Tag)

	err := s.controller.AddProvider(req.Tag, req.Type)
	if err != nil {
		return &ControlResponse{
			Success:   false,
			Message:   fmt.Sprintf("Failed to add provider: %v", err),
			Timestamp: time.Now().Unix(),
		}, nil
	}

	return &ControlResponse{
		Success:   true,
		Message:   "Provider added successfully",
		Timestamp: time.Now().Unix(),
	}, nil
}

// RemoveProvider deletes a notification provider
func (s *ControlServiceImpl) RemoveProvider(ctx context.Context, req *RemoveProviderRequest) (*ControlResponse, error) {
	s.logger.Info("%s : received RemoveProvider request: %s", s.Name, req.Tag)

	err := s.controller.RemoveProvider(req.Tag)
	if err != nil {
		return &ControlResponse{
			Success:   false,
			Message:   fmt.Sprintf("Failed to remove provider: %v", err),
			Timestamp: time.Now().Unix(),
		}, nil
	}

	return &ControlResponse{
		Success:   true,
		Message:   "Provider removed successfully",
		Timestamp: time.Now().Unix(),
	}, nil
}

// GetSupportedTypes returns the list of available notification drivers
func (s *ControlServiceImpl) GetSupportedTypes(ctx context.Context, req *GetSupportedTypesRequest) (*GetSupportedTypesResponse, error) {
	s.logger.Debug("%s : received GetSupportedTypes request", s.Name)

	types := s.controller.GetSupportedTypes()

	return &GetSupportedTypesResponse{
		Success: true,
		Types:   types,
	}, nil
}
