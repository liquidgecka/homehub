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
	"testing"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"

	"github.com/liquidgecka/homehub/activity"
	"github.com/liquidgecka/homehub/dpms"
)

func findButtonByText(obj fyne.CanvasObject, text string) *widget.Button {
	if btn, ok := obj.(*widget.Button); ok && btn.Text == text {
		return btn
	}
	if cont, ok := obj.(*fyne.Container); ok {
		for _, child := range cont.Objects {
			if found := findButtonByText(child, text); found != nil {
				return found
			}
		}
	}
	return nil
}

func waitVisibleState(t *testing.T, ch <-chan bool, want bool) {
	t.Helper()
	select {
	case got := <-ch:
		if got != want {
			t.Fatalf("overlay visible state = %v, want %v", got, want)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("timed out waiting for overlay visible=%v", want)
	}
}

func TestParseAlertPayload(t *testing.T) {
	t.Run("Empty payload errors", func(t *testing.T) {
		if _, err := ParseAlertPayload([]byte("   ")); err == nil {
			t.Fatal("expected error for empty payload")
		}
	})

	t.Run("Plain text alert", func(t *testing.T) {
		a, err := ParseAlertPayload([]byte("Washer cycle finished"))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if a.Title != "Home Assistant" ||
			a.Message != "Washer cycle finished" ||
			a.Level != "info" ||
			a.Duration != DefaultAlertDuration ||
			a.Dismiss {
			t.Errorf("unexpected parsed alert: %+v", a)
		}
	})

	t.Run("Plain text dismiss", func(t *testing.T) {
		a, err := ParseAlertPayload([]byte("dismiss"))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !a.Dismiss {
			t.Error("expected Dismiss=true")
		}
	})

	t.Run("JSON payload with level and duration", func(t *testing.T) {
		payload := []byte(
			`{"title":"Garage","message":"Left open",` +
				`"level":"critical","duration_seconds":999}`,
		)
		a, err := ParseAlertPayload(payload)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if a.Title != "Garage" || a.Message != "Left open" ||
			a.Level != "critical" || a.Duration != MaxAlertDuration {
			t.Errorf("unexpected parsed JSON alert: %+v", a)
		}
	})

	t.Run("JSON dismiss payload", func(t *testing.T) {
		a, err := ParseAlertPayload([]byte(`{"dismiss":true}`))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !a.Dismiss {
			t.Error("expected Dismiss=true")
		}
	})

	t.Run("JSON missing title and message errors", func(t *testing.T) {
		if _, err := ParseAlertPayload([]byte(`{"level":"info"}`)); err == nil {
			t.Fatal("expected error when both title and message are missing")
		}
	})
}

func TestCreateAlertOverlayView(t *testing.T) {
	dpms.ResetState()
	defer dpms.ResetState()
	ClearAlertHandlers()
	defer ClearAlertHandlers()

	origReset := activity.ResetTimer
	defer func() { activity.ResetTimer = origReset }()
	activity.ResetTimer = func() {}

	updates := make(chan bool, 10)
	origHook := onAlertOverlayUpdate
	onAlertOverlayUpdate = func(visible bool) {
		updates <- visible
	}
	defer func() { onAlertOverlayUpdate = origHook }()

	test.NewApp()
	overlay := CreateAlertOverlayView()
	if overlay == nil {
		t.Fatal("CreateAlertOverlayView returned nil")
	}
	if overlay.Visible() {
		t.Fatal("expected alert overlay to start hidden")
	}

	// Ignored when display is off.
	dpms.SetDisplayOn(false)
	TriggerAlert(Alert{
		Title:    "Ignored",
		Message:  "Should stay hidden",
		Level:    "warning",
		Duration: 5 * time.Second,
	})
	select {
	case v := <-updates:
		t.Fatalf("did not expect overlay update while display off, got %v", v)
	case <-time.After(30 * time.Millisecond):
	}

	// Shown when display is on.
	dpms.SetDisplayOn(true)
	TriggerAlert(Alert{
		Title:    "Front Door",
		Message:  "Package delivered",
		Level:    "warning",
		Duration: 10 * time.Second,
	})
	waitVisibleState(t, updates, true)

	// Dismiss via DismissAlert().
	DismissAlert()
	waitVisibleState(t, updates, false)

	// Show again and dismiss via button tap.
	TriggerAlert(Alert{
		Title:    "Reminder",
		Message:  "Check oven",
		Level:    "critical",
		Duration: 10 * time.Second,
	})
	waitVisibleState(t, updates, true)
	btn := findButtonByText(overlay, "Dismiss")
	if btn == nil {
		t.Fatal("expected Dismiss button in alert overlay")
	}
	btn.OnTapped()
	waitVisibleState(t, updates, false)

	// Auto-dismisses after duration.
	TriggerAlert(Alert{
		Title:    "Quick",
		Message:  "Auto hide",
		Level:    "info",
		Duration: 60 * time.Millisecond,
	})
	waitVisibleState(t, updates, true)
	waitVisibleState(t, updates, false)

	// Hides immediately when DPMS turns off.
	TriggerAlert(Alert{
		Title:    "Door",
		Message:  "Opened",
		Duration: 5 * time.Second,
	})
	waitVisibleState(t, updates, true)
	dpms.SetDisplayOn(false)
	waitVisibleState(t, updates, false)
}
