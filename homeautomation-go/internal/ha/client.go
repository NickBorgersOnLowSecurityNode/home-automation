package ha

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

// HAClient defines the interface for Home Assistant WebSocket client
type HAClient interface {
	Connect() error
	Disconnect() error
	IsConnected() bool
	GetState(entityID string) (*State, error)
	GetAllStates() ([]*State, error)
	CallService(domain, service string, data map[string]interface{}) error
	SubscribeStateChanges(entityID string, handler StateChangeHandler) (Subscription, error)
	SetInputBoolean(name string, value bool) error
	SetInputNumber(name string, value float64) error
	SetInputText(name string, value string) error
}

// subscriberEntry holds a handler with its unique subscription ID
type subscriberEntry struct {
	subID   int
	handler StateChangeHandler
}

// Client implements HAClient interface
type Client struct {
	url         string
	token       string
	logger      *zap.Logger
	conn        *websocket.Conn
	connected   bool
	connMu      sync.RWMutex
	msgID       int
	msgIDMu     sync.Mutex
	pending     map[int]chan Message
	pendingMu   sync.Mutex
	subscribers map[string][]subscriberEntry
	subsMu      sync.RWMutex
	nextSubID   int
	nextSubIDMu sync.Mutex
	ctx         context.Context
	cancel      context.CancelFunc
	reconnect   bool
	writeMu     sync.Mutex // Protects websocket writes
}

func (c *Client) clearSubscribers() {
	c.subsMu.Lock()
	defer c.subsMu.Unlock()

	if len(c.subscribers) == 0 {
		c.subscribers = make(map[string][]subscriberEntry)
		return
	}

	c.subscribers = make(map[string][]subscriberEntry)
}

func (c *Client) resetContextLocked() {
	if c.cancel != nil {
		c.cancel()
	}
	c.ctx, c.cancel = context.WithCancel(context.Background())
}

// NewClient creates a new Home Assistant WebSocket client
func NewClient(url, token string, logger *zap.Logger) *Client {
	ctx, cancel := context.WithCancel(context.Background())
	return &Client{
		url:         url,
		token:       token,
		logger:      logger,
		pending:     make(map[int]chan Message),
		subscribers: make(map[string][]subscriberEntry),
		ctx:         ctx,
		cancel:      cancel,
		reconnect:   true,
	}
}

// Connect establishes WebSocket connection and authenticates
func (c *Client) Connect() error {
	c.connMu.Lock()

	if c.connected {
		c.connMu.Unlock()
		return fmt.Errorf("already connected")
	}

	// Connect to WebSocket
	conn, _, err := websocket.DefaultDialer.Dial(c.url, nil)
	if err != nil {
		c.connMu.Unlock()
		return fmt.Errorf("failed to connect to WebSocket: %w", err)
	}
	c.conn = conn

	// Receive auth_required message
	var authRequired Message
	if err := c.conn.ReadJSON(&authRequired); err != nil {
		c.conn.Close()
		c.connMu.Unlock()
		return fmt.Errorf("failed to read auth_required: %w", err)
	}

	if authRequired.Type != "auth_required" {
		c.conn.Close()
		c.connMu.Unlock()
		return fmt.Errorf("expected auth_required, got %s", authRequired.Type)
	}

	// Send authentication
	authMsg := AuthMessage{
		Type:        "auth",
		AccessToken: c.token,
	}
	c.writeMu.Lock()
	err = c.conn.WriteJSON(authMsg)
	c.writeMu.Unlock()

	if err != nil {
		c.conn.Close()
		c.connMu.Unlock()
		return fmt.Errorf("failed to send auth: %w", err)
	}

	// Receive auth response
	var authResponse Message
	if err := c.conn.ReadJSON(&authResponse); err != nil {
		c.conn.Close()
		c.connMu.Unlock()
		return fmt.Errorf("failed to read auth response: %w", err)
	}

	if authResponse.Type == "auth_invalid" {
		c.conn.Close()
		c.connMu.Unlock()
		return fmt.Errorf("authentication failed: invalid token")
	}

	if authResponse.Type != "auth_ok" {
		c.conn.Close()
		c.connMu.Unlock()
		return fmt.Errorf("expected auth_ok, got %s", authResponse.Type)
	}

	c.resetContextLocked()
	c.connected = true
	c.reconnect = true
	c.logger.Info("Connected to Home Assistant")

	// Start background message receiver
	go c.receiveMessages()

	// Release lock before calling subscribeToStateChanges to avoid deadlock
	c.connMu.Unlock()

	// Subscribe to state_changed events
	if err := c.subscribeToStateChanges(); err != nil {
		c.logger.Warn("Failed to subscribe to state changes", zap.Error(err))
	}

	return nil
}

// Disconnect closes the WebSocket connection
func (c *Client) Disconnect() error {
	c.connMu.Lock()
	defer c.connMu.Unlock()

	if !c.connected {
		return nil
	}

	c.reconnect = false
	c.cancel()
	c.connected = false

	if c.conn != nil {
		// Send close message (protected by writeMu)
		c.writeMu.Lock()
		c.conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
		c.writeMu.Unlock()

		c.conn.Close()
		c.conn = nil
	}

	c.clearSubscribers()
	c.logger.Info("Disconnected from Home Assistant")
	return nil
}

// IsConnected returns true if client is connected
func (c *Client) IsConnected() bool {
	c.connMu.RLock()
	defer c.connMu.RUnlock()
	return c.connected
}

// nextMsgID returns the next message ID
func (c *Client) nextMsgID() int {
	c.msgIDMu.Lock()
	defer c.msgIDMu.Unlock()
	c.msgID++
	return c.msgID
}

// sendMessage sends a message and waits for response
func (c *Client) sendMessage(msg interface{}) (*Message, error) {
	c.connMu.RLock()
	if !c.connected {
		c.connMu.RUnlock()
		return nil, fmt.Errorf("not connected")
	}
	c.connMu.RUnlock()

	// Get message ID
	var msgID int
	switch m := msg.(type) {
	case *CallServiceRequest:
		msgID = m.ID
	case *GetStatesRequest:
		msgID = m.ID
	case *SubscribeEventsRequest:
		msgID = m.ID
	default:
		return nil, fmt.Errorf("unsupported message type")
	}

	// Create response channel
	respChan := make(chan Message, 1)
	c.pendingMu.Lock()
	c.pending[msgID] = respChan
	c.pendingMu.Unlock()

	// Clean up after timeout
	defer func() {
		c.pendingMu.Lock()
		delete(c.pending, msgID)
		c.pendingMu.Unlock()
	}()

	// Send message (protected by writeMu to prevent concurrent writes)
	c.writeMu.Lock()
	err := c.conn.WriteJSON(msg)
	c.writeMu.Unlock()

	if err != nil {
		return nil, fmt.Errorf("failed to send message: %w", err)
	}

	// Wait for response with timeout
	select {
	case resp := <-respChan:
		if resp.Success != nil && !*resp.Success {
			if resp.Error != nil {
				return nil, fmt.Errorf("HA error: %s - %s", resp.Error.Code, resp.Error.Message)
			}
			return nil, fmt.Errorf("request failed")
		}
		return &resp, nil
	case <-time.After(10 * time.Second):
		return nil, fmt.Errorf("timeout waiting for response")
	case <-c.ctx.Done():
		return nil, fmt.Errorf("client disconnected")
	}
}

// receiveMessages handles incoming messages in the background
func (c *Client) receiveMessages() {
	for {
		select {
		case <-c.ctx.Done():
			return
		default:
		}

		var msg Message
		if err := c.conn.ReadJSON(&msg); err != nil {
			c.logger.Error("Failed to read message", zap.Error(err))
			c.handleDisconnect()
			return
		}

		// Handle event messages
		if msg.Type == "event" {
			c.handleEvent(&msg)
			continue
		}

		// Route response to waiting goroutine
		if msg.ID > 0 {
			c.pendingMu.Lock()
			if ch, ok := c.pending[msg.ID]; ok {
				select {
				case ch <- msg:
				default:
					c.logger.Warn("Response channel full", zap.Int("msg_id", msg.ID))
				}
			}
			c.pendingMu.Unlock()
		}
	}
}

// handleEvent processes event messages
func (c *Client) handleEvent(msg *Message) {
	if msg.Event == nil {
		return
	}

	// Only handle state_changed events
	if msg.Event.EventType != "state_changed" {
		return
	}

	var eventData StateChangedEvent
	if err := json.Unmarshal(msg.Event.Data, &eventData); err != nil {
		c.logger.Error("Failed to unmarshal state_changed event", zap.Error(err))
		return
	}

	// Notify subscribers
	c.subsMu.RLock()
	entries := append([]subscriberEntry(nil), c.subscribers[eventData.EntityID]...)
	c.subsMu.RUnlock()

	for _, entry := range entries {
		entry.handler(eventData.EntityID, eventData.OldState, eventData.NewState)
	}
}

// handleDisconnect handles connection loss
func (c *Client) handleDisconnect() {
	c.connMu.Lock()
	c.connected = false
	c.connMu.Unlock()

	c.logger.Warn("Connection lost")

	if !c.reconnect {
		return
	}

	// Attempt to reconnect with exponential backoff
	go c.attemptReconnect()
}

// attemptReconnect tries to reconnect with exponential backoff
func (c *Client) attemptReconnect() {
	backoff := time.Second
	maxBackoff := 30 * time.Second

	for {
		select {
		case <-c.ctx.Done():
			return
		case <-time.After(backoff):
		}

		c.logger.Info("Attempting to reconnect...")

		if err := c.Connect(); err != nil {
			c.logger.Error("Reconnection failed", zap.Error(err))
			backoff *= 2
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
			continue
		}

		c.logger.Info("Reconnected successfully")
		return
	}
}

// subscribeToStateChanges subscribes to all state_changed events
func (c *Client) subscribeToStateChanges() error {
	msgID := c.nextMsgID()
	req := &SubscribeEventsRequest{
		ID:        msgID,
		Type:      "subscribe_events",
		EventType: "state_changed",
	}

	_, err := c.sendMessage(req)
	return err
}

// GetState retrieves the state of an entity
func (c *Client) GetState(entityID string) (*State, error) {
	states, err := c.GetAllStates()
	if err != nil {
		return nil, err
	}

	for _, state := range states {
		if state.EntityID == entityID {
			return state, nil
		}
	}

	return nil, fmt.Errorf("entity %s not found", entityID)
}

// GetAllStates retrieves all entity states
func (c *Client) GetAllStates() ([]*State, error) {
	msgID := c.nextMsgID()
	req := &GetStatesRequest{
		ID:   msgID,
		Type: "get_states",
	}

	resp, err := c.sendMessage(req)
	if err != nil {
		return nil, err
	}

	var states []*State
	if err := json.Unmarshal(resp.Result, &states); err != nil {
		return nil, fmt.Errorf("failed to unmarshal states: %w", err)
	}

	return states, nil
}

// CallService calls a Home Assistant service
func (c *Client) CallService(domain, service string, data map[string]interface{}) error {
	msgID := c.nextMsgID()
	req := &CallServiceRequest{
		ID:          msgID,
		Type:        "call_service",
		Domain:      domain,
		Service:     service,
		ServiceData: data,
	}

	_, err := c.sendMessage(req)
	return err
}

// SubscribeStateChanges subscribes to state changes for a specific entity
func (c *Client) SubscribeStateChanges(entityID string, handler StateChangeHandler) (Subscription, error) {
	// Get unique subscription ID
	c.nextSubIDMu.Lock()
	subID := c.nextSubID
	c.nextSubID++
	c.nextSubIDMu.Unlock()

	// Add subscriber entry
	c.subsMu.Lock()
	c.subscribers[entityID] = append(c.subscribers[entityID], subscriberEntry{
		subID:   subID,
		handler: handler,
	})
	c.subsMu.Unlock()

	return &subscription{
		entityID: entityID,
		subID:    subID,
		client:   c,
	}, nil
}

// unsubscribe removes a specific subscription by entity ID and subscription ID
func (c *Client) unsubscribe(entityID string, subID int) error {
	c.subsMu.Lock()
	defer c.subsMu.Unlock()

	subscribers, ok := c.subscribers[entityID]
	if !ok {
		return nil // Already unsubscribed
	}

	// Find and remove the subscription with matching subID
	for i, entry := range subscribers {
		if entry.subID == subID {
			// Remove this entry by slicing
			c.subscribers[entityID] = append(subscribers[:i], subscribers[i+1:]...)

			// If no more subscribers for this entity, delete the entry
			if len(c.subscribers[entityID]) == 0 {
				delete(c.subscribers, entityID)
			}
			break
		}
	}

	return nil
}

// SetInputBoolean sets the value of an input_boolean
func (c *Client) SetInputBoolean(name string, value bool) error {
	service := "turn_off"
	if value {
		service = "turn_on"
	}

	return c.CallService("input_boolean", service, map[string]interface{}{
		"entity_id": fmt.Sprintf("input_boolean.%s", name),
	})
}

// SetInputNumber sets the value of an input_number
func (c *Client) SetInputNumber(name string, value float64) error {
	return c.CallService("input_number", "set_value", map[string]interface{}{
		"entity_id": fmt.Sprintf("input_number.%s", name),
		"value":     value,
	})
}

// SetInputText sets the value of an input_text
func (c *Client) SetInputText(name string, value string) error {
	return c.CallService("input_text", "set_value", map[string]interface{}{
		"entity_id": fmt.Sprintf("input_text.%s", name),
		"value":     value,
	})
}
