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

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
	"github.com/gorilla/websocket"

	"github.com/liquidgecka/homehub/activity"
	"github.com/liquidgecka/homehub/config"
)

func findTiles(obj fyne.CanvasObject) []*smartHomeTile {
	var out []*smartHomeTile
	if tile, ok := obj.(*smartHomeTile); ok {
		return []*smartHomeTile{tile}
	}
	if scroll, ok := obj.(*container.Scroll); ok && scroll.Content != nil {
		return findTiles(scroll.Content)
	}
	if cont, ok := obj.(*fyne.Container); ok {
		for _, child := range cont.Objects {
			out = append(out, findTiles(child)...)
		}
	}
	return out
}

func findClimateButtons(
	obj fyne.CanvasObject,
) (*widget.Button, *widget.Button) {
	var decBtn, incBtn *widget.Button
	var walk func(fyne.CanvasObject)
	walk = func(o fyne.CanvasObject) {
		if btn, ok := o.(*widget.Button); ok {
			if btn.Text == "-" {
				decBtn = btn
			} else if btn.Text == "+" {
				incBtn = btn
			}
			return
		}
		if cont, ok := o.(*fyne.Container); ok {
			for _, child := range cont.Objects {
				walk(child)
			}
		}
	}
	walk(obj)
	return decBtn, incBtn
}

func TestCreateSmartHomeView(t *testing.T) {
	origReset := activity.ResetTimer
	defer func() { activity.ResetTimer = origReset }()
	activity.ResetTimer = func() {}

	updated := make(chan string, 16)
	origHook := onSmartHomeTileUpdated
	onSmartHomeTileUpdated = func(id string) {
		updated <- id
	}
	defer func() { onSmartHomeTileUpdated = origHook }()

	var mu sync.Mutex
	var actions []string
	var setTemp float64
	pushWS := make(chan EntityState, 2)

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
				var auth map[string]interface{}
				_ = conn.ReadJSON(&auth)
				_ = conn.WriteJSON(map[string]string{"type": "auth_ok"})
				var sub map[string]interface{}
				_ = conn.ReadJSON(&sub)

				doneRead := make(chan struct{})
				go func() {
					defer close(doneRead)
					for {
						if _, _, err := conn.ReadMessage(); err != nil {
							return
						}
					}
				}()

				for {
					select {
					case <-doneRead:
						return
					case st := <-pushWS:
						_ = conn.WriteJSON(map[string]interface{}{
							"type": "event",
							"event": map[string]interface{}{
								"event_type": "state_changed",
								"data": map[string]interface{}{
									"new_state": st,
								},
							},
						})
					}
				}
			}

			if r.URL.Path == "/api/states" {
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode([]EntityState{
					{
						EntityID: "light.kitchen",
						State:    "on",
						Attributes: map[string]interface{}{
							"brightness": 255.0,
						},
					},
					{
						EntityID: "climate.hall",
						State:    "heat",
						Attributes: map[string]interface{}{
							"temperature":         70.0,
							"current_temperature": 68.0,
						},
					},
					{
						EntityID: "sensor.temp",
						State:    "72",
						Attributes: map[string]interface{}{
							"unit_of_measurement": "°F",
						},
					},
				})
				return
			}

			var body map[string]interface{}
			_ = json.NewDecoder(r.Body).Decode(&body)
			mu.Lock()
			actions = append(actions, r.URL.Path)
			if tmp, ok := body["temperature"].(float64); ok {
				setTemp = tmp
			}
			mu.Unlock()
			_, _ = w.Write([]byte(`[]`))
		},
	))
	defer srv.Close()

	test.NewApp()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	entities := []config.HomeAssistantEntitySpec{
		{EntityID: "light.kitchen", Name: "Kitchen"},
		{EntityID: "climate.hall", Name: "Hallway"},
		{EntityID: "sensor.temp", Name: "Outdoor"},
	}

	view, stop := createSmartHomeViewWithClient(
		ctx, NewClient(srv.URL, "tok"), entities,
	)
	if view == nil {
		t.Fatal("expected non-nil view")
	}

	// Wait for all 3 initial tile updates.
	seen := make(map[string]bool)
	for len(seen) < 3 {
		select {
		case id := <-updated:
			seen[id] = true
		case <-time.After(2 * time.Second):
			t.Fatalf("timed out waiting for initial tiles, got %v", seen)
		}
	}

	tiles := findTiles(view)
	if len(tiles) != 3 {
		t.Fatalf("expected 3 tiles, got %d", len(tiles))
	}
	if tiles[0].getState().State != "on" {
		t.Errorf("light state = %q, want on", tiles[0].getState().State)
	}
	if tiles[2].status.Text != "72 °F" {
		t.Errorf("sensor status = %q, want '72 °F'", tiles[2].status.Text)
	}

	// Tap light tile and climate '+' button.
	tiles[0].Tapped(&fyne.PointEvent{})
	_, incBtn := findClimateButtons(tiles[1].content)
	if incBtn == nil {
		t.Fatal("expected '+' button on climate tile")
	}
	incBtn.OnTapped()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		mu.Lock()
		n := len(actions)
		mu.Unlock()
		if n >= 2 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	mu.Lock()
	gotActions := append([]string{}, actions...)
	gotTemp := setTemp
	mu.Unlock()
	if len(gotActions) < 2 {
		t.Fatalf("expected 2 service calls, got %v", gotActions)
	}
	if gotTemp != 71 {
		t.Errorf("climate temp = %v, want 71", gotTemp)
	}

	// Push live WebSocket update and verify tile updates.
	pushWS <- EntityState{
		EntityID: "light.kitchen",
		State:    "off",
	}
waitLoop:
	for {
		select {
		case id := <-updated:
			if id == "light.kitchen" && tiles[0].getState().State == "off" {
				break waitLoop
			}
		case <-time.After(2 * time.Second):
			t.Fatal("timed out waiting for live WS update on tile")
		}
	}

	// Stop is idempotent.
	stop()
	stop()
}

func TestSmartHomeTileFormattingAndRenderer(t *testing.T) {
	cases := []struct {
		state      EntityState
		wantStatus string
		wantSub    string
	}{
		{
			EntityState{EntityID: "light.den", State: "on"},
			"On", "Tap to toggle",
		},
		{
			EntityState{
				EntityID:   "light.den",
				State:      "on",
				Attributes: map[string]interface{}{"brightness": 255.0},
			},
			"On · 100%", "Tap to toggle",
		},
		{
			EntityState{EntityID: "light.den", State: "off"},
			"Off", "Tap to toggle",
		},
		{
			EntityState{EntityID: "switch.tv", State: "on"},
			"On", "Tap to toggle",
		},
		{
			EntityState{EntityID: "lock.door", State: "locked"},
			"Locked", "Tap to unlock",
		},
		{
			EntityState{EntityID: "lock.door", State: "unlocked"},
			"Unlocked", "Tap to lock",
		},
		{
			EntityState{EntityID: "cover.garage", State: "closed"},
			"Closed", "Tap to toggle",
		},
		{
			EntityState{EntityID: "scene.bedtime", State: "scening"},
			"Scene", "Tap to activate",
		},
		{
			EntityState{EntityID: "button.ping", State: "unknown_unused"},
			"Button", "Tap to press",
		},
		{
			EntityState{EntityID: "climate.hall", State: "heat"},
			"Heat", "Heat",
		},
		{
			EntityState{EntityID: "sensor.temp", State: "unavailable"},
			"Unavailable", "",
		},
	}

	for _, tc := range cases {
		st, sub := formatTileStatus(tc.state)
		if st != tc.wantStatus || sub != tc.wantSub {
			t.Errorf("formatTileStatus(%s/%s) = (%q, %q), want (%q, %q)",
				tc.state.EntityID, tc.state.State, st, sub,
				tc.wantStatus, tc.wantSub)
		}
		_, _, _ = colorsForEntityState(tc.state)
	}

	// Renderer and non-interactive tap no-op.
	tapped := false
	sensorTile := newSmartHomeTile(
		config.HomeAssistantEntitySpec{EntityID: "sensor.humidity"},
		func(EntityState) { tapped = true },
		nil,
	)
	r := sensorTile.CreateRenderer()
	if r == nil {
		t.Fatal("CreateRenderer returned nil")
	}
	if ms := sensorTile.MinSize(); ms.Width <= 0 || ms.Height <= 0 {
		t.Errorf("unexpected MinSize: %v", ms)
	}
	sensorTile.Tapped(&fyne.PointEvent{})
	sensorTile.TappedSecondary(&fyne.PointEvent{})
	if tapped {
		t.Error("non-interactive tile should not invoke onTap")
	}
}

func TestCreateSmartHomeView_EmptyAndErrors(t *testing.T) {
	origReset := activity.ResetTimer
	defer func() { activity.ResetTimer = origReset }()
	activity.ResetTimer = func() {}

	test.NewApp()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Empty entity list renders cleanly.
	emptyView, emptyStop := createSmartHomeViewWithClient(
		ctx, NewClient("", ""), nil,
	)
	if emptyView == nil {
		t.Fatal("expected non-nil empty view")
	}
	emptyStop()

	// Error response on initial fetch, then success on Refresh tap.
	var mu sync.Mutex
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/api/states" {
				return
			}
			mu.Lock()
			calls++
			c := calls
			mu.Unlock()
			if c == 1 {
				http.Error(w, "boom", http.StatusInternalServerError)
				return
			}
			_, _ = w.Write([]byte(`[{"entity_id":"switch.a","state":"on"}]`))
		},
	))
	defer srv.Close()

	updated := make(chan string, 4)
	origHook := onSmartHomeTileUpdated
	onSmartHomeTileUpdated = func(id string) { updated <- id }
	defer func() { onSmartHomeTileUpdated = origHook }()

	view, stop := createSmartHomeViewWithClient(
		ctx,
		NewClient(srv.URL, "tok"),
		[]config.HomeAssistantEntitySpec{{EntityID: "switch.a"}},
	)
	defer stop()

	// Wait for first failed fetch.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		mu.Lock()
		c := calls
		mu.Unlock()
		if c >= 1 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	var refreshBtn *widget.Button
	var findRefresh func(fyne.CanvasObject)
	findRefresh = func(o fyne.CanvasObject) {
		if b, ok := o.(*widget.Button); ok && b.Text == "Refresh" {
			refreshBtn = b
			return
		}
		if cont, ok := o.(*fyne.Container); ok {
			for _, child := range cont.Objects {
				findRefresh(child)
			}
		}
	}
	findRefresh(view)
	if refreshBtn == nil {
		t.Fatal("expected Refresh button")
	}
	refreshBtn.OnTapped()

	select {
	case id := <-updated:
		if id != "switch.a" {
			t.Errorf("updated id = %q, want switch.a", id)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for Refresh fetch")
	}
}
