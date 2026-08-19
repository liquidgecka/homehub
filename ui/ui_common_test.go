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
	"path/filepath"
	"testing"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/test"
	"github.com/liquidgecka/homehub/activity"
	"github.com/liquidgecka/homehub/config"
)

func TestRunOnscreenKeyboardCommand(t *testing.T) {
	// Exercise show and debounced hide logic
	RunOnscreenKeyboardCommand(true)
	RunOnscreenKeyboardCommand(false)
	// Cancel pending hide by requesting show again
	RunOnscreenKeyboardCommand(true)
	// Allow any background timers to settle
	time.Sleep(200 * time.Millisecond)
}

func TestKeyboardEntry(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	win := app.NewWindow("Test")
	defer win.Close()

	var timerReset bool
	activity.ResetTimer = func() {
		timerReset = true
	}
	defer func() { activity.ResetTimer = nil }()

	entry := NewKeyboardEntry(win)
	if entry == nil {
		t.Fatal("NewKeyboardEntry returned nil")
	}

	entry.Tapped(&fyne.PointEvent{})
	if !timerReset {
		t.Error("Expected activity.ResetTimer to be called on Tapped")
	}

	timerReset = false
	entry.FocusGained()
	if !timerReset {
		t.Error("Expected activity.ResetTimer to be called on FocusGained")
	}

	entry.FocusLost()

	timerReset = false
	entry.TypedRune('a')
	if !timerReset {
		t.Error("Expected activity.ResetTimer to be called on TypedRune")
	}

	timerReset = false
	entry.TypedKey(&fyne.KeyEvent{Name: fyne.KeyReturn})
	if !timerReset {
		t.Error("Expected activity.ResetTimer to be called on TypedKey")
	}
}

func TestNoPaddingLayout(t *testing.T) {
	l := &NoPaddingLayout{}
	txt1 := canvas.NewText("Line 1", color.White)
	txt2 := canvas.NewText("Line 2", color.White)
	txtHidden := canvas.NewText("Hidden", color.White)
	txtHidden.Hide()

	objects := []fyne.CanvasObject{txt1, txt2, txtHidden}
	minSize := l.MinSize(objects)
	if minSize.Height <= 0 {
		t.Errorf("Expected positive height, got %f", minSize.Height)
	}

	l.Layout(objects, fyne.NewSize(100, 50))
}

func TestCustomTheme(t *testing.T) {
	ct := &customTheme{}
	if ct.Padding() != 0 {
		t.Errorf("Expected padding 0, got %f", ct.Padding())
	}
}

func TestCompactLabel(t *testing.T) {
	cl := NewCompactLabel("Test Label")
	if cl == nil || cl.Text == nil {
		t.Fatal("NewCompactLabel returned invalid object")
	}
	renderer := cl.CreateRenderer()
	if renderer == nil {
		t.Fatal("CreateRenderer returned nil")
	}
	_ = renderer.MinSize()
	renderer.Layout(fyne.NewSize(100, 20))
	renderer.Refresh()
	if len(renderer.Objects()) != 1 {
		t.Errorf("Expected 1 object in renderer, got %d", len(renderer.Objects()))
	}
	renderer.Destroy()
}

func TestTappableText(t *testing.T) {
	var tapped bool
	activity.ResetTimer = func() {}
	defer func() { activity.ResetTimer = nil }()

	tt := NewTappableText("Click Me", color.White, 16, func() {
		tapped = true
	}, fyne.TextAlignCenter)

	if tt == nil {
		t.Fatal("NewTappableText returned nil")
	}

	renderer := tt.CreateRenderer()
	_ = renderer.MinSize()
	renderer.Layout(fyne.NewSize(80, 20))
	renderer.Refresh()
	if len(renderer.Objects()) != 1 {
		t.Errorf("Expected 1 object, got %d", len(renderer.Objects()))
	}
	renderer.Destroy()

	tt.Tapped(&fyne.PointEvent{})
	if !tapped {
		t.Error("Expected OnTap callback to be invoked")
	}
}

func TestTappableIcon(t *testing.T) {
	var tapped bool
	activity.ResetTimer = func() {}
	defer func() { activity.ResetTimer = nil }()

	res := fyne.NewStaticResource("test.png", []byte{})
	ti := NewTappableIcon(res, func() {
		tapped = true
	})

	if ti == nil {
		t.Fatal("NewTappableIcon returned nil")
	}

	if ti.MinSize().Width != 48 || ti.MinSize().Height != 48 {
		t.Errorf("Unexpected MinSize: %v", ti.MinSize())
	}

	renderer := ti.CreateRenderer()
	if renderer == nil {
		t.Fatal("CreateRenderer returned nil")
	}

	ti.Tapped(&fyne.PointEvent{})
	if !tapped {
		t.Error("Expected OnTapped callback to be invoked")
	}

	newRes := fyne.NewStaticResource("new.png", []byte{})
	ti.SetResource(newRes)
	if ti.Resource.Name() != "new.png" {
		t.Errorf("Expected Resource name to be 'new.png', got '%s'", ti.Resource.Name())
	}
}

func TestGetIconPath(t *testing.T) {
	cfg := config.GetConfig()
	cfg.App.IconsDirectory = "/tmp/icons"

	path := GetIconPath("bell.svg")
	expected := filepath.Join("/tmp/icons", "bell.svg")
	if path != expected {
		t.Errorf("GetIconPath = %q, want %q", path, expected)
	}
}
