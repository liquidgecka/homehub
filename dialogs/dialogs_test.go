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

package dialogs

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
)

// mockDialog is a mock implementation of dialog.Dialog for testing.
type mockDialog struct {
	hidden bool
}

func (m *mockDialog) Show() {}

func (m *mockDialog) Hide() {
	m.hidden = true
}

func (m *mockDialog) SetDismissText(text string) {}

func (m *mockDialog) SetOnClosed(callback func()) {}

func (m *mockDialog) Refresh() {}

func (m *mockDialog) MinSize() fyne.Size {
	return fyne.NewSize(100, 100)
}

func (m *mockDialog) Resize(size fyne.Size) {}

func (m *mockDialog) Dismiss() {
	m.hidden = true
}

func TestDialogTracking(t *testing.T) {
	// Ensure the dialog slice is initially empty.
	if len(openDialogs) != 0 {
		t.Errorf("Expected openDialogs to be empty, but got %d", len(openDialogs))
	}

	// Create some mock dialogs.
	d1 := &mockDialog{}
	d2 := &mockDialog{}

	// Add the dialogs and check the slice length.
	Add(d1)
	if len(openDialogs) != 1 {
		t.Errorf(
			"Expected openDialogs to have 1 dialog, but got %d",
			len(openDialogs),
		)
	}
	Add(d2)
	if len(openDialogs) != 2 {
		t.Errorf(
			"Expected openDialogs to have 2 dialogs, but got %d",
			len(openDialogs),
		)
	}

	// Remove a dialog and check the slice length.
	Remove(d1)
	if len(openDialogs) != 1 {
		t.Errorf(
			"Expected openDialogs to have 1 dialog, but got %d",
			len(openDialogs),
		)
	}
	if openDialogs[0] != d2 {
		t.Error("Expected the remaining dialog to be d2")
	}

	// Remove the other dialog.
	Remove(d2)
	if len(openDialogs) != 0 {
		t.Errorf("Expected openDialogs to be empty, but got %d", len(openDialogs))
	}
}

func TestCloseAll(t *testing.T) {
	// Ensure the dialog slice is initially empty.
	openDialogs = []dialog.Dialog{}

	// Create some mock dialogs.
	d1 := &mockDialog{}
	d2 := &mockDialog{}

	// Add the dialogs.
	Add(d1)
	Add(d2)

	// Call CloseAll and check that Hide() was called on each dialog
	// and the slice is cleared.
	CloseAll()
	if !d1.hidden {
		t.Error("Expected d1.Hide() to be called")
	}
	if !d2.hidden {
		t.Error("Expected d2.Hide() to be called")
	}
	if len(openDialogs) != 0 {
		t.Errorf(
			"Expected openDialogs empty after CloseAll, got %d",
			len(openDialogs),
		)
	}
}

func TestShowFunctions(t *testing.T) {
	app := test.NewApp()
	_ = app // Acknowledge app is needed for test driver setup
	win := test.NewWindow(widget.NewLabel("Test Content"))
	defer win.Close()

	t.Run("ShowCustomConfirm", func(t *testing.T) {
		openDialogs = []dialog.Dialog{} // Reset
		ShowCustomConfirm(
			"Title", "OK", "Cancel", widget.NewLabel("Content"), nil, win,
		)

		if len(openDialogs) != 1 {
			t.Errorf(
				"Expected 1 dialog after ShowCustomConfirm, got %d",
				len(openDialogs),
			)
		}
		// The dialog is shown and running in a separate goroutine.
		// We can call CloseAll to test the other side of tracking.
		CloseAll()
		if len(openDialogs) != 0 {
			t.Errorf(
				"Expected openDialogs empty after CloseAll, got %d",
				len(openDialogs),
			)
		}
	})

	t.Run("ShowImageDialog", func(t *testing.T) {
		openDialogs = []dialog.Dialog{} // Reset
		res := fyne.NewStaticResource(
			"test.png",
			[]byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a},
		)
		ShowImageDialog(win, "Image", res)

		if len(openDialogs) != 1 {
			t.Errorf(
				"Expected 1 dialog after ShowImageDialog, got %d",
				len(openDialogs),
			)
		}
		CloseAll()
		if len(openDialogs) != 0 {
			t.Errorf(
				"Expected openDialogs empty after CloseAll, got %d",
				len(openDialogs),
			)
		}
	})
}
