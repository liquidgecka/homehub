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
	"fmt"
	"image/color"
	"log"
	"math"
	"strings"
	"sync"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/liquidgecka/homehub/activity"
	"github.com/liquidgecka/homehub/config"
)

var onSmartHomeTileUpdated func(entityID string)

type smartHomeTile struct {
	widget.BaseWidget

	spec        config.HomeAssistantEntitySpec
	domain      string
	interactive bool

	bg      *canvas.Rectangle
	title   *canvas.Text
	status  *canvas.Text
	sub     *canvas.Text
	content *fyne.Container

	stateMu sync.RWMutex
	state   EntityState

	onTap func(EntityState)
}

func newSmartHomeTile(
	spec config.HomeAssistantEntitySpec,
	onTap func(EntityState),
	onTempDelta func(EntityState, float64),
) *smartHomeTile {
	parts := strings.SplitN(spec.EntityID, ".", 2)
	domain := ""
	if len(parts) > 0 {
		domain = parts[0]
	}

	interactive := isInteractiveDomain(domain)

	bg := canvas.NewRectangle(color.NRGBA{R: 30, G: 35, B: 48, A: 255})
	bg.CornerRadius = 12
	bg.StrokeColor = color.NRGBA{R: 71, G: 85, B: 105, A: 255}
	bg.StrokeWidth = 2

	displayTitle := strings.TrimSpace(spec.Name)
	if displayTitle == "" {
		displayTitle = spec.EntityID
	}

	title := canvas.NewText(displayTitle, color.White)
	title.TextSize = 18
	title.TextStyle.Bold = true
	title.Alignment = fyne.TextAlignCenter

	status := canvas.NewText("Loading...", color.NRGBA{
		R: 203, G: 213, B: 225, A: 255,
	})
	status.TextSize = 22
	status.TextStyle.Bold = true
	status.Alignment = fyne.TextAlignCenter

	sub := canvas.NewText("", color.NRGBA{R: 148, G: 163, B: 184, A: 255})
	sub.TextSize = 13
	sub.Alignment = fyne.TextAlignCenter

	t := &smartHomeTile{
		spec:        spec,
		domain:      domain,
		interactive: interactive,
		bg:          bg,
		title:       title,
		status:      status,
		sub:         sub,
		state:       EntityState{EntityID: spec.EntityID},
		onTap:       onTap,
	}

	centerBox := container.NewVBox(
		title,
		status,
		sub,
	)

	var body fyne.CanvasObject = container.NewCenter(centerBox)
	if domain == "climate" && onTempDelta != nil {
		decBtn := widget.NewButton("-", func() {
			if activity.ResetTimer != nil {
				activity.ResetTimer()
			}
			onTempDelta(t.getState(), -1)
		})
		incBtn := widget.NewButton("+", func() {
			if activity.ResetTimer != nil {
				activity.ResetTimer()
			}
			onTempDelta(t.getState(), 1)
		})
		incBtn.Importance = widget.HighImportance
		controls := container.NewGridWithColumns(2, decBtn, incBtn)
		body = container.NewBorder(
			nil, container.NewPadded(controls), nil, nil,
			container.NewCenter(centerBox),
		)
	}

	t.content = container.New(
		layout.NewMaxLayout(),
		bg,
		container.NewPadded(body),
	)
	t.ExtendBaseWidget(t)
	return t
}

func (t *smartHomeTile) getState() EntityState {
	t.stateMu.RLock()
	defer t.stateMu.RUnlock()
	return t.state
}

func (t *smartHomeTile) applyStateLocked(st EntityState) {
	t.stateMu.Lock()
	t.state = st
	t.stateMu.Unlock()

	t.title.Text = st.FriendlyName(t.spec.Name)
	statusStr, subStr := formatTileStatus(st)
	t.status.Text = statusStr
	t.sub.Text = subStr

	fill, stroke, statusCol := colorsForEntityState(st)
	t.bg.FillColor = fill
	t.bg.StrokeColor = stroke
	t.status.Color = statusCol

	t.title.Refresh()
	t.status.Refresh()
	t.sub.Refresh()
	t.bg.Refresh()
}

func (t *smartHomeTile) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(t.content)
}

func (t *smartHomeTile) MinSize() fyne.Size {
	return fyne.NewSize(180, 140)
}

func (t *smartHomeTile) Tapped(*fyne.PointEvent) {
	if !t.interactive || t.onTap == nil {
		return
	}
	if activity.ResetTimer != nil {
		activity.ResetTimer()
	}
	t.onTap(t.getState())
}

func (t *smartHomeTile) TappedSecondary(*fyne.PointEvent) {}

func isInteractiveDomain(domain string) bool {
	switch domain {
	case "light", "switch", "fan", "input_boolean",
		"scene", "script", "button", "input_button",
		"lock", "cover":
		return true
	default:
		return false
	}
}

func formatTileStatus(st EntityState) (string, string) {
	low := strings.ToLower(strings.TrimSpace(st.State))
	if low == "" || low == "unavailable" || low == "unknown" {
		if low == "" {
			return "Unavailable", ""
		}
		return strings.ToUpper(low[:1]) + low[1:], ""
	}

	switch st.Domain() {
	case "light":
		if low == "on" {
			if bri, ok := st.FloatAttr("brightness"); ok {
				pct := int(math.Round(bri * 100 / 255))
				return fmt.Sprintf("On · %d%%", pct), "Tap to toggle"
			}
			return "On", "Tap to toggle"
		}
		return "Off", "Tap to toggle"
	case "switch", "fan", "input_boolean":
		if low == "on" {
			return "On", "Tap to toggle"
		}
		return "Off", "Tap to toggle"
	case "lock":
		if low == "locked" {
			return "Locked", "Tap to unlock"
		}
		return "Unlocked", "Tap to lock"
	case "cover":
		return strings.ToUpper(low[:1]) + low[1:], "Tap to toggle"
	case "scene", "script":
		return "Scene", "Tap to activate"
	case "button", "input_button":
		return "Button", "Tap to press"
	case "climate":
		target, hasTarget := st.FloatAttr("temperature")
		curr, hasCurr := st.FloatAttr("current_temperature")
		main := strings.ToUpper(low[:1]) + low[1:]
		if hasTarget {
			main = fmt.Sprintf("%.0f°", target)
		}
		var subParts []string
		if hasCurr {
			subParts = append(subParts, fmt.Sprintf("Now %.0f°", curr))
		}
		subParts = append(subParts, strings.ToUpper(low[:1])+low[1:])
		return main, strings.Join(subParts, " · ")
	default:
		unit := st.StringAttr("unit_of_measurement")
		if unit != "" {
			return fmt.Sprintf("%s %s", st.State, unit), ""
		}
		return st.State, ""
	}
}

func colorsForEntityState(st EntityState) (
	color.NRGBA, color.NRGBA, color.NRGBA,
) {
	low := strings.ToLower(strings.TrimSpace(st.State))
	if low == "" || low == "unavailable" || low == "unknown" {
		return color.NRGBA{R: 28, G: 28, B: 34, A: 255},
			color.NRGBA{R: 75, G: 85, B: 99, A: 255},
			color.NRGBA{R: 156, G: 163, B: 175, A: 255}
	}
	if st.Domain() == "climate" {
		return color.NRGBA{R: 20, G: 50, B: 65, A: 255},
			color.NRGBA{R: 56, G: 189, B: 248, A: 255},
			color.NRGBA{R: 224, G: 242, B: 254, A: 255}
	}
	if st.IsActive() {
		return color.NRGBA{R: 30, G: 64, B: 175, A: 255},
			color.NRGBA{R: 96, G: 165, B: 250, A: 255},
			color.NRGBA{R: 255, G: 255, B: 255, A: 255}
	}
	return color.NRGBA{R: 30, G: 35, B: 48, A: 255},
		color.NRGBA{R: 71, G: 85, B: 105, A: 255},
		color.NRGBA{R: 203, G: 213, B: 225, A: 255}
}

// CreateSmartHomeView creates the touchscreen Smart Home control grid and a
// stop callback that tears down the live WebSocket listener on tab exit.
func CreateSmartHomeView(
	parentCtx context.Context, cfg config.HomeAssistantConfig,
) (fyne.CanvasObject, func()) {
	client := NewClient(cfg.URL, cfg.Token)
	return createSmartHomeViewWithClient(parentCtx, client, cfg.Entities)
}

func createSmartHomeViewWithClient(
	parentCtx context.Context,
	client *Client,
	entities []config.HomeAssistantEntitySpec,
) (fyne.CanvasObject, func()) {
	tileHook := onSmartHomeTileUpdated
	viewCtx, cancel := context.WithCancel(parentCtx)

	var (
		uiMu     sync.Mutex
		wg       sync.WaitGroup
		stopOnce sync.Once
	)
	doUI := func(fn func()) {
		fyne.Do(func() {
			uiMu.Lock()
			defer uiMu.Unlock()
			fn()
		})
	}

	headerText := canvas.NewText("Smart Home", color.White)
	headerText.TextSize = 22
	headerText.TextStyle.Bold = true

	statusLabel := canvas.NewText(
		"Connecting...",
		color.NRGBA{R: 148, G: 163, B: 184, A: 255},
	)
	statusLabel.TextSize = 14

	tilesByID := make(map[string]*smartHomeTile, len(entities))
	entityIDs := make(map[string]bool, len(entities))
	tileObjects := make([]fyne.CanvasObject, 0, len(entities))

	for _, spec := range entities {
		if strings.TrimSpace(spec.EntityID) == "" {
			continue
		}
		entityIDs[spec.EntityID] = true
		tile := newSmartHomeTile(
			spec,
			func(st EntityState) {
				go func() {
					if err := client.ToggleOrActivate(
						viewCtx, st,
					); err != nil {
						log.Printf(
							"HA UI: Failed action for %s: %v",
							st.EntityID, err,
						)
					}
				}()
			},
			func(st EntityState, delta float64) {
				target, ok := st.FloatAttr("temperature")
				if !ok {
					target = 70
				}
				newTemp := target + delta
				go func() {
					if err := client.SetClimateTemperature(
						viewCtx, st.EntityID, newTemp,
					); err != nil {
						log.Printf(
							"HA UI: Failed set temp for %s: %v",
							st.EntityID, err,
						)
					}
				}()
			},
		)
		tilesByID[spec.EntityID] = tile
		tileObjects = append(tileObjects, container.NewPadded(tile))
	}

	fetchStates := func() {
		states, err := client.GetStates(viewCtx)
		if err != nil {
			if viewCtx.Err() == nil {
				log.Printf("HA UI: Failed to fetch states: %v", err)
				doUI(func() {
					statusLabel.Text = "Error loading states"
					statusLabel.Color = color.NRGBA{
						R: 248, G: 113, B: 113, A: 255,
					}
					statusLabel.Refresh()
				})
			}
			return
		}
		doUI(func() {
			for id, tile := range tilesByID {
				if st, ok := states[id]; ok {
					tile.applyStateLocked(st)
				} else {
					tile.applyStateLocked(EntityState{
						EntityID: id,
						State:    "unavailable",
					})
				}
				if tileHook != nil {
					tileHook(id)
				}
			}
		})
	}

	refreshBtn := widget.NewButtonWithIcon(
		"Refresh", theme.ViewRefreshIcon(), func() {
			if activity.ResetTimer != nil {
				activity.ResetTimer()
			}
			wg.Add(1)
			go func() {
				defer wg.Done()
				fetchStates()
			}()
		},
	)

	header := container.NewPadded(
		container.NewHBox(
			headerText,
			layout.NewSpacer(),
			container.NewCenter(statusLabel),
			refreshBtn,
		),
	)

	var body fyne.CanvasObject
	if len(tileObjects) == 0 {
		empty := widget.NewLabel(
			"No Home Assistant entities configured in config.toml.",
		)
		empty.Alignment = fyne.TextAlignCenter
		body = container.NewCenter(empty)
	} else {
		grid := container.NewGridWithColumns(3, tileObjects...)
		body = container.NewVScroll(container.NewPadded(grid))
	}

	root := container.NewBorder(header, nil, nil, nil, body)

	wg.Add(1)
	go func() {
		defer wg.Done()
		fetchStates()
	}()

	if len(entityIDs) > 0 && client.baseURL != "" {
		wg.Add(1)
		go func() {
			defer wg.Done()
			client.SubscribeStateChanges(
				viewCtx,
				entityIDs,
				func(st EntityState) {
					if tile, ok := tilesByID[st.EntityID]; ok {
						doUI(func() {
							tile.applyStateLocked(st)
							if tileHook != nil {
								tileHook(st.EntityID)
							}
						})
					}
				},
				func(connected bool) {
					doUI(func() {
						if connected {
							statusLabel.Text = "Live"
							statusLabel.Color = color.NRGBA{
								R: 74, G: 222, B: 128, A: 255,
							}
						} else {
							statusLabel.Text = "Reconnecting..."
							statusLabel.Color = color.NRGBA{
								R: 251, G: 191, B: 36, A: 255,
							}
						}
						statusLabel.Refresh()
					})
				},
			)
		}()
	}

	stop := func() {
		stopOnce.Do(func() {
			cancel()
			wg.Wait()
		})
	}

	return root, stop
}
