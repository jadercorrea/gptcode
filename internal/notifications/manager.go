package notifications

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/jadercorrea/gptcode/internal/config"
	"github.com/jadercorrea/gptcode/internal/live"
)

type BlockedNotification struct {
	ID         string    `json:"id"`
	Type       string    `json:"type"` // "blocked"
	Action     string    `json:"action"`
	Language   string    `json:"language"`
	Model      string    `json:"model,omitempty"`
	Failed     []string  `json:"failed_models"`
	Message    string    `json:"message"`
	Timestamp  time.Time `json:"timestamp"`
	Suggestion string    `json:"suggestion,omitempty"`
}

// Manager handles notifications
type Manager struct {
	setup         *config.Setup
	notifications []BlockedNotification
	liveClient    *live.Client
}

// NewManager creates a new notification manager
func NewManager(setup *config.Setup) *Manager {
	agentID := "local-cli"
	if hostname, err := os.Hostname(); err == nil {
		agentID = hostname
	}

	liveClient := live.NewClient(live.GetDashboardURL(), agentID)
	// Start the client asynchronously
	go func() {
		if err := liveClient.Connect(); err != nil {
			fmt.Fprintf(os.Stderr, "[Live] Failed to connect: %v\n", err)
		}
	}()

	// Make the client globally available for the tracer
	live.SetGlobalClient(liveClient)

	return &Manager{
		setup:         setup,
		notifications: []BlockedNotification{},
		liveClient:    liveClient,
	}
}

// SendBlockedNotification sends a notification when all models fail
func (m *Manager) SendBlockedNotification(ctx context.Context, action, language, model string, failedModels []string) error {
	if m.setup == nil || !m.setup.IsBlockedNotificationEnabled() {
		return nil
	}

	notification := BlockedNotification{
		ID:         fmt.Sprintf("blocked-%d", time.Now().Unix()),
		Type:       "blocked",
		Action:     action,
		Language:   language,
		Model:      model,
		Failed:     failedModels,
		Message:    m.setup.Notifications.Blocked.Message,
		Timestamp:  time.Now(),
		Suggestion: "Add a new approved model or check API credentials",
	}

	// Save notification to file for Live to pick up
	if err := m.saveNotification(notification); err != nil {
		fmt.Fprintf(os.Stderr, "[NOTIFICATIONS] Failed to save notification: %v\n", err)
	}

	// Check channels
	channels := m.setup.Notifications.Blocked.Channels
	for _, channel := range channels {
		switch channel {
		case "live":
			m.sendToLive(notification)
		case "telegram":
			m.sendToTelegram(notification)
		}
	}

	m.notifications = append(m.notifications, notification)
	return nil
}

// saveNotification saves notification to a file
func (m *Manager) saveNotification(n BlockedNotification) error {
	// Get notifications directory
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	notifyDir := filepath.Join(homeDir, ".gptcode", "notifications")
	if err := os.MkdirAll(notifyDir, 0755); err != nil {
		return err
	}

	filename := filepath.Join(notifyDir, fmt.Sprintf("%s.json", n.ID))
	data, err := json.MarshalIndent(n, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(filename, data, 0644)
}

func (m *Manager) sendToLive(n BlockedNotification) {
	// Original CLI print logs
	fmt.Printf("\n🔔 [BLOCKED] GT needs intervention!\n")
	fmt.Printf("   Action: %s\n", n.Action)
	fmt.Printf("   Language: %s\n", n.Language)
	fmt.Printf("   Failed models: %v\n", n.Failed)
	fmt.Printf("   Suggestion: %s\n", n.Suggestion)
	fmt.Printf("   Check Live dashboard for more details\n\n")

	// Now push directly to the Live Server via WebSocket
	if m.liveClient != nil {
		payload := map[string]interface{}{
			"data": n,
		}
		if err := m.liveClient.SendExecutionStep("error", "BLOCKED: "+n.Message, payload); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to stream notification to Live dashboard: %v\n", err)
		}
	}
}

// This is a placeholder - would need actual Telegram bot integration
func (m *Manager) sendToTelegram(n BlockedNotification) {
	// Would integrate with Telegram bot here
	// For now, just log
	fmt.Printf("[NOTIFICATIONS] Would send Telegram notification: %s\n", n.Message)
}

// GetNotifications returns all notifications
func (m *Manager) GetNotifications() []BlockedNotification {
	return m.notifications
}
