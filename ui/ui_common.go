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

package ui

import (
	"image/color"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/widget"

	"github.com/liquidgecka/homehub/activity"
	"github.com/liquidgecka/homehub/config"
)

var (
	keyboardTimer *time.Timer
	keyboardMu    sync.Mutex
)

// RunOnscreenKeyboardCommand executes the configured onscreen keyboard command.
// If show is true, it attempts to launch the configured command or the default Onboard show command.
// If show is false, it attempts to hide Onboard using its D-Bus interface.
func RunOnscreenKeyboardCommand(show bool) {
	keyboardMu.Lock()
	defer keyboardMu.Unlock()

	if show {
		if keyboardTimer != nil {
			keyboardTimer.Stop()
			keyboardTimer = nil
		}
		execKeyboardCommand(true)
	} else {
		if keyboardTimer != nil {
			keyboardTimer.Stop()
		}
		keyboardTimer = time.AfterFunc(150*time.Millisecond, func() {
			keyboardMu.Lock()
			defer keyboardMu.Unlock()
			keyboardTimer = nil
			execKeyboardCommand(false)
		})
	}
}

func execKeyboardCommand(show bool) {
	cfg := config.GetConfig()
	configuredCommand := cfg.App.OnscreenKeyboardCommand

	if show {
		if configuredCommand != "" {
			parts := strings.Fields(configuredCommand)
			if len(parts) == 0 {
				log.Println("WARNING: OnscreenKeyboardCommand is set but empty. Not running.")
				return
			}
			cmd := exec.Command(parts[0], parts[1:]...)
			if err := cmd.Start(); err != nil { // Use Start for non-blocking execution
				log.Printf("ERROR: Failed to run configured onscreen keyboard command '%s': %v", configuredCommand, err)
			} else {
				log.Printf("Executed configured onscreen keyboard command: %s", configuredCommand)
				go func() {
					_ = cmd.Wait()
				}()
			}
		} else {
			// Fallback to default Onboard D-Bus show command if no command is configured
			cmd := exec.Command(
				"dbus-send",
				"--type=method_call",
				"--dest=org.onboard.Onboard",
				"/org/onboard/Onboard/Keyboard",
				"org.onboard.Onboard.Keyboard.Show",
			)
			if err := cmd.Run(); err != nil {
				log.Printf("ERROR: Failed to show Onboard keyboard (default): %v", err)
			} else {
				log.Println("Onboard keyboard shown (default).")
			}
		}
	} else {
		// Always attempt to hide Onboard using its D-Bus interface
		cmd := exec.Command(
			"dbus-send",
			"--type=method_call",
			"--dest=org.onboard.Onboard",
			"/org/onboard/Onboard/Keyboard",
			"org.onboard.Onboard.Keyboard.Hide",
		)
		if err := cmd.Run(); err != nil {
			log.Printf("ERROR: Failed to hide Onboard keyboard (default): %v", err)
		} else {
			log.Println("Onboard keyboard hidden (default).")
		}
	}
}

// KeyboardEntry is a custom Entry widget that shows/hides the on-screen keyboard on focus change.
type KeyboardEntry struct {
	widget.Entry
	win fyne.Window
}

// NewKeyboardEntry creates a new keyboardEntry widget.
func NewKeyboardEntry(win fyne.Window) *KeyboardEntry {
	e := &KeyboardEntry{
		win: win,
	}
	e.ExtendBaseWidget(e)
	return e
}

// Tapped is called when the keyboardEntry is tapped.
func (e *KeyboardEntry) Tapped(ev *fyne.PointEvent) {
	if activity.ResetTimer != nil {
		activity.ResetTimer()
	}
	RunOnscreenKeyboardCommand(true) // Ensure keyboard shows when text box is tapped
	e.Entry.Tapped(ev)
}

// FocusGained is called when the keyboardEntry gains focus.
func (e *KeyboardEntry) FocusGained() {
	if activity.ResetTimer != nil {
		activity.ResetTimer()
	}
	RunOnscreenKeyboardCommand(true) // Show keyboard
	e.Entry.FocusGained()            // Call the embedded Entry's FocusGained
}

// FocusLost is called when the keyboardEntry loses focus.
func (e *KeyboardEntry) FocusLost() {
	RunOnscreenKeyboardCommand(false) // Hide keyboard
	e.Entry.FocusLost()               // Call the embedded Entry's FocusLost
}

// TypedRune is called when a rune is typed into the entry.
func (e *KeyboardEntry) TypedRune(r rune) {
	if activity.ResetTimer != nil {
		activity.ResetTimer()
	}
	e.Entry.TypedRune(r)
}

// TypedKey is called when a key is typed into the entry.
func (e *KeyboardEntry) TypedKey(k *fyne.KeyEvent) {
	if activity.ResetTimer != nil {
		activity.ResetTimer()
	}
	e.Entry.TypedKey(k)
}

// NoPaddingLayout is a custom layout that stacks objects vertically with 0 spacing.
type NoPaddingLayout struct{}

// MinSize calculates the minimum size of the layout.
func (n *NoPaddingLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	minSize := fyne.NewSize(0, 0)
	for _, o := range objects {
		if !o.Visible() {
			continue
		}
		itemMin := o.MinSize()
		minSize.Width = fyne.Max(minSize.Width, itemMin.Width)
		minSize.Height += itemMin.Height
	}
	return minSize
}

// Layout positions and sizes the objects within the given container size.
func (n *NoPaddingLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	yOffset := float32(0)
	for _, o := range objects {
		if !o.Visible() {
			continue
		}
		itemMin := o.MinSize()
		o.Resize(fyne.NewSize(size.Width, itemMin.Height))
		o.Move(fyne.NewPos(0, yOffset))
		yOffset += itemMin.Height
	}
}

// customTheme allows overriding specific theme values like icon size.
type customTheme struct {
	fyne.Theme // Embed the default theme
}

func (t *customTheme) Padding() float32 {
	return 0
}

// CompactLabel is a custom widget for displaying text with no padding.
type CompactLabel struct {
	widget.BaseWidget
	Text *canvas.Text
}

// NewCompactLabel creates a new CompactLabel widget.
func NewCompactLabel(text string) *CompactLabel {
	l := &CompactLabel{
		Text: canvas.NewText(text, color.White),
	}
	l.ExtendBaseWidget(l)
	return l
}

// CreateRenderer returns a new WidgetRenderer for this widget.
func (l *CompactLabel) CreateRenderer() fyne.WidgetRenderer {
	return &compactLabelRenderer{text: l.Text}
}

type compactLabelRenderer struct {
	text *canvas.Text
}

func (r *compactLabelRenderer) MinSize() fyne.Size {
	return r.text.MinSize()
}

func (r *compactLabelRenderer) Layout(size fyne.Size) {
	r.text.Resize(size)
}

func (r *compactLabelRenderer) Refresh() {
	// No-op
}

func (r *compactLabelRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.text}
}

func (r *compactLabelRenderer) Destroy() {
	// No-op
}

// TappableText is a custom widget for clickable text with explicit font size.
type TappableText struct {
	widget.BaseWidget
	Text      *canvas.Text
	OnTap     func()
	Alignment fyne.TextAlign
}

// NewTappableText creates a new TappableText widget.
func NewTappableText(text string, textColor color.Color, textSize float32, onTap func(), alignment fyne.TextAlign) *TappableText {
	t := &TappableText{
		Text:      canvas.NewText(text, textColor),
		OnTap:     onTap,
		Alignment: alignment,
	}
	t.ExtendBaseWidget(t)
	t.Text.TextSize = textSize
	return t
}

// CreateRenderer returns a new WidgetRenderer for this widget.
func (t *TappableText) CreateRenderer() fyne.WidgetRenderer {
	return &tappableTextRenderer{text: t.Text, alignment: t.Alignment}
}

// Tapped is called when the widget is tapped.
func (t *TappableText) Tapped(*fyne.PointEvent) {
	if activity.ResetTimer != nil {
		activity.ResetTimer()
	}
	if t.OnTap != nil {
		t.OnTap()
	}
}

type tappableTextRenderer struct {
	text      *canvas.Text
	alignment fyne.TextAlign
}

func (r *tappableTextRenderer) MinSize() fyne.Size {
	return r.text.MinSize()
}

func (r *tappableTextRenderer) Layout(size fyne.Size) {
	r.text.Resize(size)
	r.text.Alignment = r.alignment
}

func (r *tappableTextRenderer) Refresh() {
	r.text.Refresh() // Ensure the internal canvas text object updates its metrics
	// The parent layout (HBox) should re-query MinSize after this, which will pick up new text size
}

func (r *tappableTextRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.text}
}

func (r *tappableTextRenderer) Destroy() {
	// No-op
}

// TappableIcon is a custom widget for a transparent, tappable icon.
type TappableIcon struct {
	widget.BaseWidget
	widget.Icon
	OnTapped func()
}

// NewTappableIcon creates a new TappableIcon.
func NewTappableIcon(resource fyne.Resource, tapped func()) *TappableIcon {
	icon := &TappableIcon{
		Icon:     widget.Icon{Resource: resource},
		OnTapped: tapped,
	}
	icon.ExtendBaseWidget(icon)
	return icon
}

// Tapped is called when the icon is tapped.
func (t *TappableIcon) Tapped(*fyne.PointEvent) {
	if activity.ResetTimer != nil {
		activity.ResetTimer()
	}
	if t.OnTapped != nil {
		t.OnTapped()
	}
}

// CreateRenderer returns a new WidgetRenderer for this widget.
func (t *TappableIcon) CreateRenderer() fyne.WidgetRenderer {
	// The Icon widget already has a renderer, we just want to ensure it's rendered.
	return widget.NewSimpleRenderer(&t.Icon)
}

// MinSize returns the minimal size of the widget.
func (t *TappableIcon) MinSize() fyne.Size {
	return fyne.NewSize(48, 48) // Fixed size for the tappable area
}

// MustLoadFile reads a file from the given path and returns its content as a byte slice.
// It panics if the file cannot be read. This is useful for loading static assets that
// are expected to always be present.
func MustLoadFile(path string) []byte {
	data, err := os.ReadFile(path)
	if err != nil {
		log.Fatalf("Failed to load file %s: %v", path, err)
	}
	return data
}

// GetIconPath returns the full path to an icon file, using the configured icons directory.
func GetIconPath(iconFileName string) string {
	cfg := config.GetConfig()
	return filepath.Join(cfg.App.IconsDirectory, iconFileName)
}
