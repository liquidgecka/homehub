// Copyright 2026 - Brady Catherman
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package homeassistant

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

// EntityState represents a Home Assistant entity state snapshot.
type EntityState struct {
	EntityID    string                 `json:"entity_id"`
	State       string                 `json:"state"`
	Attributes  map[string]interface{} `json:"attributes"`
	LastChanged string                 `json:"last_changed"`
}

// Domain returns the Home Assistant domain prefix of EntityID (e.g. "light").
func (e EntityState) Domain() string {
	parts := strings.SplitN(e.EntityID, ".", 2)
	if len(parts) == 0 {
		return ""
	}
	return parts[0]
}

// FriendlyName returns override if non-empty, otherwise the entity's
// friendly_name attribute or EntityID.
func (e EntityState) FriendlyName(override string) string {
	if trimmed := strings.TrimSpace(override); trimmed != "" {
		return trimmed
	}
	if name := e.StringAttr("friendly_name"); name != "" {
		return name
	}
	return e.EntityID
}

// IsActive reports whether the entity is considered active/on for UI styling.
func (e EntityState) IsActive() bool {
	switch strings.ToLower(strings.TrimSpace(e.State)) {
	case "on", "open", "opening", "unlocked", "heat", "cool",
		"heat_cool", "auto", "dry", "fan_only", "home", "active":
		return true
	default:
		return false
	}
}

// FloatAttr retrieves a numeric attribute as a float64.
func (e EntityState) FloatAttr(key string) (float64, bool) {
	if e.Attributes == nil {
		return 0, false
	}
	val, ok := e.Attributes[key]
	if !ok || val == nil {
		return 0, false
	}
	switch v := val.(type) {
	case float64:
		return v, true
	case int:
		return float64(v), true
	case int64:
		return float64(v), true
	default:
		return 0, false
	}
}

// StringAttr retrieves a string attribute if present.
func (e EntityState) StringAttr(key string) string {
	if e.Attributes == nil {
		return ""
	}
	if s, ok := e.Attributes[key].(string); ok {
		return s
	}
	return ""
}

// Client communicates with a Home Assistant instance over REST and WebSocket.
type Client struct {
	baseURL    string
	token      string
	httpClient *http.Client
}

// NewClient constructs a Home Assistant API client.
func NewClient(baseURL, token string) *Client {
	return &Client{
		baseURL: strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		token:   strings.TrimSpace(token),
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// GetStates retrieves current entity states via GET /api/states.
func (c *Client) GetStates(
	ctx context.Context,
) (map[string]EntityState, error) {
	if c.baseURL == "" {
		return nil, fmt.Errorf("home assistant URL not configured")
	}
	req, err := http.NewRequestWithContext(
		ctx, http.MethodGet, c.baseURL+"/api/states", nil,
	)
	if err != nil {
		return nil, err
	}
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf(
			"home assistant GET /api/states returned %d: %s",
			resp.StatusCode, strings.TrimSpace(string(body)),
		)
	}

	var list []EntityState
	if err := json.NewDecoder(resp.Body).Decode(&list); err != nil {
		return nil, fmt.Errorf("failed to decode states: %w", err)
	}
	states := make(map[string]EntityState, len(list))
	for _, s := range list {
		states[s.EntityID] = s
	}
	return states, nil
}

// CallService invokes a Home Assistant service via POST /api/services/<d>/<s>.
func (c *Client) CallService(
	ctx context.Context, domain, service string, data map[string]interface{},
) error {
	if c.baseURL == "" {
		return fmt.Errorf("home assistant URL not configured")
	}
	bodyBytes, err := json.Marshal(data)
	if err != nil {
		return err
	}
	endpoint := fmt.Sprintf(
		"%s/api/services/%s/%s", c.baseURL, domain, service,
	)
	req, err := http.NewRequestWithContext(
		ctx, http.MethodPost, endpoint, bytes.NewReader(bodyBytes),
	)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf(
			"service %s.%s returned %d: %s",
			domain, service, resp.StatusCode, strings.TrimSpace(string(body)),
		)
	}
	return nil
}

// ToggleOrActivate performs the primary interactive action for an entity.
func (c *Client) ToggleOrActivate(
	ctx context.Context, state EntityState,
) error {
	domain := state.Domain()
	data := map[string]interface{}{"entity_id": state.EntityID}

	switch domain {
	case "light", "switch", "fan", "input_boolean":
		return c.CallService(ctx, domain, "toggle", data)
	case "scene", "script":
		return c.CallService(ctx, domain, "turn_on", data)
	case "button", "input_button":
		return c.CallService(ctx, domain, "press", data)
	case "lock":
		svc := "lock"
		if strings.EqualFold(state.State, "locked") {
			svc = "unlock"
		}
		return c.CallService(ctx, "lock", svc, data)
	case "cover":
		svc := "open_cover"
		low := strings.ToLower(state.State)
		if low == "open" || low == "opening" {
			svc = "close_cover"
		}
		return c.CallService(ctx, "cover", svc, data)
	default:
		return fmt.Errorf("unsupported interactive domain: %s", domain)
	}
}

// SetClimateTemperature updates a climate entity's target temperature.
func (c *Client) SetClimateTemperature(
	ctx context.Context, entityID string, temp float64,
) error {
	return c.CallService(ctx, "climate", "set_temperature", map[string]interface{}{
		"entity_id":   entityID,
		"temperature": temp,
	})
}

func websocketURL(baseURL string) (string, error) {
	u, err := url.Parse(strings.TrimSpace(baseURL))
	if err != nil {
		return "", err
	}
	switch u.Scheme {
	case "https":
		u.Scheme = "wss"
	case "http", "":
		u.Scheme = "ws"
	case "ws", "wss":
	default:
		return "", fmt.Errorf("unsupported scheme %q", u.Scheme)
	}
	u.Path = strings.TrimRight(u.Path, "/") + "/api/websocket"
	return u.String(), nil
}

// SubscribeStateChanges maintains a WebSocket connection to Home Assistant and
// emits live state_changed events for entities in entityIDs until ctx ends.
func (c *Client) SubscribeStateChanges(
	ctx context.Context,
	entityIDs map[string]bool,
	onChange func(EntityState),
	onStatus func(connected bool),
) {
	wsURL, err := websocketURL(c.baseURL)
	if err != nil {
		log.Printf("HA WS: Invalid base URL %q: %v", c.baseURL, err)
		return
	}

	const maxBackoff = 60 * time.Second
	backoff := 2 * time.Second

	for {
		if ctx.Err() != nil {
			return
		}

		err := c.subscribeOnce(ctx, wsURL, entityIDs, onChange, onStatus)
		if onStatus != nil {
			onStatus(false)
		}
		if ctx.Err() != nil {
			return
		}
		if err != nil {
			log.Printf(
				"HA WS: Connection error: %v. Reconnecting in %s...",
				err, backoff,
			)
		}

		select {
		case <-time.After(backoff):
			backoff *= 2
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
		case <-ctx.Done():
			return
		}
	}
}

func (c *Client) subscribeOnce(
	ctx context.Context,
	wsURL string,
	entityIDs map[string]bool,
	onChange func(EntityState),
	onStatus func(connected bool),
) error {
	conn, _, err := websocket.DefaultDialer.DialContext(ctx, wsURL, nil)
	if err != nil {
		return err
	}

	connDone := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			_ = conn.Close()
		case <-connDone:
		}
	}()
	defer func() {
		close(connDone)
		_ = conn.Close()
	}()

	var authMsg struct {
		Type string `json:"type"`
	}
	if err := conn.ReadJSON(&authMsg); err != nil {
		return err
	}
	if authMsg.Type == "auth_required" {
		if err := conn.WriteJSON(map[string]interface{}{
			"type":         "auth",
			"access_token": c.token,
		}); err != nil {
			return err
		}
		if err := conn.ReadJSON(&authMsg); err != nil {
			return err
		}
	}
	if authMsg.Type != "auth_ok" {
		return fmt.Errorf("unexpected auth response %q", authMsg.Type)
	}

	if err := conn.WriteJSON(map[string]interface{}{
		"id":         1,
		"type":       "subscribe_events",
		"event_type": "state_changed",
	}); err != nil {
		return err
	}

	if onStatus != nil {
		onStatus(true)
	}

	for {
		var envelope struct {
			Type  string `json:"type"`
			Event *struct {
				EventType string `json:"event_type"`
				Data      struct {
					NewState *EntityState `json:"new_state"`
				} `json:"data"`
			} `json:"event"`
		}
		if err := conn.ReadJSON(&envelope); err != nil {
			return err
		}
		if envelope.Type == "event" &&
			envelope.Event != nil &&
			envelope.Event.EventType == "state_changed" &&
			envelope.Event.Data.NewState != nil {
			ns := *envelope.Event.Data.NewState
			if len(entityIDs) == 0 || entityIDs[ns.EntityID] {
				if onChange != nil {
					onChange(ns)
				}
			}
		}
	}
}
