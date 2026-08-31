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

package celebrations

import (
	"image/color"
	"os"
	"strings"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/liquidgecka/homehub/database"
	"github.com/liquidgecka/homehub/ui"
)

// DisplayDuration defines how long a celebration overlay stays visible on
// screen.
const DisplayDuration = 25 * time.Second

// loadIconResource safely loads an SVG icon from disk with fallback.
func loadIconResource(iconFileName string) fyne.Resource {
	iconPath := ui.GetIconPath(iconFileName)
	data, err := os.ReadFile(iconPath)
	if err == nil && len(data) > 0 {
		return fyne.NewStaticResource(iconFileName, data)
	}
	return theme.InfoIcon()
}

// GetIconForType returns the matching festive icon resource for a celebration type.
func GetIconForType(cType string) fyne.Resource {
	switch strings.ToLower(strings.TrimSpace(cType)) {
	case "birthday":
		return loadIconResource("balloons.svg")
	case "anniversary":
		return loadIconResource("rings.svg")
	case "graduation":
		return loadIconResource("graduation.svg")
	case "school", "first_day_of_school":
		return loadIconResource("school.svg")
	case "party", "congratulations", "holiday":
		return loadIconResource("party.svg")
	default:
		return loadIconResource("party.svg")
	}
}

// GetBannerText returns an appropriate festive banner headline.
func GetBannerText(c database.Celebration) string {
	switch strings.ToLower(strings.TrimSpace(c.Type)) {
	case "birthday":
		return "🎈 HAPPY BIRTHDAY! 🎈"
	case "anniversary":
		return "💍 HAPPY ANNIVERSARY! 💍"
	case "graduation":
		return "🎓 CONGRATULATIONS GRADUATE! 🎓"
	case "school", "first_day_of_school":
		return "🎒 FIRST DAY OF SCHOOL! 🎒"
	case "party":
		return "🎉 CELEBRATION! 🎉"
	case "holiday":
		return "🌟 HAPPY HOLIDAYS! 🌟"
	default:
		return "🎉 CELEBRATION 🎉"
	}
}

// CreatePhotoOverlayView builds the overlay container that pops up over the
// slideshow when celebrations trigger.
func CreatePhotoOverlayView() fyne.CanvasObject {
	bannerText := canvas.NewText(
		"🎉 CELEBRATION 🎉",
		color.NRGBA{R: 255, G: 215, B: 0, A: 255},
	)
	bannerText.TextSize = 20
	bannerText.TextStyle.Bold = true
	bannerText.Alignment = fyne.TextAlignCenter

	messageText := canvas.NewText("", color.White)
	messageText.TextSize = 24
	messageText.TextStyle.Bold = true
	messageText.Alignment = fyne.TextAlignCenter

	titleText := canvas.NewText(
		"", color.NRGBA{R: 220, G: 220, B: 240, A: 255},
	)
	titleText.TextSize = 16
	titleText.Alignment = fyne.TextAlignCenter

	iconImg := widget.NewIcon(loadIconResource("balloons.svg"))

	dismissBtn := widget.NewButtonWithIcon(
		"Dismiss", theme.CancelIcon(), nil,
	)
	dismissBtn.Importance = widget.LowImportance

	bg := canvas.NewRectangle(color.NRGBA{R: 15, G: 20, B: 35, A: 245})
	bg.CornerRadius = 16

	centerContent := container.NewVBox(
		bannerText,
		messageText,
		titleText,
	)

	cardBorder := container.NewBorder(
		nil, nil,
		container.NewPadded(iconImg),
		container.NewCenter(container.NewPadded(dismissBtn)),
		container.NewCenter(centerContent),
	)

	card := container.New(
		layout.NewMaxLayout(),
		bg,
		container.NewPadded(cardBorder),
	)

	rootContainer := container.New(
		layout.NewCustomPaddedLayout(30, 30, 30, 30),
		container.NewVBox(
			container.NewCenter(card),
		),
	)
	rootContainer.Hide()

	var dismissTimer *time.Timer
	var timerMu sync.Mutex

	hideOverlay := func() {
		timerMu.Lock()
		if dismissTimer != nil {
			dismissTimer.Stop()
			dismissTimer = nil
		}
		timerMu.Unlock()
		fyne.Do(func() {
			rootContainer.Hide()
			rootContainer.Refresh()
		})
	}

	dismissBtn.OnTapped = hideOverlay

	RegisterDisplayHandler(func(c database.Celebration) {
		fyne.Do(func() {
			bannerText.Text = GetBannerText(c)
			bannerText.Refresh()

			message := c.Message
			if message == "" {
				message = c.Title
			}
			messageText.Text = message
			messageText.Refresh()

			if c.Title != "" && c.Title != message {
				titleText.Text = c.Title
				titleText.Show()
			} else {
				titleText.Hide()
			}
			titleText.Refresh()

			iconImg.SetResource(GetIconForType(c.Type))
			iconImg.Refresh()

			rootContainer.Show()
			rootContainer.Refresh()

			timerMu.Lock()
			if dismissTimer != nil {
				dismissTimer.Stop()
			}
			dismissTimer = time.AfterFunc(DisplayDuration, hideOverlay)
			timerMu.Unlock()
		})
	})

	return rootContainer
}
