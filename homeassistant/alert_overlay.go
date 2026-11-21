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
	"encoding/json"
	"errors"
	"image/color"
	"strings"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/liquidgecka/homehub/activity"
	"github.com/liquidgecka/homehub/dpms"
)

const (
	// DefaultAlertDuration is how long an alert banner stays visible when no
	// explicit duration is specified.
	DefaultAlertDuration = 20 * time.Second

	// MaxAlertDuration caps how long a single Home Assistant alert remains on
	// screen before auto-dismissing.
	MaxAlertDuration = 300 * time.Second
)

// Alert describes an alert banner sent from Home Assistant for display on the
// HomeHub touchscreen.
type Alert struct {
	Title    string
	Message  string
	Level    string
	Duration time.Duration
	Dismiss  bool
}

var (
	alertHandlersMu      sync.Mutex
	alertHandlers        []func(Alert)
	onAlertOverlayUpdate func(visible bool)
)

// RegisterAlertHandler registers a callback invoked when an alert is
// triggered or dismissed.
func RegisterAlertHandler(fn func(Alert)) {
	if fn == nil {
		return
	}
	alertHandlersMu.Lock()
	defer alertHandlersMu.Unlock()
	alertHandlers = append(alertHandlers, fn)
}

// ClearAlertHandlers removes all registered alert handlers (useful for tests).
func ClearAlertHandlers() {
	alertHandlersMu.Lock()
	defer alertHandlersMu.Unlock()
	alertHandlers = nil
}

// TriggerAlert dispatches an alert to registered UI overlay handlers.
func TriggerAlert(a Alert) {
	if !a.Dismiss {
		a.Level = normalizeAlertLevel(a.Level)
		a.Duration = clampAlertDuration(a.Duration)
		if strings.TrimSpace(a.Title) == "" {
			a.Title = "Home Assistant"
		}
	}
	alertHandlersMu.Lock()
	handlers := make([]func(Alert), len(alertHandlers))
	copy(handlers, alertHandlers)
	alertHandlersMu.Unlock()

	for _, fn := range handlers {
		if fn != nil {
			fn(a)
		}
	}
}

// DismissAlert hides any currently displayed Home Assistant alert overlay.
func DismissAlert() {
	TriggerAlert(Alert{Dismiss: true})
}

func normalizeAlertLevel(level string) string {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "warning", "warn":
		return "warning"
	case "critical", "error", "urgent":
		return "critical"
	default:
		return "info"
	}
}

func clampAlertDuration(d time.Duration) time.Duration {
	if d <= 0 {
		return DefaultAlertDuration
	}
	if d > MaxAlertDuration {
		return MaxAlertDuration
	}
	return d
}

// ParseAlertPayload parses either JSON or plain-text MQTT alert payloads into
// an Alert struct.
func ParseAlertPayload(payload []byte) (Alert, error) {
	trimmed := strings.TrimSpace(string(payload))
	if trimmed == "" {
		return Alert{}, errors.New("empty alert payload")
	}

	if strings.HasPrefix(trimmed, "{") {
		var raw struct {
			Title           string  `json:"title"`
			Message         string  `json:"message"`
			Level           string  `json:"level"`
			DurationSeconds float64 `json:"duration_seconds"`
			Dismiss         bool    `json:"dismiss"`
		}
		if err := json.Unmarshal([]byte(trimmed), &raw); err != nil {
			return Alert{}, err
		}
		if raw.Dismiss {
			return Alert{Dismiss: true}, nil
		}
		title := strings.TrimSpace(raw.Title)
		msg := strings.TrimSpace(raw.Message)
		if title == "" && msg == "" {
			return Alert{}, errors.New("alert requires message or title")
		}
		if title == "" {
			title = "Home Assistant"
		}
		if msg == "" {
			msg = title
		}
		dur := time.Duration(raw.DurationSeconds * float64(time.Second))
		return Alert{
			Title:    title,
			Message:  msg,
			Level:    normalizeAlertLevel(raw.Level),
			Duration: clampAlertDuration(dur),
		}, nil
	}

	if strings.EqualFold(trimmed, "dismiss") ||
		strings.EqualFold(trimmed, "clear") {
		return Alert{Dismiss: true}, nil
	}

	return Alert{
		Title:    "Home Assistant",
		Message:  trimmed,
		Level:    "info",
		Duration: DefaultAlertDuration,
	}, nil
}

func colorsForAlertLevel(level string) (color.NRGBA, color.NRGBA) {
	switch normalizeAlertLevel(level) {
	case "critical":
		return color.NRGBA{R: 239, G: 68, B: 68, A: 255},
			color.NRGBA{R: 252, G: 165, B: 165, A: 255}
	case "warning":
		return color.NRGBA{R: 245, G: 158, B: 11, A: 255},
			color.NRGBA{R: 253, G: 224, B: 71, A: 255}
	default:
		return color.NRGBA{R: 59, G: 130, B: 246, A: 255},
			color.NRGBA{R: 147, G: 197, B: 253, A: 255}
	}
}

// CreateAlertOverlayView creates the touchscreen overlay container used to
// display incoming Home Assistant notifications over the Home view.
func CreateAlertOverlayView() *fyne.Container {
	overlayHook := onAlertOverlayUpdate

	headerText := canvas.NewText(
		"HOME ASSISTANT",
		color.NRGBA{R: 147, G: 197, B: 253, A: 255},
	)
	headerText.TextSize = 15
	headerText.TextStyle.Bold = true
	headerText.Alignment = fyne.TextAlignCenter

	titleText := canvas.NewText("", color.White)
	titleText.TextSize = 22
	titleText.TextStyle.Bold = true
	titleText.Alignment = fyne.TextAlignCenter

	messageLabel := widget.NewLabel("")
	messageLabel.Alignment = fyne.TextAlignCenter
	messageLabel.Wrapping = fyne.TextWrapWord

	dismissBtn := widget.NewButtonWithIcon(
		"Dismiss", theme.CancelIcon(), nil,
	)
	dismissBtn.Importance = widget.HighImportance

	bg := canvas.NewRectangle(color.NRGBA{R: 15, G: 23, B: 42, A: 242})
	bg.CornerRadius = 14
	bg.StrokeColor = color.NRGBA{R: 59, G: 130, B: 246, A: 255}
	bg.StrokeWidth = 3

	textBox := container.NewVBox(
		headerText,
		titleText,
		messageLabel,
	)

	cardBody := container.NewBorder(
		nil,
		container.NewCenter(container.NewPadded(dismissBtn)),
		nil,
		nil,
		textBox,
	)

	card := container.New(
		layout.NewMaxLayout(),
		bg,
		container.NewPadded(cardBody),
	)

	rootContainer := container.New(
		layout.NewCustomPaddedLayout(24, 24, 36, 36),
		container.NewVBox(container.NewCenter(card)),
	)
	rootContainer.Hide()

	var (
		timerMu      sync.Mutex
		dismissTimer *time.Timer
		alertSeq     uint64
		uiMu         sync.Mutex
	)

	doUI := func(fn func()) {
		fyne.Do(func() {
			uiMu.Lock()
			defer uiMu.Unlock()
			fn()
		})
	}

	hideUI := func() {
		doUI(func() {
			if rootContainer.Visible() {
				rootContainer.Hide()
				rootContainer.Refresh()
				if overlayHook != nil {
					overlayHook(false)
				}
			}
		})
	}

	hideOverlay := func() {
		timerMu.Lock()
		alertSeq++
		if dismissTimer != nil {
			dismissTimer.Stop()
			dismissTimer = nil
		}
		timerMu.Unlock()
		hideUI()
	}

	hideOverlayForSeq := func(seq uint64) {
		timerMu.Lock()
		if seq != alertSeq {
			timerMu.Unlock()
			return
		}
		dismissTimer = nil
		timerMu.Unlock()
		hideUI()
	}

	dismissBtn.OnTapped = func() {
		if activity.ResetTimer != nil {
			activity.ResetTimer()
		}
		hideOverlay()
	}

	dpms.RegisterStateListener(func(on bool) {
		if !on {
			hideOverlay()
		}
	})

	RegisterAlertHandler(func(a Alert) {
		if a.Dismiss {
			hideOverlay()
			return
		}
		if !dpms.IsDisplayOn() {
			return
		}

		strokeCol, headerCol := colorsForAlertLevel(a.Level)
		dur := clampAlertDuration(a.Duration)

		timerMu.Lock()
		alertSeq++
		seq := alertSeq
		if dismissTimer != nil {
			dismissTimer.Stop()
		}
		dismissTimer = time.AfterFunc(dur, func() {
			hideOverlayForSeq(seq)
		})
		timerMu.Unlock()

		doUI(func() {
			if activity.ResetTimer != nil {
				activity.ResetTimer()
			}
			bg.StrokeColor = strokeCol
			bg.Refresh()
			headerText.Color = headerCol
			headerText.Text = "HOME ASSISTANT · " +
				strings.ToUpper(normalizeAlertLevel(a.Level))
			headerText.Refresh()

			titleText.Text = a.Title
			titleText.Refresh()
			messageLabel.SetText(a.Message)

			rootContainer.Show()
			rootContainer.Refresh()
			if overlayHook != nil {
				overlayHook(true)
			}
		})
	})

	return rootContainer
}
