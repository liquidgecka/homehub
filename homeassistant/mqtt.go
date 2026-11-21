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
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"

	"github.com/liquidgecka/homehub/config"
	"github.com/liquidgecka/homehub/database"
	"github.com/liquidgecka/homehub/dpms"
	"github.com/liquidgecka/homehub/logging"
	"github.com/liquidgecka/homehub/reminders"
)

// Hooks connects Home Assistant MQTT commands to main application actions
// without introducing package import cycles.
type Hooks struct {
	OnRestart      func()
	OnNextPhoto    func()
	OnSelectView   func(view string)
	GetActiveView  func() string
	AvailableViews []string
}

var (
	activeViewMu        sync.RWMutex
	activeView          = "home"
	activeViewListeners []func(string)
)

// SetActiveView records the currently displayed touchscreen view and notifies
// registered listeners when it changes.
func SetActiveView(name string) {
	if name == "" {
		return
	}
	activeViewMu.Lock()
	changed := activeView != name
	activeView = name
	var listeners []func(string)
	if changed && len(activeViewListeners) > 0 {
		listeners = make([]func(string), len(activeViewListeners))
		copy(listeners, activeViewListeners)
	}
	activeViewMu.Unlock()

	if changed {
		for _, fn := range listeners {
			if fn != nil {
				fn(name)
			}
		}
	}
}

// GetActiveView returns the name of the currently active touchscreen view.
func GetActiveView() string {
	activeViewMu.RLock()
	defer activeViewMu.RUnlock()
	return activeView
}

// RegisterActiveViewListener registers a callback invoked when the active
// touchscreen view changes.
func RegisterActiveViewListener(fn func(string)) {
	if fn == nil {
		return
	}
	activeViewMu.Lock()
	defer activeViewMu.Unlock()
	activeViewListeners = append(activeViewListeners, fn)
}

// ClearActiveViewListeners removes all registered active view listeners.
func ClearActiveViewListeners() {
	activeViewMu.Lock()
	defer activeViewMu.Unlock()
	activeViewListeners = nil
}

type mqttPublisher interface {
	Publish(
		topic string, qos byte, retained bool, payload interface{},
	) mqtt.Token
}

// MQTTBridge manages Home Assistant MQTT Discovery publishing and command
// routing for HomeHub.
type MQTTBridge struct {
	cfg   config.HomeAssistantMQTTConfig
	hooks Hooks

	mu  sync.RWMutex
	pub mqttPublisher
}

// NewMQTTBridge constructs a new Home Assistant MQTT discovery bridge.
func NewMQTTBridge(
	cfg config.HomeAssistantMQTTConfig, hooks Hooks,
) *MQTTBridge {
	if cfg.DiscoveryPrefix == "" {
		cfg.DiscoveryPrefix = "homeassistant"
	}
	if cfg.TopicPrefix == "" {
		cfg.TopicPrefix = "homehub"
	}
	if cfg.DeviceID == "" {
		cfg.DeviceID = "homehub"
	}
	if cfg.DeviceName == "" {
		cfg.DeviceName = "HomeHub"
	}
	return &MQTTBridge{
		cfg:   cfg,
		hooks: hooks,
	}
}

// SetPublisher assigns the MQTT publisher used by the bridge.
func (b *MQTTBridge) SetPublisher(pub mqttPublisher) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.pub = pub
}

func (b *MQTTBridge) publish(
	topic string, qos byte, retained bool, payload interface{},
) {
	b.mu.RLock()
	pub := b.pub
	b.mu.RUnlock()
	if pub == nil {
		return
	}
	tok := pub.Publish(topic, qos, retained, payload)
	if tok != nil {
		tok.WaitTimeout(5 * time.Second)
		if err := tok.Error(); err != nil {
			log.Printf("HA MQTT: Publish error on %s: %v", topic, err)
		}
	}
}

func (b *MQTTBridge) topicPrefix() string {
	if b.cfg.TopicPrefix != "" {
		return strings.Trim(b.cfg.TopicPrefix, "/")
	}
	return "homehub"
}

func (b *MQTTBridge) discoveryPrefix() string {
	if b.cfg.DiscoveryPrefix != "" {
		return strings.Trim(b.cfg.DiscoveryPrefix, "/")
	}
	return "homeassistant"
}

func (b *MQTTBridge) deviceID() string {
	if b.cfg.DeviceID != "" {
		return b.cfg.DeviceID
	}
	return "homehub"
}

func (b *MQTTBridge) deviceName() string {
	if b.cfg.DeviceName != "" {
		return b.cfg.DeviceName
	}
	return "HomeHub"
}

func (b *MQTTBridge) availableViews() []string {
	if len(b.hooks.AvailableViews) > 0 {
		return b.hooks.AvailableViews
	}
	return []string{"home"}
}

// AvailabilityTopic returns the LWT/online availability topic for HomeHub.
func (b *MQTTBridge) AvailabilityTopic() string {
	return b.topicPrefix() + "/availability"
}

// CommandTopics returns all MQTT topics the bridge subscribes to.
func (b *MQTTBridge) CommandTopics() []string {
	prefix := b.topicPrefix()
	return []string{
		prefix + "/away/set",
		prefix + "/view/set",
		prefix + "/button/restart/set",
		prefix + "/button/next_photo/set",
		prefix + "/alert/set",
		prefix + "/alert/dismiss",
		b.discoveryPrefix() + "/status",
	}
}

func (b *MQTTBridge) publishDiscoveryEntity(
	domain, objectID string, payload map[string]interface{},
) {
	data, err := json.Marshal(payload)
	if err != nil {
		log.Printf(
			"HA MQTT: Failed to marshal discovery for %s/%s: %v",
			domain, objectID, err,
		)
		return
	}
	topic := fmt.Sprintf(
		"%s/%s/%s/%s/config",
		b.discoveryPrefix(), domain, b.deviceID(), objectID,
	)
	b.publish(topic, 1, true, data)
}

// PublishDiscovery publishes retained Home Assistant MQTT Discovery payloads
// for all HomeHub entities.
func (b *MQTTBridge) PublishDiscovery() {
	prefix := b.topicPrefix()
	devID := b.deviceID()
	availTopic := b.AvailabilityTopic()

	device := map[string]interface{}{
		"identifiers":  []string{devID},
		"name":         b.deviceName(),
		"manufacturer": "HomeHub",
		"model":        "Touchscreen Dashboard",
	}

	b.publishDiscoveryEntity("switch", "away", map[string]interface{}{
		"name":               "Away Mode",
		"unique_id":          devID + "_away",
		"state_topic":        prefix + "/away/state",
		"command_topic":      prefix + "/away/set",
		"payload_on":         "ON",
		"payload_off":        "OFF",
		"state_on":           "ON",
		"state_off":          "OFF",
		"icon":               "mdi:home-export-outline",
		"availability_topic": availTopic,
		"device":             device,
	})

	b.publishDiscoveryEntity("binary_sensor", "display", map[string]interface{}{
		"name":               "Display",
		"unique_id":          devID + "_display",
		"state_topic":        prefix + "/display/state",
		"payload_on":         "ON",
		"payload_off":        "OFF",
		"device_class":       "power",
		"availability_topic": availTopic,
		"device":             device,
	})

	b.publishDiscoveryEntity(
		"sensor", "pending_reminders", map[string]interface{}{
			"name":                  "Pending Reminders",
			"unique_id":             devID + "_pending_reminders",
			"state_topic":           prefix + "/reminders/pending/state",
			"json_attributes_topic": prefix + "/reminders/pending/attributes",
			"icon":                  "mdi:bell-ring-outline",
			"unit_of_measurement":   "reminders",
			"availability_topic":    availTopic,
			"device":                device,
		},
	)

	b.publishDiscoveryEntity("select", "active_view", map[string]interface{}{
		"name":               "Active View",
		"unique_id":          devID + "_active_view",
		"state_topic":        prefix + "/view/state",
		"command_topic":      prefix + "/view/set",
		"options":            b.availableViews(),
		"icon":               "mdi:monitor-dashboard",
		"availability_topic": availTopic,
		"device":             device,
	})

	b.publishDiscoveryEntity("button", "restart", map[string]interface{}{
		"name":               "Restart",
		"unique_id":          devID + "_restart",
		"command_topic":      prefix + "/button/restart/set",
		"payload_press":      "PRESS",
		"device_class":       "restart",
		"availability_topic": availTopic,
		"device":             device,
	})

	b.publishDiscoveryEntity("button", "next_photo", map[string]interface{}{
		"name":               "Next Photo",
		"unique_id":          devID + "_next_photo",
		"command_topic":      prefix + "/button/next_photo/set",
		"payload_press":      "PRESS",
		"icon":               "mdi:image-move",
		"availability_topic": availTopic,
		"device":             device,
	})

	b.publishDiscoveryEntity(
		"button", "dismiss_alert", map[string]interface{}{
			"name":               "Dismiss Alert",
			"unique_id":          devID + "_dismiss_alert",
			"command_topic":      prefix + "/alert/dismiss",
			"payload_press":      "PRESS",
			"icon":               "mdi:bell-cancel-outline",
			"availability_topic": availTopic,
			"device":             device,
		},
	)
}

// PublishAvailability publishes online/offline availability to MQTT.
func (b *MQTTBridge) PublishAvailability(online bool) {
	payload := "offline"
	if online {
		payload = "online"
	}
	b.publish(b.AvailabilityTopic(), 1, true, payload)
}

// PublishAwayState publishes the current Away Mode state.
func (b *MQTTBridge) PublishAwayState() {
	state := "OFF"
	if dpms.IsAway() {
		state = "ON"
	}
	b.publish(b.topicPrefix()+"/away/state", 1, true, state)
}

// PublishDisplayState publishes the current display power state.
func (b *MQTTBridge) PublishDisplayState() {
	state := "OFF"
	if dpms.IsDisplayOn() {
		state = "ON"
	}
	b.publish(b.topicPrefix()+"/display/state", 1, true, state)
}

// PublishRemindersState publishes the count and titles of unacknowledged
// reminders.
func (b *MQTTBridge) PublishRemindersState() {
	pending, err := reminders.GetPendingReminders()
	if err != nil {
		logging.Debugf("HA MQTT: Skipping reminders state publish: %v", err)
		return
	}
	titles := make([]string, 0, len(pending))
	for _, r := range pending {
		titles = append(titles, r.Title)
	}
	attrs, err := json.Marshal(map[string]interface{}{
		"count":  len(pending),
		"titles": titles,
	})
	if err != nil {
		log.Printf("HA MQTT: Failed to marshal reminder attributes: %v", err)
		return
	}
	prefix := b.topicPrefix()
	b.publish(
		prefix+"/reminders/pending/state", 1, true, fmt.Sprintf("%d", len(pending)),
	)
	b.publish(prefix+"/reminders/pending/attributes", 1, true, attrs)
}

// PublishActiveViewState publishes the active touchscreen view name.
func (b *MQTTBridge) PublishActiveViewState() {
	view := GetActiveView()
	if b.hooks.GetActiveView != nil {
		if v := b.hooks.GetActiveView(); v != "" {
			view = v
		}
	}
	if view == "" {
		view = "home"
	}
	b.publish(b.topicPrefix()+"/view/state", 1, true, view)
}

// PublishAllStates publishes retained states for availability and all entities.
func (b *MQTTBridge) PublishAllStates() {
	b.PublishAvailability(true)
	b.PublishAwayState()
	b.PublishDisplayState()
	b.PublishRemindersState()
	b.PublishActiveViewState()
}

// HandleMessage routes incoming MQTT command messages from Home Assistant.
func (b *MQTTBridge) HandleMessage(topic string, payload []byte) {
	prefix := b.topicPrefix()
	msgStr := strings.TrimSpace(string(payload))

	switch topic {
	case prefix + "/away/set":
		var away bool
		switch strings.ToUpper(msgStr) {
		case "ON", "TRUE", "1":
			away = true
		case "OFF", "FALSE", "0":
			away = false
		default:
			log.Printf("HA MQTT: Invalid away command payload: %q", msgStr)
			return
		}
		if err := database.SetAwayMode(away); err != nil {
			log.Printf("HA MQTT: Failed to persist away mode: %v", err)
			return
		}
		dpms.SetAwayMode(away)

	case prefix + "/view/set":
		if !dpms.IsDisplayOn() {
			logging.Debugf(
				"HA MQTT: Ignoring view switch %q while display is off.", msgStr,
			)
			return
		}
		valid := false
		for _, v := range b.availableViews() {
			if v == msgStr {
				valid = true
				break
			}
		}
		if !valid {
			log.Printf("HA MQTT: Unknown view requested: %q", msgStr)
			return
		}
		if b.hooks.OnSelectView != nil {
			b.hooks.OnSelectView(msgStr)
		}

	case prefix + "/button/restart/set":
		if b.hooks.OnRestart != nil {
			b.hooks.OnRestart()
		}

	case prefix + "/button/next_photo/set":
		if b.hooks.OnNextPhoto != nil {
			b.hooks.OnNextPhoto()
		}

	case prefix + "/alert/set":
		alert, err := ParseAlertPayload(payload)
		if err != nil {
			log.Printf("HA MQTT: Invalid alert payload: %v", err)
			return
		}
		if alert.Dismiss {
			DismissAlert()
			return
		}
		TriggerAlert(alert)

	case prefix + "/alert/dismiss":
		DismissAlert()

	case b.discoveryPrefix() + "/status":
		if strings.EqualFold(msgStr, "online") {
			log.Println(
				"HA MQTT: Home Assistant birth message received, republishing.",
			)
			b.PublishDiscovery()
			b.PublishAllStates()
		}
	}
}

// StartMQTTDiscovery connects to the configured MQTT broker, publishes Home
// Assistant MQTT Discovery configs, and syncs HomeHub state until ctx closes.
func StartMQTTDiscovery(
	ctx context.Context, cfg *config.Config, hooks Hooks,
) {
	if cfg == nil {
		cfg = config.GetConfig()
	}
	mqttCfg := cfg.ResolveHomeAssistantMQTT()
	if !mqttCfg.Enabled {
		log.Println("HA MQTT: Discovery is disabled in config.toml.")
		return
	}
	if mqttCfg.Host == "" {
		log.Println("HA MQTT: No MQTT host configured, skipping discovery.")
		return
	}

	bridge := NewMQTTBridge(mqttCfg, hooks)
	brokerURL := fmt.Sprintf("tcp://%s:%d", mqttCfg.Host, mqttCfg.Port)
	log.Printf("HA MQTT: Initializing discovery client for %s", brokerURL)

	opts := mqtt.NewClientOptions()
	opts.AddBroker(brokerURL)
	opts.SetClientID(bridge.deviceID() + "-ha")
	opts.SetUsername(mqttCfg.Username)
	opts.SetPassword(mqttCfg.Password)
	opts.SetWill(bridge.AvailabilityTopic(), "offline", 1, true)
	opts.SetConnectTimeout(10 * time.Second)
	opts.SetDefaultPublishHandler(func(client mqtt.Client, msg mqtt.Message) {
		bridge.HandleMessage(msg.Topic(), msg.Payload())
	})
	opts.OnConnectionLost = func(client mqtt.Client, err error) {
		log.Printf("HA MQTT: Connection lost: %v", err)
	}
	opts.OnConnect = func(client mqtt.Client) {
		log.Println("HA MQTT: Connected to broker, subscribing and publishing.")
		topics := make(map[string]byte)
		for _, t := range bridge.CommandTopics() {
			topics[t] = 1
		}
		tok := client.SubscribeMultiple(topics, func(
			c mqtt.Client, msg mqtt.Message,
		) {
			bridge.HandleMessage(msg.Topic(), msg.Payload())
		})
		if tok.WaitTimeout(5*time.Second) && tok.Error() != nil {
			log.Printf("HA MQTT: Subscription error: %v", tok.Error())
		}
		bridge.PublishDiscovery()
		bridge.PublishAllStates()
	}

	client := mqtt.NewClient(opts)
	bridge.SetPublisher(client)

	dpms.RegisterAwayListener(func(away bool) {
		if ctx.Err() == nil && client.IsConnected() {
			bridge.PublishAwayState()
		}
	})
	dpms.RegisterStateListener(func(on bool) {
		if ctx.Err() == nil && client.IsConnected() {
			bridge.PublishDisplayState()
		}
	})
	reminders.RegisterChangeListener(func() {
		if ctx.Err() == nil && client.IsConnected() {
			bridge.PublishRemindersState()
		}
	})
	RegisterActiveViewListener(func(view string) {
		if ctx.Err() == nil && client.IsConnected() {
			bridge.PublishActiveViewState()
		}
	})

	const maxBackoff = 5 * time.Minute
	backoff := 2 * time.Second

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		if tok := client.Connect(); tok.Wait() && tok.Error() != nil {
			log.Printf(
				"HA MQTT: Connect failed: %v. Retrying in %s...",
				tok.Error(), backoff,
			)
			select {
			case <-time.After(backoff):
				backoff *= 2
				if backoff > maxBackoff {
					backoff = maxBackoff
				}
				continue
			case <-ctx.Done():
				return
			}
		}

		backoff = 2 * time.Second
		for client.IsConnected() {
			select {
			case <-ctx.Done():
				bridge.PublishAvailability(false)
				client.Disconnect(250)
				log.Println("HA MQTT: Client disconnected.")
				return
			case <-time.After(1 * time.Second):
			}
		}
	}
}
