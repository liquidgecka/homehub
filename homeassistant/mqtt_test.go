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
	"errors"
	"sync"
	"testing"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"

	"github.com/liquidgecka/homehub/config"
	"github.com/liquidgecka/homehub/database"
	"github.com/liquidgecka/homehub/dpms"
	"github.com/liquidgecka/homehub/reminders"
)

type fakeToken struct {
	err error
}

func (t *fakeToken) Wait() bool                     { return true }
func (t *fakeToken) WaitTimeout(time.Duration) bool { return true }
func (t *fakeToken) Done() <-chan struct{} {
	ch := make(chan struct{})
	close(ch)
	return ch
}
func (t *fakeToken) Error() error { return t.err }

type publishedMsg struct {
	topic    string
	qos      byte
	retained bool
	payload  string
}

type fakePublisher struct {
	mu   sync.Mutex
	msgs []publishedMsg
}

func (f *fakePublisher) Publish(
	topic string, qos byte, retained bool, payload interface{},
) mqtt.Token {
	var str string
	switch v := payload.(type) {
	case string:
		str = v
	case []byte:
		str = string(v)
	}
	f.mu.Lock()
	f.msgs = append(f.msgs, publishedMsg{
		topic:    topic,
		qos:      qos,
		retained: retained,
		payload:  str,
	})
	f.mu.Unlock()
	return &fakeToken{}
}

func (f *fakePublisher) lastFor(topic string) (publishedMsg, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for i := len(f.msgs) - 1; i >= 0; i-- {
		if f.msgs[i].topic == topic {
			return f.msgs[i], true
		}
	}
	return publishedMsg{}, false
}

func (f *fakePublisher) clear() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.msgs = nil
}

func TestMQTTBridge_PublishDiscoveryAndStates(t *testing.T) {
	dpms.ResetState()
	defer dpms.ResetState()
	ClearActiveViewListeners()
	defer ClearActiveViewListeners()
	reminders.ClearListeners()
	defer reminders.ClearListeners()

	_, cleanup, err := database.NewTestDB()
	if err != nil {
		t.Fatalf("NewTestDB failed: %v", err)
	}
	defer cleanup()

	_, err = database.AddReminderDB(database.Reminder{
		Title:         "Take Vitamins",
		Time:          "08:00",
		Days:          "Everyday",
		Enabled:       true,
		Acknowledged:  false,
		LastTriggered: time.Now(),
	})
	if err != nil {
		t.Fatalf("AddReminderDB failed: %v", err)
	}

	dpms.InitAwayMode(true)
	dpms.InitDisplayState(false)
	SetActiveView("shopping")
	defer SetActiveView("home")

	cfg := config.HomeAssistantMQTTConfig{
		Enabled:         true,
		DiscoveryPrefix: "homeassistant",
		TopicPrefix:     "homehub",
		DeviceID:        "homehub_test",
		DeviceName:      "HomeHub Test",
	}
	hooks := Hooks{
		AvailableViews: []string{"home", "shopping", "reminders"},
	}

	bridge := NewMQTTBridge(cfg, hooks)
	pub := &fakePublisher{}
	bridge.SetPublisher(pub)

	bridge.PublishDiscovery()

	expectedDiscTopics := []string{
		"homeassistant/switch/homehub_test/away/config",
		"homeassistant/binary_sensor/homehub_test/display/config",
		"homeassistant/sensor/homehub_test/pending_reminders/config",
		"homeassistant/select/homehub_test/active_view/config",
		"homeassistant/button/homehub_test/restart/config",
		"homeassistant/button/homehub_test/next_photo/config",
		"homeassistant/button/homehub_test/dismiss_alert/config",
	}
	for _, topic := range expectedDiscTopics {
		msg, ok := pub.lastFor(topic)
		if !ok {
			t.Fatalf("expected discovery topic %s to be published", topic)
		}
		if !msg.retained {
			t.Errorf("expected discovery topic %s to be retained", topic)
		}
		var parsed map[string]interface{}
		if err := json.Unmarshal([]byte(msg.payload), &parsed); err != nil {
			t.Errorf("invalid JSON on %s: %v", topic, err)
		}
		if parsed["availability_topic"] != "homehub/availability" {
			t.Errorf("unexpected availability_topic on %s: %v",
				topic, parsed["availability_topic"])
		}
	}

	bridge.PublishAllStates()

	checks := map[string]string{
		"homehub/availability":            "online",
		"homehub/away/state":              "ON",
		"homehub/display/state":           "OFF",
		"homehub/reminders/pending/state": "1",
		"homehub/view/state":              "shopping",
	}
	for topic, want := range checks {
		msg, ok := pub.lastFor(topic)
		if !ok || msg.payload != want {
			t.Errorf("topic %s = %q (ok=%v), want %q",
				topic, msg.payload, ok, want)
		}
	}

	attrMsg, ok := pub.lastFor("homehub/reminders/pending/attributes")
	if !ok {
		t.Fatal("expected pending reminder attributes topic")
	}
	var attrs struct {
		Count  int      `json:"count"`
		Titles []string `json:"titles"`
	}
	if err := json.Unmarshal([]byte(attrMsg.payload), &attrs); err != nil {
		t.Fatalf("failed to decode reminder attributes: %v", err)
	}
	if attrs.Count != 1 || len(attrs.Titles) != 1 ||
		attrs.Titles[0] != "Take Vitamins" {
		t.Errorf("unexpected reminder attributes: %+v", attrs)
	}
}

func TestMQTTBridge_HandleMessage(t *testing.T) {
	dpms.ResetState()
	defer dpms.ResetState()
	ClearAlertHandlers()
	defer ClearAlertHandlers()

	_, cleanup, err := database.NewTestDB()
	if err != nil {
		t.Fatalf("NewTestDB failed: %v", err)
	}
	defer cleanup()

	var restarted bool
	var nextPhoto bool
	var selectedView string
	var alerts []Alert

	RegisterAlertHandler(func(a Alert) {
		alerts = append(alerts, a)
	})

	cfg := config.HomeAssistantMQTTConfig{
		Enabled:         true,
		DiscoveryPrefix: "homeassistant",
		TopicPrefix:     "homehub",
		DeviceID:        "homehub",
		DeviceName:      "HomeHub",
	}
	hooks := Hooks{
		OnRestart:      func() { restarted = true },
		OnNextPhoto:    func() { nextPhoto = true },
		OnSelectView:   func(view string) { selectedView = view },
		AvailableViews: []string{"home", "calendar", "shopping"},
	}

	bridge := NewMQTTBridge(cfg, hooks)
	pub := &fakePublisher{}
	bridge.SetPublisher(pub)

	// Away ON -> persists and flips dpms.
	bridge.HandleMessage("homehub/away/set", []byte("ON"))
	if !dpms.IsAway() || dpms.IsDisplayOn() {
		t.Error("expected away=true and displayOn=false after away/set ON")
	}
	persisted, err := database.GetAwayMode()
	if err != nil || !persisted {
		t.Errorf("expected persisted away=true, got %v, err=%v", persisted, err)
	}

	// View switch ignored while display is off.
	bridge.HandleMessage("homehub/view/set", []byte("calendar"))
	if selectedView != "" {
		t.Errorf("expected view switch ignored while display off, got %q",
			selectedView)
	}

	// Away OFF -> display back on.
	bridge.HandleMessage("homehub/away/set", []byte("OFF"))
	if dpms.IsAway() || !dpms.IsDisplayOn() {
		t.Error("expected away=false and displayOn=true after away/set OFF")
	}

	// Valid and invalid view switches while display is on.
	bridge.HandleMessage("homehub/view/set", []byte("invalid_tab"))
	if selectedView != "" {
		t.Errorf("expected invalid view ignored, got %q", selectedView)
	}
	bridge.HandleMessage("homehub/view/set", []byte("calendar"))
	if selectedView != "calendar" {
		t.Errorf("expected selectedView='calendar', got %q", selectedView)
	}

	// Buttons.
	bridge.HandleMessage("homehub/button/restart/set", []byte("PRESS"))
	if !restarted {
		t.Error("expected OnRestart to be invoked")
	}
	bridge.HandleMessage("homehub/button/next_photo/set", []byte("PRESS"))
	if !nextPhoto {
		t.Error("expected OnNextPhoto to be invoked")
	}

	// Alerts.
	bridge.HandleMessage("homehub/alert/set", []byte(`{"message":"Door open"}`))
	bridge.HandleMessage("homehub/alert/dismiss", []byte("PRESS"))
	if len(alerts) != 2 || alerts[0].Message != "Door open" || !alerts[1].Dismiss {
		t.Errorf("unexpected alerts sequence: %+v", alerts)
	}

	// HA birth message triggers full discovery + state republish.
	pub.clear()
	bridge.HandleMessage("homeassistant/status", []byte("online"))
	if _, ok := pub.lastFor("homeassistant/switch/homehub/away/config"); !ok {
		t.Error("expected discovery republish on HA birth message")
	}
	if msg, ok := pub.lastFor("homehub/availability"); !ok ||
		msg.payload != "online" {
		t.Error("expected online availability republish on HA birth message")
	}

	// DB error on away command does not update dpms.
	origSetAway := database.SetAwayMode
	database.SetAwayMode = func(bool) error { return errors.New("db fail") }
	bridge.HandleMessage("homehub/away/set", []byte("ON"))
	database.SetAwayMode = origSetAway
	if dpms.IsAway() {
		t.Error("expected dpms away mode unchanged on DB persistence failure")
	}
}

func TestActiveViewListeners(t *testing.T) {
	ClearActiveViewListeners()
	defer ClearActiveViewListeners()
	SetActiveView("home")
	defer SetActiveView("home")

	var events []string
	RegisterActiveViewListener(nil)
	RegisterActiveViewListener(func(v string) {
		events = append(events, v)
	})

	SetActiveView("")
	SetActiveView("home")
	SetActiveView("weather")
	SetActiveView("weather")
	SetActiveView("ledger")

	if len(events) != 2 || events[0] != "weather" || events[1] != "ledger" {
		t.Errorf("unexpected active view events: %v", events)
	}
}

func TestMQTTBridge_DefaultsAndLifecycle(t *testing.T) {
	dpms.ResetState()
	defer dpms.ResetState()

	bridge := NewMQTTBridge(config.HomeAssistantMQTTConfig{}, Hooks{
		GetActiveView: func() string { return "reminders" },
	})
	pub := &fakePublisher{}
	bridge.SetPublisher(pub)

	wantTopics := []string{
		"homehub/away/set",
		"homehub/view/set",
		"homehub/button/restart/set",
		"homehub/button/next_photo/set",
		"homehub/alert/set",
		"homehub/alert/dismiss",
		"homeassistant/status",
	}
	gotTopics := bridge.CommandTopics()
	if len(gotTopics) != len(wantTopics) {
		t.Fatalf("CommandTopics len = %d, want %d",
			len(gotTopics), len(wantTopics))
	}
	for i := range wantTopics {
		if gotTopics[i] != wantTopics[i] {
			t.Errorf("topic[%d] = %q, want %q",
				i, gotTopics[i], wantTopics[i])
		}
	}

	bridge.PublishDiscovery()
	selMsg, ok := pub.lastFor(
		"homeassistant/select/homehub/active_view/config",
	)
	if !ok {
		t.Fatal("expected default active_view discovery topic")
	}
	var selCfg map[string]interface{}
	if err := json.Unmarshal([]byte(selMsg.payload), &selCfg); err != nil {
		t.Fatalf("unmarshal select discovery: %v", err)
	}
	opts, ok := selCfg["options"].([]interface{})
	if !ok || len(opts) != 1 || opts[0] != "home" {
		t.Errorf("unexpected default select options: %v", selCfg["options"])
	}

	bridge.PublishActiveViewState()
	if msg, ok := pub.lastFor("homehub/view/state"); !ok ||
		msg.payload != "reminders" {
		t.Errorf("view state = %+v, want reminders", msg)
	}

	bridge.PublishAvailability(false)
	if msg, ok := pub.lastFor("homehub/availability"); !ok ||
		msg.payload != "offline" {
		t.Errorf("availability = %+v, want offline", msg)
	}

	// StartMQTTDiscovery exits immediately when disabled or host is empty.
	ctx := context.Background()
	StartMQTTDiscovery(ctx, &config.Config{}, Hooks{})
	StartMQTTDiscovery(ctx, &config.Config{
		HomeAssistant: config.HomeAssistantConfig{
			MQTT: config.HomeAssistantMQTTConfig{Enabled: true},
		},
	}, Hooks{})
}

func TestMQTTBridge_HandleMessageEdgeCases(t *testing.T) {
	dpms.ResetState()
	defer dpms.ResetState()
	ClearAlertHandlers()
	defer ClearAlertHandlers()

	var alerts []Alert
	RegisterAlertHandler(func(a Alert) {
		alerts = append(alerts, a)
	})

	bridge := NewMQTTBridge(config.HomeAssistantMQTTConfig{}, Hooks{})

	// Invalid away payload leaves away state unchanged.
	bridge.HandleMessage("homehub/away/set", []byte("MAYBE"))
	if dpms.IsAway() {
		t.Error("expected away=false after invalid payload")
	}

	// Invalid JSON alert does not trigger.
	bridge.HandleMessage("homehub/alert/set", []byte("{bad-json"))
	if len(alerts) != 0 {
		t.Errorf("expected no alerts on invalid JSON, got %d", len(alerts))
	}

	// Plain-text alert and 'clear' dismiss command.
	bridge.HandleMessage("homehub/alert/set", []byte("Washer done"))
	bridge.HandleMessage("homehub/alert/set", []byte("clear"))
	if len(alerts) != 2 || alerts[0].Message != "Washer done" ||
		!alerts[1].Dismiss {
		t.Errorf("unexpected alerts: %+v", alerts)
	}
}
