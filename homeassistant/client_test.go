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
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func TestEntityStateHelpers(t *testing.T) {
	s := EntityState{
		EntityID: "light.kitchen",
		State:    "on",
		Attributes: map[string]interface{}{
			"friendly_name": "Kitchen Ceiling",
			"brightness":    128.0,
		},
	}
	if s.Domain() != "light" {
		t.Errorf("Domain() = %q, want light", s.Domain())
	}
	if s.FriendlyName("") != "Kitchen Ceiling" {
		t.Errorf("FriendlyName('') = %q", s.FriendlyName(""))
	}
	if s.FriendlyName("Custom") != "Custom" {
		t.Errorf("FriendlyName('Custom') = %q", s.FriendlyName("Custom"))
	}
	if !s.IsActive() {
		t.Error("expected light on to be active")
	}
	if b, ok := s.FloatAttr("brightness"); !ok || b != 128.0 {
		t.Errorf("FloatAttr = %v, %v", b, ok)
	}
}

func TestClientRESTAndWebSocket(t *testing.T) {
	var mu sync.Mutex
	var serviceCalls []string
	var lastTemp float64

	upgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool { return true },
	}

	srv := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/api/websocket" {
				conn, err := upgrader.Upgrade(w, r, nil)
				if err != nil {
					return
				}
				defer conn.Close()

				_ = conn.WriteJSON(map[string]string{"type": "auth_required"})
				var auth struct {
					AccessToken string `json:"access_token"`
				}
				if err := conn.ReadJSON(&auth); err != nil ||
					auth.AccessToken != "test-token" {
					_ = conn.WriteJSON(map[string]string{"type": "auth_invalid"})
					return
				}
				_ = conn.WriteJSON(map[string]string{"type": "auth_ok"})

				var sub map[string]interface{}
				if err := conn.ReadJSON(&sub); err != nil {
					return
				}
				_ = conn.WriteJSON(map[string]interface{}{
					"id":      1,
					"type":    "result",
					"success": true,
				})

				// Push a live state change for a subscribed entity and an
				// unsubscribed one.
				_ = conn.WriteJSON(map[string]interface{}{
					"type": "event",
					"event": map[string]interface{}{
						"event_type": "state_changed",
						"data": map[string]interface{}{
							"new_state": map[string]interface{}{
								"entity_id": "light.ignored",
								"state":     "on",
							},
						},
					},
				})
				_ = conn.WriteJSON(map[string]interface{}{
					"type": "event",
					"event": map[string]interface{}{
						"event_type": "state_changed",
						"data": map[string]interface{}{
							"new_state": map[string]interface{}{
								"entity_id": "light.kitchen",
								"state":     "off",
							},
						},
					},
				})
				for {
					if _, _, err := conn.ReadMessage(); err != nil {
						return
					}
				}
			}

			if r.Header.Get("Authorization") != "Bearer test-token" {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}

			switch r.URL.Path {
			case "/api/states":
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode([]EntityState{
					{
						EntityID: "light.kitchen",
						State:    "on",
						Attributes: map[string]interface{}{
							"friendly_name": "Kitchen",
						},
					},
					{
						EntityID: "lock.front_door",
						State:    "locked",
					},
				})
			default:
				var body map[string]interface{}
				_ = json.NewDecoder(r.Body).Decode(&body)
				mu.Lock()
				serviceCalls = append(serviceCalls, r.URL.Path)
				if temp, ok := body["temperature"].(float64); ok {
					lastTemp = temp
				}
				mu.Unlock()
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`[]`))
			}
		},
	))
	defer srv.Close()

	client := NewClient(srv.URL, "test-token")
	ctx := context.Background()

	states, err := client.GetStates(ctx)
	if err != nil {
		t.Fatalf("GetStates failed: %v", err)
	}
	if len(states) != 2 || states["light.kitchen"].State != "on" {
		t.Fatalf("unexpected states: %+v", states)
	}

	if err := client.ToggleOrActivate(
		ctx, states["light.kitchen"],
	); err != nil {
		t.Fatalf("Toggle light failed: %v", err)
	}
	if err := client.ToggleOrActivate(
		ctx, states["lock.front_door"],
	); err != nil {
		t.Fatalf("Toggle lock failed: %v", err)
	}
	if err := client.ToggleOrActivate(
		ctx, EntityState{EntityID: "scene.movie"},
	); err != nil {
		t.Fatalf("Activate scene failed: %v", err)
	}
	if err := client.ToggleOrActivate(
		ctx, EntityState{EntityID: "cover.garage", State: "open"},
	); err != nil {
		t.Fatalf("Toggle cover failed: %v", err)
	}
	if err := client.SetClimateTemperature(
		ctx, "climate.main", 71,
	); err != nil {
		t.Fatalf("SetClimateTemperature failed: %v", err)
	}

	mu.Lock()
	gotCalls := append([]string{}, serviceCalls...)
	gotTemp := lastTemp
	mu.Unlock()

	wantCalls := []string{
		"/api/services/light/toggle",
		"/api/services/lock/unlock",
		"/api/services/scene/turn_on",
		"/api/services/cover/close_cover",
		"/api/services/climate/set_temperature",
	}
	if len(gotCalls) != len(wantCalls) {
		t.Fatalf("got calls %v, want %v", gotCalls, wantCalls)
	}
	for i := range wantCalls {
		if gotCalls[i] != wantCalls[i] {
			t.Errorf("call[%d] = %s, want %s", i, gotCalls[i], wantCalls[i])
		}
	}
	if gotTemp != 71 {
		t.Errorf("lastTemp = %v, want 71", gotTemp)
	}

	// Test WebSocket subscription + clean cancellation.
	wsCtx, wsCancel := context.WithCancel(context.Background())
	defer wsCancel()
	changes := make(chan EntityState, 4)
	done := make(chan struct{})
	go func() {
		client.SubscribeStateChanges(
			wsCtx,
			map[string]bool{"light.kitchen": true},
			func(s EntityState) { changes <- s },
			nil,
		)
		close(done)
	}()

	select {
	case ev := <-changes:
		if ev.EntityID != "light.kitchen" || ev.State != "off" {
			t.Errorf("unexpected WS event: %+v", ev)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for WS state_changed event")
	}

	wsCancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("SubscribeStateChanges did not exit after cancel")
	}
}
