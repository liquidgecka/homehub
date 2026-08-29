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
	"testing"
)

type mockMessage struct {
	topic   string
	payload []byte
}

func (m *mockMessage) Duplicate() bool   { return false }
func (m *mockMessage) Qos() byte         { return 0 }
func (m *mockMessage) Retained() bool    { return false }
func (m *mockMessage) Topic() string     { return m.topic }
func (m *mockMessage) MessageID() uint16 { return 0 }
func (m *mockMessage) Payload() []byte   { return m.payload }
func (m *mockMessage) Ack()              {}

func TestMessagePubHandler(t *testing.T) {
	// Drain any existing items
	for len(CameraEventChan) > 0 {
		<-CameraEventChan
	}

	// Test "new" event
	newPayload := []byte(
		`{"type":"new","before":{"camera":"driveway","type":""},` +
			`"after":{"camera":"driveway","type":"person"}}`,
	)
	messagePubHandler(
		nil, &mockMessage{topic: "frigate/events", payload: newPayload},
	)

	select {
	case ev := <-CameraEventChan:
		if ev.Camera != "driveway" || ev.Type != "start" {
			t.Errorf("Unexpected event received: %+v", ev)
		}
	default:
		t.Errorf("Expected start event in CameraEventChan")
	}

	// Test "update" event (should not emit to CameraEventChan or error)
	updatePayload := []byte(
		`{"type":"update","before":{"camera":"driveway","type":"person"},` +
			`"after":{"camera":"driveway","type":"person"}}`,
	)
	messagePubHandler(
		nil, &mockMessage{topic: "frigate/events", payload: updatePayload},
	)

	select {
	case ev := <-CameraEventChan:
		t.Errorf("Did not expect event in CameraEventChan for update: %+v", ev)
	default:
		// expected
	}

	// Test "end" event
	endPayload := []byte(
		`{"type":"end","before":{"camera":"driveway","type":"person"},` +
			`"after":{"camera":"driveway","type":""}}`,
	)
	messagePubHandler(
		nil, &mockMessage{topic: "frigate/events", payload: endPayload},
	)

	select {
	case ev := <-CameraEventChan:
		if ev.Camera != "driveway" || ev.Type != "end" {
			t.Errorf("Unexpected event received: %+v", ev)
		}
	default:
		t.Errorf("Expected end event in CameraEventChan")
	}

	// Test invalid JSON
	invalidPayload := []byte(`not json`)
	messagePubHandler(
		nil, &mockMessage{topic: "frigate/events", payload: invalidPayload},
	)
}
