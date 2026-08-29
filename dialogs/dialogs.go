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
	"image"
	"sync"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/dialog"
)

var (
	openDialogs []dialog.Dialog
	mutex       sync.Mutex
)

// Add tracks a new dialog.
func Add(d dialog.Dialog) {
	mutex.Lock()
	defer mutex.Unlock()
	openDialogs = append(openDialogs, d)
}

// Remove stops tracking a dialog.
func Remove(d dialog.Dialog) {
	mutex.Lock()
	defer mutex.Unlock()
	for i, open := range openDialogs {
		if open == d {
			openDialogs = append(openDialogs[:i], openDialogs[i+1:]...)
			return
		}
	}
}

// CloseAll hides all tracked dialogs.
func CloseAll() {
	mutex.Lock()
	dialogsToClose := make([]dialog.Dialog, len(openDialogs))
	copy(dialogsToClose, openDialogs)
	openDialogs = []dialog.Dialog{} // Clear the list while we have the lock
	mutex.Unlock()

	for _, d := range dialogsToClose {
		d.Hide()
	}
}

// NewCustomConfirm creates a new confirmation dialog and tracks it.
// When the dialog is closed, it is automatically removed from the tracker.
func NewCustomConfirm(
	title, confirmation, dismiss string,
	content fyne.CanvasObject,
	callback func(bool),
	parent fyne.Window,
) dialog.Dialog {
	d := dialog.NewCustomConfirm(
		title, confirmation, dismiss, content, callback, parent,
	)
	d.SetOnClosed(func() {
		Remove(d)
	})
	Add(d)
	return d
}

// ShowCustomConfirm shows a new confirmation dialog and tracks it.
func ShowCustomConfirm(
	title, confirmation, dismiss string,
	content fyne.CanvasObject,
	callback func(bool),
	parent fyne.Window,
) {
	d := NewCustomConfirm(
		title, confirmation, dismiss, content, callback, parent,
	)
	d.Show()
}

// ShowImageDialog shows a dialog with an image.
func ShowImageDialog(win fyne.Window, title string, res fyne.Resource) {
	img := canvas.NewImageFromResource(res)
	img.FillMode = canvas.ImageFillContain
	img.SetMinSize(fyne.NewSize(640, 480))

	d := dialog.NewCustom(title, "Close", img, win)
	d.SetOnClosed(func() {
		Remove(d)
	})
	Add(d)
	d.Show()
}

// ShowImageDialogFromImage shows a dialog with an image.
func ShowImageDialogFromImage(
	win fyne.Window, title string, imageObj image.Image,
) {
	img := canvas.NewImageFromImage(imageObj)
	img.FillMode = canvas.ImageFillContain
	img.SetMinSize(fyne.NewSize(640, 480))

	d := dialog.NewCustom(title, "Close", img, win)
	d.SetOnClosed(func() {
		Remove(d)
	})
	Add(d)
	d.Show()
}
