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

package security

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"

	"github.com/liquidgecka/homehub/config"
	"github.com/liquidgecka/homehub/logging"
)

// FrigateEvent represents the structure of an event from Frigate's MQTT topic.
type FrigateEvent struct {
	Before struct {
		Camera string `json:"camera"`
		Type   string `json:"type"`
	} `json:"before"`
	After struct {
		Camera string `json:"camera"`
		Type   string `json:"type"`
	} `json:"after"`
	Type string `json:"type"`
}

// CameraEvent is a simplified event structure for UI communication.
type CameraEvent struct {
	Camera string
	Type   string // "start" or "end"
}

// CameraEventChan is a channel to send camera events to the UI.
var CameraEventChan = make(chan CameraEvent, 10)

var messagePubHandler mqtt.MessageHandler = func(
	client mqtt.Client, msg mqtt.Message,
) {
	logging.Debugf("Received MQTT message on topic: %s", msg.Topic())
	logging.Debugf("Raw MQTT Payload: %s", msg.Payload())

	var event FrigateEvent
	if err := json.Unmarshal(msg.Payload(), &event); err != nil {
		log.Printf("Error unmarshalling frigate event: %v", err)
		return
	}

	logging.Debugf("Parsed Frigate Event: %+v", event)

	switch event.Type {
	case "new":
		log.Printf(
			"Frigate event detected: new object on camera %s. "+
				"Sending 'start' event to UI.",
			event.After.Camera,
		)
		select {
		case CameraEventChan <- CameraEvent{
			Camera: event.After.Camera, Type: "start",
		}:
		default:
			log.Printf(
				"Warning: CameraEventChan full, dropping start event for %s",
				event.After.Camera,
			)
		}
	case "end":
		log.Printf(
			"Frigate event detected: object tracking ended on camera %s. "+
				"Sending 'end' event to UI.",
			event.Before.Camera,
		)
		select {
		case CameraEventChan <- CameraEvent{
			Camera: event.Before.Camera, Type: "end",
		}:
		default:
			log.Printf(
				"Warning: CameraEventChan full, dropping end event for %s",
				event.Before.Camera,
			)
		}
	case "update":
		// Frigate publishes 'update' events continuously while an object is
		// being tracked in frame.
		logging.Debugf(
			"Frigate event update: tracking active on camera %s",
			event.After.Camera,
		)
	default:
		logging.Debugf(
			"Unhandled frigate event type: '%s' for camera '%s'",
			event.Type, event.After.Camera,
		)
	}
}

var connectHandler mqtt.OnConnectHandler = func(client mqtt.Client) {
	log.Println("Successfully connected to MQTT broker.")
	token := client.Subscribe("frigate/events", 1, nil)
	if token.Wait() && token.Error() != nil {
		log.Printf(
			"Error subscribing to frigate/events topic: %v",
			token.Error(),
		)
	} else {
		log.Println("Successfully subscribed to 'frigate/events' topic.")
	}
}

var connectLostHandler mqtt.ConnectionLostHandler = func(
	client mqtt.Client, err error,
) {
	log.Printf("Connection to MQTT broker lost: %v", err)
}

func StartMqttListener(ctx context.Context) {
	cfg := config.GetConfig().Security.FrigateMQTT
	if cfg.Host == "" {
		log.Println(
			"MQTT host not configured in config.toml, skipping listener.",
		)
		return
	}

	log.Printf(
		"Initializing MQTT client for broker: %s:%d",
		cfg.Host, cfg.Port,
	)

	opts := mqtt.NewClientOptions()
	opts.AddBroker(fmt.Sprintf("tcp://%s:%d", cfg.Host, cfg.Port))
	opts.SetClientID("homehub-client")
	opts.SetUsername(cfg.Username)
	opts.SetPassword(cfg.Password)
	opts.SetDefaultPublishHandler(messagePubHandler)
	opts.OnConnect = connectHandler
	opts.OnConnectionLost = connectLostHandler
	opts.SetConnectTimeout(10 * time.Second)

	client := mqtt.NewClient(opts)

	go func() {
		defer log.Println("MQTT connection manager shut down.")
		const maxBackoff = 5 * time.Minute
		backoff := 2 * time.Second

		for {
			select {
			case <-ctx.Done():
				return
			default:
			}

			log.Println("Attempting to connect to MQTT broker...")
			if token := client.Connect(); token.Wait() && token.Error() != nil {
				log.Printf(
					"MQTT connection failed: %v. Retrying in %s...",
					token.Error(), backoff,
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

			// Connection is successful, reset backoff and wait for disconnection
			backoff = 2 * time.Second
			for client.IsConnected() {
				select {
				case <-ctx.Done():
					return
				case <-time.After(1 * time.Second):
					// Continue periodic check
				}
			}
			// If we exit this loop, connection was lost.
		}
	}()

	// Wait for the context to be cancelled, then disconnect.
	<-ctx.Done()
	if client.IsConnected() {
		client.Disconnect(250)
	}
	log.Println("MQTT client shutting down and disconnected.")
}
