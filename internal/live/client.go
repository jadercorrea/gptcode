package live

import (
	crypto_rand "crypto/rand"
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/jadercorrea/gptcode/internal/crypto"

	"github.com/gorilla/websocket"
)

// Global client instance
var globalClient *Client

// Agent types based on skill/purpose
const (
	AgentTypeBuilder  = "builder"  // Full implementation, TDD cycle
	AgentTypeReviewer = "reviewer" // Code review
	AgentTypeFixer    = "fixer"    // Bug fix, red-green-commit
	AgentTypeGuardian = "guardian" // Security audit
	AgentTypeShipper  = "shipper"  // CI/CD, deploy
	AgentTypeTester   = "tester"   // E2E, QA automation
)

// AgentTypeFromWorkflow maps a workflow name to an agent type
func AgentTypeFromWorkflow(workflow string) string {
	switch workflow {
	case "implement", "builder":
		return AgentTypeBuilder
	case "code-review", "review", "design-review":
		return AgentTypeReviewer
	case "tdd-bug-fix", "fix", "fixer":
		return AgentTypeFixer
	case "security", "sec-ops", "guardian":
		return AgentTypeGuardian
	case "dev-ops", "shipper", "ship":
		return AgentTypeShipper
	case "qa-automation", "tester", "test":
		return AgentTypeTester
	default:
		return AgentTypeBuilder
	}
}

// AgentTypeFromInput infers agent type from the task description text
func AgentTypeFromInput(input string) string {
	lower := strings.ToLower(input)

	// Order matters: check more specific patterns first
	reviewKeywords := []string{"review", "audit code", "check code", "code quality"}
	for _, kw := range reviewKeywords {
		if strings.Contains(lower, kw) {
			return AgentTypeReviewer
		}
	}

	fixKeywords := []string{"fix", "bug", "debug", "repair", "patch"}
	for _, kw := range fixKeywords {
		if strings.Contains(lower, kw) {
			return AgentTypeFixer
		}
	}

	securityKeywords := []string{"security", "vulnerab", "owasp", "cve", "pentest"}
	for _, kw := range securityKeywords {
		if strings.Contains(lower, kw) {
			return AgentTypeGuardian
		}
	}

	shipKeywords := []string{"deploy", "ci/cd", "pipeline", "release", "ship"}
	for _, kw := range shipKeywords {
		if strings.Contains(lower, kw) {
			return AgentTypeShipper
		}
	}

	testKeywords := []string{"test", "e2e", "qa", "regression", "coverage"}
	for _, kw := range testKeywords {
		if strings.Contains(lower, kw) {
			return AgentTypeTester
		}
	}

	return AgentTypeBuilder
}

var (
	processAgentID string
	agentIDMutex   sync.Mutex
)

// GetAgentID returns a unique agent identifier per execution.
// Format: {type}-{short_random} (e.g. "reviewer-a3f2")
func GetAgentID() string {
	return GetAgentIDWithType(AgentTypeBuilder)
}

// GetAgentIDWithType returns a unique agent identifier with the given type prefix
// It caches the ID so the same process always uses the same ID.
func GetAgentIDWithType(agentType string) string {
	agentIDMutex.Lock()
	defer agentIDMutex.Unlock()

	if processAgentID != "" {
		return processAgentID
	}

	b := make([]byte, 4)
	_, _ = crypto_rand.Read(b)
	suffix := fmt.Sprintf("%x", b)[:4]
	processAgentID = fmt.Sprintf("%s-%s", agentType, suffix)
	return processAgentID
}

type Client struct {
	conn               *websocket.Conn
	agentID            string
	agentType          string
	engine             string
	context            string
	taskDescription    string
	model              string
	url                string
	authToken          string
	mu                 sync.Mutex
	joinRef            interface{} // nil for Phoenix Channel
	msgRef             string      // "1", "2", etc
	onEdit             func(contextType, content string)
	onCommand          func(command string, payload map[string]interface{})
	e2e                *crypto.E2ESession
	encrypted          bool
	onEncryptedMessage func(data []byte)
}

// inferContext derives the project context from workspace path or env var
func inferContext() string {
	if ctx := os.Getenv("GPTCODE_CONTEXT"); ctx != "" {
		return ctx
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "gptcode"
	}
	parts := strings.Split(cwd, string(os.PathSeparator))
	for i, p := range parts {
		if p == "workspace" && i+1 < len(parts) {
			return parts[i+1]
		}
	}
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return "gptcode"
}

// NewClient creates a new Live Dashboard client
func NewClient(dashboardURL, agentID string) *Client {
	return &Client{
		url:       dashboardURL,
		agentID:   agentID,
		agentType: AgentTypeBuilder, // default
		engine:    "cli",
		context:   inferContext(),
		authToken: os.Getenv("GPTCODE_LIVE_TOKEN"),
		joinRef:   nil,
		msgRef:    "1",
	}
}

// SetAgentType sets the agent's type/purpose
func (c *Client) SetAgentType(t string) {
	c.agentType = t
}

// SetTask sets the task description for this agent
func (c *Client) SetTask(task string) {
	c.taskDescription = task
}

// SetModel sets the model name for this agent
func (c *Client) SetModel(model string) {
	c.model = model
}

// incrementMsgRef increments the message ref as a string
func (c *Client) incrementMsgRef() {
	n, _ := strconv.Atoi(c.msgRef)
	c.msgRef = strconv.Itoa(n + 1)
}

// Connect establishes WebSocket connection to Phoenix
func (c *Client) Connect() error {
	u, err := url.Parse(c.url)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}

	// Convert to ws/wss scheme
	scheme := "ws"
	if u.Scheme == "https" || u.Scheme == "wss" {
		scheme = "wss"
	}
	u.Scheme = scheme
	u.Path = "/agent/websocket"
	u.RawQuery = "vsn=2.0.0"

	conn, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}
	c.conn = conn

	// Join agent channel
	if err := c.joinChannel(); err != nil {
		conn.Close()
		return fmt.Errorf("failed to join channel: %w", err)
	}

	// Start message handler
	go c.handleMessages()

	return nil
}

func (c *Client) joinChannel() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	topic := fmt.Sprintf("agent:%s", c.agentID)

	// Host info is metadata, not identity
	host := GetHostInfo()

	// Build join payload: type, task, host metadata, and auth
	payload := map[string]interface{}{
		"engine":    c.engine,
		"type":      c.agentType,
		"context":   c.context,
		"task":      c.taskDescription,
		"hostname":  host["hostname"],
		"workspace": host["workspace"],
		"model":     c.model,
	}

	// Add model limits if known
	if limits := GetModelLimits(c.model); limits != nil {
		payload["model_limits"] = map[string]interface{}{
			"context_window": limits.ContextWindow,
			"max_output":     limits.MaxOutput,
			"rpm":            limits.RPM,
			"tpm":            limits.TPM,
			"rpd":            limits.RPD,
			"provider":       limits.Provider,
			"tier":           limits.Tier,
		}
	}
	if c.authToken != "" {
		payload["token"] = c.authToken
	}

	msg := []interface{}{
		c.joinRef,
		c.msgRef,
		topic,
		"phx_join",
		payload,
	}
	c.incrementMsgRef()

	return c.conn.WriteJSON(msg)
}

func (c *Client) handleMessages() {
	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if !websocket.IsCloseError(err, websocket.CloseNormalClosure) {
				log.Printf("Live: connection error: %v", err)
			}
			return
		}

		var msg []interface{}
		if err := json.Unmarshal(message, &msg); err != nil {
			continue
		}

		if len(msg) < 5 {
			continue
		}

		event, ok := msg[3].(string)
		if !ok {
			continue
		}

		payload, ok := msg[4].(map[string]interface{})
		if !ok {
			continue
		}

		switch event {
		case "context_edit":
			c.handleContextEdit(payload)
		case "phx_reply":
			// Handle join reply
		case "request_sessions":
			// Respond to session request from Live
			c.handleRequestSessions(payload)
		default:
			if c.onCommand != nil {
				c.onCommand(event, payload)
			} else {
				log.Printf("Live: unknown control command: %s", event)
			}
		}
	}
}

func (c *Client) handleContextEdit(payload map[string]interface{}) {
	contextType, _ := payload["type"].(string)
	content, _ := payload["content"].(string)

	if c.onEdit != nil {
		c.onEdit(contextType, content)
	} else {
		// Default: write to .gptcode/context/
		if err := WriteContextFile(contextType, content); err != nil {
			log.Printf("Live: failed to write context: %v", err)
		}
	}
}

// handleRequestSessions responds to a session info request from Live
func (c *Client) handleRequestSessions(payload map[string]interface{}) {
	// Get hostname and workspace info
	hostname, _ := os.Hostname()
	cwd, _ := os.Getwd()
	workspace := filepath.Base(cwd)

	// Send session info back
	sessionInfo := map[string]interface{}{
		"agent_id":  c.agentID,
		"type":      c.agentType,
		"task":      c.taskDescription,
		"hostname":  hostname,
		"workspace": workspace,
		"pid":       os.Getpid(),
		"status":    "running",
	}

	// Send via sessions_update event
	c.mu.Lock()
	defer c.mu.Unlock()

	topic := fmt.Sprintf("agent:%s", c.agentID)
	msg := []interface{}{
		c.joinRef,
		c.msgRef,
		topic,
		"sessions_update",
		sessionInfo,
	}
	c.incrementMsgRef()

	if c.conn != nil {
		_ = c.conn.WriteJSON(msg)
	}
}

// SendContextUpdate sends current project context to Live
func (c *Client) SendContextUpdate(shared, next, roadmap string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	topic := fmt.Sprintf("agent:%s", c.agentID)
	msg := []interface{}{
		c.joinRef,
		c.msgRef,
		topic,
		"context_update",
		map[string]interface{}{
			"shared":  shared,
			"next":    next,
			"roadmap": roadmap,
		},
	}
	c.incrementMsgRef()

	return c.conn.WriteJSON(msg)
}

// SendTraceData sends trace data to the Live dashboard
func (c *Client) SendTraceData(data map[string]interface{}) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	topic := fmt.Sprintf("agent:%s", c.agentID)
	msg := []interface{}{
		c.joinRef,
		c.msgRef,
		topic,
		"trace_data",
		data,
	}
	c.incrementMsgRef()

	return c.conn.WriteJSON(msg)
}

// OnContextEdit sets callback for when Live edits context
func (c *Client) OnContextEdit(fn func(contextType, content string)) {
	c.onEdit = fn
}

// OnCommand sets callback for when Live sends a command
func (c *Client) OnCommand(fn func(command string, payload map[string]interface{})) {
	c.onCommand = fn
}

// Close closes the WebSocket connection
func (c *Client) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// EnableEncryption initializes E2E encryption and sends public key to server
func (c *Client) EnableEncryption() error {
	session, err := crypto.NewE2ESession()
	if err != nil {
		return fmt.Errorf("failed to create E2E session: %w", err)
	}
	c.e2e = session

	// Send our public key to server
	c.mu.Lock()
	defer c.mu.Unlock()

	topic := fmt.Sprintf("agent:%s", c.agentID)
	msg := []interface{}{
		c.joinRef,
		c.msgRef,
		topic,
		"key_exchange",
		map[string]interface{}{
			"public_key": c.e2e.PublicKey(),
		},
	}
	c.incrementMsgRef()

	log.Printf("Live: Initiated key exchange, fingerprint: %s", c.e2e.Fingerprint())
	return c.conn.WriteJSON(msg)
}

// SetRemotePublicKey completes key exchange with browser's public key
func (c *Client) SetRemotePublicKey(publicKey string) error {
	if c.e2e == nil {
		return fmt.Errorf("encryption not initialized")
	}
	if err := c.e2e.SetRemotePublicKey(publicKey); err != nil {
		return err
	}
	c.encrypted = true
	log.Printf("Live: Key exchange complete, remote fingerprint: %s", c.e2e.RemoteFingerprint())
	return nil
}

// SendEncrypted sends an encrypted message
func (c *Client) SendEncrypted(sessionID string, data []byte) error {
	if !c.encrypted || c.e2e == nil {
		return fmt.Errorf("encryption not ready")
	}

	ciphertext, err := c.e2e.Encrypt(data)
	if err != nil {
		return err
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	topic := fmt.Sprintf("agent:%s", c.agentID)
	msg := []interface{}{
		c.joinRef,
		c.msgRef,
		topic,
		"encrypted_payload",
		map[string]interface{}{
			"session_id": sessionID,
			"data":       ciphertext,
		},
	}
	c.incrementMsgRef()

	return c.conn.WriteJSON(msg)
}

// DecryptMessage decrypts a message from the browser
func (c *Client) DecryptMessage(ciphertext string) ([]byte, error) {
	if !c.encrypted || c.e2e == nil {
		return nil, fmt.Errorf("encryption not ready")
	}
	return c.e2e.Decrypt(ciphertext)
}

// IsEncrypted returns true if E2E encryption is active
func (c *Client) IsEncrypted() bool {
	return c.encrypted && c.e2e != nil && c.e2e.IsReady()
}

// OnEncryptedMessage sets callback for encrypted messages from browser
func (c *Client) OnEncryptedMessage(fn func(data []byte)) {
	c.onEncryptedMessage = fn
}

// ReadContextFile reads a context file from .gptcode/context/
func ReadContextFile(contextType string) (string, error) {
	gptcodeDir, err := findGPTCodeDir()
	if err != nil {
		return "", err
	}

	filename := contextType + ".md"
	path := filepath.Join(gptcodeDir, "context", filename)

	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}

	return string(data), nil
}

// WriteContextFile writes to a context file in .gptcode/context/
func WriteContextFile(contextType, content string) error {
	gptcodeDir, err := findGPTCodeDir()
	if err != nil {
		return err
	}

	filename := contextType + ".md"
	path := filepath.Join(gptcodeDir, "context", filename)

	return os.WriteFile(path, []byte(content), 0644)
}

// ReadAllContext reads all context files
func ReadAllContext() (shared, next, roadmap string, err error) {
	shared, _ = ReadContextFile("shared")
	next, _ = ReadContextFile("next")
	roadmap, _ = ReadContextFile("roadmap")
	return shared, next, roadmap, nil
}

func findGPTCodeDir() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}

	dir := cwd
	for {
		gptcodePath := filepath.Join(dir, ".gptcode")
		if _, err := os.Stat(gptcodePath); err == nil {
			return gptcodePath, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	return "", fmt.Errorf(".gptcode directory not found")
}

// AutoSync connects and syncs context automatically
func AutoSync(dashboardURL, agentID string) (*Client, error) {
	client := NewClient(dashboardURL, agentID)

	if err := client.Connect(); err != nil {
		return nil, err
	}

	// Send initial context
	shared, next, roadmap, _ := ReadAllContext()
	if shared != "" || next != "" || roadmap != "" {
		if err := client.SendContextUpdate(shared, next, roadmap); err != nil {
			log.Printf("Live: failed to send initial context: %v", err)
		}
	}

	// Watch for local changes
	go watchContextChanges(client)

	return client, nil
}

func watchContextChanges(client *Client) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	var lastShared, lastNext, lastRoadmap string

	for range ticker.C {
		shared, next, roadmap, _ := ReadAllContext()

		if shared != lastShared || next != lastNext || roadmap != lastRoadmap {
			if err := client.SendContextUpdate(shared, next, roadmap); err != nil {
				log.Printf("Live: failed to sync context: %v", err)
			}
			lastShared, lastNext, lastRoadmap = shared, next, roadmap
		}
	}
}

// GetDashboardURL returns the Live dashboard URL from config or default
func GetDashboardURL() string {
	if url := os.Getenv("GPTCODE_LIVE_URL"); url != "" {
		return url
	}
	return "https://live.gptcode.app"
}

// GetHostInfo returns machine metadata (hostname + workspace).
// This is NOT the agent's identity — it's where the agent runs.
func GetHostInfo() map[string]string {
	hostname, err := os.Hostname()
	if err != nil {
		hostname = "unknown"
	}
	// Clean up ".local" suffix
	hostname = strings.TrimSuffix(hostname, ".local")

	cwd, _ := os.Getwd()
	parts := strings.Split(cwd, string(os.PathSeparator))
	workspace := ""
	if len(parts) > 0 {
		workspace = parts[len(parts)-1]
	}

	return map[string]string{
		"hostname":  hostname,
		"workspace": workspace,
	}
}

// GetClient returns the global live client instance
func GetClient() *Client {
	return globalClient
}

// SetGlobalClient sets the global live client instance
func SetGlobalClient(client *Client) {
	globalClient = client
}

// SendExecutionStep sends a real-time execution step update
func (c *Client) SendExecutionStep(stepType, description string, metadata map[string]interface{}) error {
	if c == nil || c.conn == nil {
		return nil
	}

	topic := fmt.Sprintf("agent:%s", c.agentID)

	event := map[string]interface{}{
		"type":        stepType,
		"description": description,
		"timestamp":   time.Now().Format(time.RFC3339),
	}

	for k, v := range metadata {
		event[k] = v
	}

	msg := []interface{}{
		0, 0, topic, "execution_step", event,
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn == nil {
		return nil
	}

	return c.conn.WriteJSON(msg)
}

// SendCommandResult sends the result of a dashboard command back to Live
func (c *Client) SendCommandResult(command string, success bool, message string) error {
	topic := fmt.Sprintf("agent:%s", c.agentID)

	msg := []interface{}{
		0, 0, topic, "command_result",
		map[string]interface{}{
			"command": command,
			"success": success,
			"message": message,
		},
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn == nil {
		return nil
	}

	return c.conn.WriteJSON(msg)
}
