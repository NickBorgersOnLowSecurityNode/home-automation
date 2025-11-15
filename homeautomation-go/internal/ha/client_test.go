package ha

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

// mockHAServer creates a mock Home Assistant WebSocket server
func mockHAServer(t *testing.T, handler func(*websocket.Conn)) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Fatalf("Failed to upgrade connection: %v", err)
		}
		defer conn.Close()

		handler(conn)
	}))
}

// standardAuthFlow handles the standard authentication flow
func standardAuthFlow(t *testing.T, conn *websocket.Conn, token string) {
	// Send auth_required
	err := conn.WriteJSON(Message{Type: "auth_required"})
	require.NoError(t, err)

	// Receive auth message
	var authMsg AuthMessage
	err = conn.ReadJSON(&authMsg)
	require.NoError(t, err)
	assert.Equal(t, "auth", authMsg.Type)
	assert.Equal(t, token, authMsg.AccessToken)

	// Send auth_ok
	err = conn.WriteJSON(Message{Type: "auth_ok"})
	require.NoError(t, err)
}

func TestClient_Connect(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	token := "test_token"

	t.Run("successful connection", func(t *testing.T) {
		server := mockHAServer(t, func(conn *websocket.Conn) {
			standardAuthFlow(t, conn, token)

			// Receive subscribe_events message
			var subMsg SubscribeEventsRequest
			conn.ReadJSON(&subMsg)

			// Send success response
			success := true
			conn.WriteJSON(Message{
				ID:      subMsg.ID,
				Type:    "result",
				Success: &success,
			})

			// Keep connection open
			time.Sleep(100 * time.Millisecond)
		})
		defer server.Close()

		url := "ws" + strings.TrimPrefix(server.URL, "http")
		client := NewClient(url, token, logger)

		err := client.Connect()
		assert.NoError(t, err)
		assert.True(t, client.IsConnected())

		client.Disconnect()
	})

	t.Run("invalid token", func(t *testing.T) {
		server := mockHAServer(t, func(conn *websocket.Conn) {
			// Send auth_required
			conn.WriteJSON(Message{Type: "auth_required"})

			// Receive auth message
			var authMsg AuthMessage
			conn.ReadJSON(&authMsg)

			// Send auth_invalid
			conn.WriteJSON(Message{Type: "auth_invalid"})
		})
		defer server.Close()

		url := "ws" + strings.TrimPrefix(server.URL, "http")
		client := NewClient(url, "wrong_token", logger)

		err := client.Connect()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "authentication failed")
		assert.False(t, client.IsConnected())
	})

	t.Run("already connected", func(t *testing.T) {
		server := mockHAServer(t, func(conn *websocket.Conn) {
			standardAuthFlow(t, conn, token)

			// Receive subscribe_events
			var subMsg SubscribeEventsRequest
			conn.ReadJSON(&subMsg)
			success := true
			conn.WriteJSON(Message{
				ID:      subMsg.ID,
				Type:    "result",
				Success: &success,
			})

			time.Sleep(100 * time.Millisecond)
		})
		defer server.Close()

		url := "ws" + strings.TrimPrefix(server.URL, "http")
		client := NewClient(url, token, logger)

		err := client.Connect()
		require.NoError(t, err)

		err = client.Connect()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "already connected")

		client.Disconnect()
	})
}

func TestClient_GetAllStates(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	token := "test_token"

	server := mockHAServer(t, func(conn *websocket.Conn) {
		standardAuthFlow(t, conn, token)

		// Handle subscribe_events
		var subMsg SubscribeEventsRequest
		conn.ReadJSON(&subMsg)
		success := true
		conn.WriteJSON(Message{
			ID:      subMsg.ID,
			Type:    "result",
			Success: &success,
		})

		// Handle get_states request
		var statesReq GetStatesRequest
		conn.ReadJSON(&statesReq)

		states := []*State{
			{
				EntityID: "input_boolean.test",
				State:    "on",
				Attributes: map[string]interface{}{
					"friendly_name": "Test Boolean",
				},
			},
			{
				EntityID: "input_number.test",
				State:    "42.5",
				Attributes: map[string]interface{}{
					"friendly_name": "Test Number",
				},
			},
		}

		statesJSON, _ := json.Marshal(states)
		conn.WriteJSON(Message{
			ID:      statesReq.ID,
			Type:    "result",
			Success: &success,
			Result:  statesJSON,
		})

		time.Sleep(100 * time.Millisecond)
	})
	defer server.Close()

	url := "ws" + strings.TrimPrefix(server.URL, "http")
	client := NewClient(url, token, logger)

	err := client.Connect()
	require.NoError(t, err)
	defer client.Disconnect()

	states, err := client.GetAllStates()
	assert.NoError(t, err)
	assert.Len(t, states, 2)
	assert.Equal(t, "input_boolean.test", states[0].EntityID)
	assert.Equal(t, "on", states[0].State)
}

func TestClient_GetState(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	token := "test_token"

	server := mockHAServer(t, func(conn *websocket.Conn) {
		standardAuthFlow(t, conn, token)

		// Handle subscribe_events
		var subMsg SubscribeEventsRequest
		conn.ReadJSON(&subMsg)
		success := true
		conn.WriteJSON(Message{
			ID:      subMsg.ID,
			Type:    "result",
			Success: &success,
		})

		// Handle get_states request
		var statesReq GetStatesRequest
		conn.ReadJSON(&statesReq)

		states := []*State{
			{
				EntityID: "input_boolean.test",
				State:    "on",
			},
		}

		statesJSON, _ := json.Marshal(states)
		conn.WriteJSON(Message{
			ID:      statesReq.ID,
			Type:    "result",
			Success: &success,
			Result:  statesJSON,
		})

		time.Sleep(100 * time.Millisecond)
	})
	defer server.Close()

	url := "ws" + strings.TrimPrefix(server.URL, "http")
	client := NewClient(url, token, logger)

	err := client.Connect()
	require.NoError(t, err)
	defer client.Disconnect()

	state, err := client.GetState("input_boolean.test")
	assert.NoError(t, err)
	assert.Equal(t, "input_boolean.test", state.EntityID)
	assert.Equal(t, "on", state.State)

	_, err = client.GetState("nonexistent")
	assert.Error(t, err)
}

func TestClient_CallService(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	token := "test_token"

	server := mockHAServer(t, func(conn *websocket.Conn) {
		standardAuthFlow(t, conn, token)

		// Handle subscribe_events
		var subMsg SubscribeEventsRequest
		conn.ReadJSON(&subMsg)
		success := true
		conn.WriteJSON(Message{
			ID:      subMsg.ID,
			Type:    "result",
			Success: &success,
		})

		// Handle call_service request
		var serviceReq CallServiceRequest
		conn.ReadJSON(&serviceReq)

		assert.Equal(t, "input_boolean", serviceReq.Domain)
		assert.Equal(t, "turn_on", serviceReq.Service)
		assert.Equal(t, "input_boolean.test", serviceReq.ServiceData["entity_id"])

		conn.WriteJSON(Message{
			ID:      serviceReq.ID,
			Type:    "result",
			Success: &success,
		})

		time.Sleep(100 * time.Millisecond)
	})
	defer server.Close()

	url := "ws" + strings.TrimPrefix(server.URL, "http")
	client := NewClient(url, token, logger)

	err := client.Connect()
	require.NoError(t, err)
	defer client.Disconnect()

	err = client.CallService("input_boolean", "turn_on", map[string]interface{}{
		"entity_id": "input_boolean.test",
	})
	assert.NoError(t, err)
}

func TestClient_SetInputBoolean(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	token := "test_token"

	testCases := []struct {
		name    string
		value   bool
		service string
	}{
		{"turn on", true, "turn_on"},
		{"turn off", false, "turn_off"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			server := mockHAServer(t, func(conn *websocket.Conn) {
				standardAuthFlow(t, conn, token)

				// Handle subscribe_events
				var subMsg SubscribeEventsRequest
				conn.ReadJSON(&subMsg)
				success := true
				conn.WriteJSON(Message{
					ID:      subMsg.ID,
					Type:    "result",
					Success: &success,
				})

				// Handle service call
				var serviceReq CallServiceRequest
				conn.ReadJSON(&serviceReq)

				assert.Equal(t, "input_boolean", serviceReq.Domain)
				assert.Equal(t, tc.service, serviceReq.Service)

				conn.WriteJSON(Message{
					ID:      serviceReq.ID,
					Type:    "result",
					Success: &success,
				})

				time.Sleep(50 * time.Millisecond)
			})
			defer server.Close()

			url := "ws" + strings.TrimPrefix(server.URL, "http")
			client := NewClient(url, token, logger)

			err := client.Connect()
			require.NoError(t, err)
			defer client.Disconnect()

			err = client.SetInputBoolean("test", tc.value)
			assert.NoError(t, err)
		})
	}
}

func TestClient_SetInputNumber(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	token := "test_token"

	server := mockHAServer(t, func(conn *websocket.Conn) {
		standardAuthFlow(t, conn, token)

		// Handle subscribe_events
		var subMsg SubscribeEventsRequest
		conn.ReadJSON(&subMsg)
		success := true
		conn.WriteJSON(Message{
			ID:      subMsg.ID,
			Type:    "result",
			Success: &success,
		})

		// Handle service call
		var serviceReq CallServiceRequest
		conn.ReadJSON(&serviceReq)

		assert.Equal(t, "input_number", serviceReq.Domain)
		assert.Equal(t, "set_value", serviceReq.Service)
		assert.Equal(t, 42.5, serviceReq.ServiceData["value"])

		conn.WriteJSON(Message{
			ID:      serviceReq.ID,
			Type:    "result",
			Success: &success,
		})

		time.Sleep(100 * time.Millisecond)
	})
	defer server.Close()

	url := "ws" + strings.TrimPrefix(server.URL, "http")
	client := NewClient(url, token, logger)

	err := client.Connect()
	require.NoError(t, err)
	defer client.Disconnect()

	err = client.SetInputNumber("test", 42.5)
	assert.NoError(t, err)
}

func TestClient_SetInputText(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	token := "test_token"

	server := mockHAServer(t, func(conn *websocket.Conn) {
		standardAuthFlow(t, conn, token)

		// Handle subscribe_events
		var subMsg SubscribeEventsRequest
		conn.ReadJSON(&subMsg)
		success := true
		conn.WriteJSON(Message{
			ID:      subMsg.ID,
			Type:    "result",
			Success: &success,
		})

		// Handle service call
		var serviceReq CallServiceRequest
		conn.ReadJSON(&serviceReq)

		assert.Equal(t, "input_text", serviceReq.Domain)
		assert.Equal(t, "set_value", serviceReq.Service)
		assert.Equal(t, "test_value", serviceReq.ServiceData["value"])

		conn.WriteJSON(Message{
			ID:      serviceReq.ID,
			Type:    "result",
			Success: &success,
		})

		time.Sleep(100 * time.Millisecond)
	})
	defer server.Close()

	url := "ws" + strings.TrimPrefix(server.URL, "http")
	client := NewClient(url, token, logger)

	err := client.Connect()
	require.NoError(t, err)
	defer client.Disconnect()

	err = client.SetInputText("test", "test_value")
	assert.NoError(t, err)
}

func TestMockClient(t *testing.T) {
	mock := NewMockClient()

	t.Run("connection", func(t *testing.T) {
		assert.False(t, mock.IsConnected())

		err := mock.Connect()
		assert.NoError(t, err)
		assert.True(t, mock.IsConnected())

		err = mock.Connect()
		assert.Error(t, err)

		err = mock.Disconnect()
		assert.NoError(t, err)
		assert.False(t, mock.IsConnected())
	})

	t.Run("state management", func(t *testing.T) {
		mock.SetState("input_boolean.test", "on", map[string]interface{}{
			"friendly_name": "Test",
		})

		state, err := mock.GetState("input_boolean.test")
		assert.NoError(t, err)
		assert.Equal(t, "on", state.State)

		_, err = mock.GetState("nonexistent")
		assert.Error(t, err)
	})

	t.Run("service calls", func(t *testing.T) {
		mock.ClearServiceCalls()

		err := mock.SetInputBoolean("test", true)
		assert.NoError(t, err)

		calls := mock.GetServiceCalls()
		assert.Len(t, calls, 1)
		assert.Equal(t, "input_boolean", calls[0].Domain)
		assert.Equal(t, "turn_on", calls[0].Service)
	})

	t.Run("subscriptions", func(t *testing.T) {
		callCount := 0
		handler := func(entityID string, oldState, newState *State) {
			callCount++
			assert.Equal(t, "input_boolean.test", entityID)
			assert.Equal(t, "off", newState.State)
		}

		_, err := mock.SubscribeStateChanges("input_boolean.test", handler)
		assert.NoError(t, err)

		mock.SimulateStateChange("input_boolean.test", "off")
		time.Sleep(50 * time.Millisecond)

		assert.Equal(t, 1, callCount)
	})
}
