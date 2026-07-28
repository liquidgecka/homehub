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

package reminders

import (
	"fmt"
	"image/color"
	"log"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/liquidgecka/homehub/database"
	"github.com/liquidgecka/homehub/dialogs"
	"github.com/liquidgecka/homehub/ui"
)

type RemindersView struct {
	win         fyne.Window
	mainContent *fyne.Container
	content     fyne.CanvasObject
}

// NewRemindersView creates a new RemindersView instance.
func NewRemindersView(win fyne.Window, mainContent *fyne.Container) *RemindersView {
	v := &RemindersView{
		win:         win,
		mainContent: mainContent,
	}
	v.makeUI()
	return v
}

// Content returns the canvas object for the view.
func (v *RemindersView) Content() fyne.CanvasObject {
	return v.content
}

// Refresh rebuilds the UI and updates the main container.
func (v *RemindersView) Refresh() {
	v.makeUI()
	if v.mainContent != nil {
		v.mainContent.Objects = []fyne.CanvasObject{v.content}
		v.mainContent.Refresh()
	}
}

func (v *RemindersView) makeUI() {
	remindersList, err := database.GetRemindersDB()
	if err != nil {
		log.Printf("Error loading reminders: %v", err)
	}

	headerText := canvas.NewText("Reminders", color.White)
	headerText.TextSize = 22
	headerText.TextStyle.Bold = true
	headerContainer := container.NewPadded(container.NewHBox(headerText, layout.NewSpacer()))

	var listContent fyne.CanvasObject
	if len(remindersList) == 0 {
		emptyLabel := widget.NewLabel("No reminders configured. Tap '+' below to add one!")
		emptyLabel.Alignment = fyne.TextAlignCenter
		listContent = container.NewCenter(emptyLabel)
	} else {
		itemsContainer := container.NewVBox()
		for _, r := range remindersList {
			item := r // capture loop variable
			itemCard := v.buildReminderCard(item)
			itemsContainer.Add(itemCard)
		}
		listContent = container.NewVScroll(container.NewPadded(itemsContainer))
	}

	addButton := widget.NewButtonWithIcon("Add Reminder", theme.ContentAddIcon(), func() {
		v.showAddReminderDialog()
	})
	addButton.Importance = widget.HighImportance

	bottomContainer := container.New(
		layout.NewMaxLayout(),
		canvas.NewRectangle(color.Black),
		container.NewPadded(addButton),
	)

	v.content = container.NewBorder(
		headerContainer,
		bottomContainer,
		nil,
		nil,
		listContent,
	)
}

func (v *RemindersView) buildReminderCard(r database.Reminder) fyne.CanvasObject {
	titleText := canvas.NewText(r.Title, color.White)
	titleText.TextSize = 18
	titleText.TextStyle.Bold = true

	subTextStr := fmt.Sprintf("Time: %s  •  Days: %s", formatTime12Hr(r.Time), r.Days)
	subText := canvas.NewText(subTextStr, color.NRGBA{R: 200, G: 200, B: 200, A: 255})
	subText.TextSize = 14

	var statusText *canvas.Text
	if !r.Enabled {
		statusText = canvas.NewText("Disabled", color.NRGBA{R: 150, G: 150, B: 150, A: 255})
	} else if !r.Acknowledged && !r.LastTriggered.IsZero() {
		statusText = canvas.NewText("⚠️ Pending Acknowledgment", color.NRGBA{R: 255, G: 165, B: 0, A: 255})
	} else if r.Acknowledged && !r.AcknowledgedAt.IsZero() {
		statusText = canvas.NewText(fmt.Sprintf("✓ Done Today (%s)", r.AcknowledgedAt.Format("3:04 PM")), color.NRGBA{R: 50, G: 205, B: 50, A: 255})
	} else {
		statusText = canvas.NewText("Active", color.NRGBA{R: 100, G: 180, B: 255, A: 255})
	}
	statusText.TextSize = 13

	leftInfo := container.NewVBox(
		titleText,
		subText,
		statusText,
	)

	actionsBox := container.NewHBox()

	if r.Enabled && !r.Acknowledged && !r.LastTriggered.IsZero() {
		ackBtn := widget.NewButtonWithIcon("Acknowledge", theme.ConfirmIcon(), func() {
			if err := AcknowledgeReminder(r.ID); err != nil {
				log.Printf("Error acknowledging reminder: %v", err)
			}
			v.Refresh()
		})
		ackBtn.Importance = widget.HighImportance
		actionsBox.Add(ackBtn)
	}

	enableCheck := widget.NewCheck("Active", func(checked bool) {
		r.Enabled = checked
		if err := database.UpdateReminderDB(r); err != nil {
			log.Printf("Error updating reminder enabled status: %v", err)
		}
		v.Refresh()
	})
	enableCheck.SetChecked(r.Enabled)
	actionsBox.Add(enableCheck)

	editBtn := widget.NewButtonWithIcon("", theme.DocumentCreateIcon(), func() {
		v.showEditReminderDialog(r)
	})
	actionsBox.Add(editBtn)

	deleteBtn := widget.NewButtonWithIcon("", theme.DeleteIcon(), func() {
		dialogs.ShowCustomConfirm(
			"Delete Reminder",
			"Delete",
			"Cancel",
			widget.NewLabel(fmt.Sprintf("Are you sure you want to delete '%s'?", r.Title)),
			func(confirm bool) {
				if confirm {
					if err := database.DeleteReminderDB(r.ID); err != nil {
						log.Printf("Error deleting reminder: %v", err)
					}
					NotifyListeners()
					v.Refresh()
				}
			},
			v.win,
		)
	})
	actionsBox.Add(deleteBtn)

	bg := canvas.NewRectangle(color.NRGBA{R: 35, G: 35, B: 40, A: 255})
	bg.CornerRadius = 8

	cardContent := container.NewBorder(
		nil, nil, nil, actionsBox, leftInfo,
	)

	return container.New(
		layout.NewMaxLayout(),
		bg,
		container.NewPadded(cardContent),
	)
}

func (v *RemindersView) showAddReminderDialog() {
	titleEntry := ui.NewKeyboardEntry(v.win)
	titleEntry.SetPlaceHolder("e.g. Feed the dogs")

	timeEntry := ui.NewKeyboardEntry(v.win)
	timeEntry.SetPlaceHolder("HH:MM (e.g. 08:00 or 18:30)")
	timeEntry.SetText("08:00")

	daysSelect := widget.NewSelect([]string{
		"Everyday",
		"Weekdays",
		"Weekends",
		"Mon,Wed,Fri",
		"Tue,Thu",
	}, nil)
	daysSelect.SetSelected("Everyday")

	enabledCheck := widget.NewCheck("Enabled", nil)
	enabledCheck.SetChecked(true)

	errorLabel := canvas.NewText("", color.NRGBA{R: 255, G: 80, B: 80, A: 255})
	errorLabel.Hide()

	form := &widget.Form{
		Items: []*widget.FormItem{
			widget.NewFormItem("Title", titleEntry),
			widget.NewFormItem("Time (HH:MM)", timeEntry),
			widget.NewFormItem("Days", daysSelect),
			widget.NewFormItem("Active", enabledCheck),
		},
	}

	d := dialogs.NewCustomConfirm(
		"Add Reminder",
		"Add",
		"Cancel",
		container.NewVBox(form, errorLabel),
		func(confirm bool) {
			if !confirm {
				return
			}
			title := strings.TrimSpace(titleEntry.Text)
			timeStr := strings.TrimSpace(timeEntry.Text)
			days := daysSelect.Selected
			if days == "" {
				days = "Everyday"
			}

			if title == "" {
				errorLabel.Text = "Title cannot be empty"
				errorLabel.Show()
				return
			}
			var h, m int
			if _, err := fmt.Sscanf(timeStr, "%d:%d", &h, &m); err != nil || h < 0 || h > 23 || m < 0 || m > 59 {
				errorLabel.Text = "Invalid time format (must be HH:MM 00:00-23:59)"
				errorLabel.Show()
				return
			}
			formattedTime := fmt.Sprintf("%02d:%02d", h, m)

			newRem := database.Reminder{
				Title:        title,
				Time:         formattedTime,
				Days:         days,
				Enabled:      enabledCheck.Checked,
				Acknowledged: true, // initial state is clear until triggered
			}

			if _, err := database.AddReminderDB(newRem); err != nil {
				log.Printf("Error adding reminder: %v", err)
			}
			NotifyListeners()
			v.Refresh()
		},
		v.win,
	)
	d.Show()
}

func (v *RemindersView) showEditReminderDialog(r database.Reminder) {
	titleEntry := ui.NewKeyboardEntry(v.win)
	titleEntry.SetText(r.Title)

	timeEntry := ui.NewKeyboardEntry(v.win)
	timeEntry.SetText(r.Time)

	daysSelect := widget.NewSelect([]string{
		"Everyday",
		"Weekdays",
		"Weekends",
		"Mon,Wed,Fri",
		"Tue,Thu",
	}, nil)
	daysSelect.SetSelected(r.Days)

	enabledCheck := widget.NewCheck("Enabled", nil)
	enabledCheck.SetChecked(r.Enabled)

	errorLabel := canvas.NewText("", color.NRGBA{R: 255, G: 80, B: 80, A: 255})
	errorLabel.Hide()

	form := &widget.Form{
		Items: []*widget.FormItem{
			widget.NewFormItem("Title", titleEntry),
			widget.NewFormItem("Time (HH:MM)", timeEntry),
			widget.NewFormItem("Days", daysSelect),
			widget.NewFormItem("Active", enabledCheck),
		},
	}

	d := dialogs.NewCustomConfirm(
		"Edit Reminder",
		"Save",
		"Cancel",
		container.NewVBox(form, errorLabel),
		func(confirm bool) {
			if !confirm {
				return
			}
			title := strings.TrimSpace(titleEntry.Text)
			timeStr := strings.TrimSpace(timeEntry.Text)
			days := daysSelect.Selected

			if title == "" {
				errorLabel.Text = "Title cannot be empty"
				errorLabel.Show()
				return
			}
			var h, m int
			if _, err := fmt.Sscanf(timeStr, "%d:%d", &h, &m); err != nil || h < 0 || h > 23 || m < 0 || m > 59 {
				errorLabel.Text = "Invalid time format (must be HH:MM)"
				errorLabel.Show()
				return
			}

			r.Title = title
			r.Time = fmt.Sprintf("%02d:%02d", h, m)
			r.Days = days
			r.Enabled = enabledCheck.Checked

			if err := database.UpdateReminderDB(r); err != nil {
				log.Printf("Error updating reminder: %v", err)
			}
			NotifyListeners()
			v.Refresh()
		},
		v.win,
	)
	d.Show()
}

// CreateRemindersView creates the view for sidebar navigation.
func CreateRemindersView(win fyne.Window, mainContent *fyne.Container) (fyne.CanvasObject, func()) {
	v := NewRemindersView(win, mainContent)
	return v.Content(), v.Refresh
}

// CreatePhotoOverlayView builds the overlay container that pops up over the Home (Photos) view when reminders are active.
func CreatePhotoOverlayView() *fyne.Container {
	overlayContent := container.NewVBox()

	outerBox := container.NewPadded(overlayContent)

	updateOverlay := func() {
		pending, err := GetPendingReminders()
		if err != nil || len(pending) == 0 {
			overlayContent.Objects = nil
			outerBox.Hide()
			return
		}

		overlayContent.Objects = nil

		bannerHeader := canvas.NewText("🔔 REMINDERS", color.NRGBA{R: 255, G: 215, B: 0, A: 255})
		bannerHeader.TextSize = 20
		bannerHeader.TextStyle.Bold = true
		bannerHeader.Alignment = fyne.TextAlignCenter

		overlayContent.Add(container.NewCenter(bannerHeader))

		for _, r := range pending {
			item := r // capture loop variable
			title := canvas.NewText(item.Title, color.White)
			title.TextSize = 22
			title.TextStyle.Bold = true

			sub := canvas.NewText(fmt.Sprintf("Scheduled for %s", formatTime12Hr(item.Time)), color.NRGBA{R: 220, G: 220, B: 220, A: 255})
			sub.TextSize = 16

			ackBtn := widget.NewButtonWithIcon("Acknowledge", theme.ConfirmIcon(), func() {
				if err := AcknowledgeReminder(item.ID); err != nil {
					log.Printf("Error acknowledging reminder %d: %v", item.ID, err)
				}
			})
			ackBtn.Importance = widget.HighImportance

			cardBg := canvas.NewRectangle(color.NRGBA{R: 25, G: 30, B: 45, A: 240})
			cardBg.CornerRadius = 12

			itemBox := container.NewBorder(
				nil, nil,
				container.NewVBox(title, sub),
				ackBtn,
			)

			cardContainer := container.New(
				layout.NewMaxLayout(),
				cardBg,
				container.NewPadded(itemBox),
			)

			overlayContent.Add(container.NewPadded(cardContainer))
		}

		outerBox.Show()
		outerBox.Refresh()
	}

	RegisterChangeListener(func() {
		fyne.Do(func() {
			updateOverlay()
		})
	})

	updateOverlay()

	// Wrap outerBox in top-centered container for photos view overlay
	return container.New(
		layout.NewCustomPaddedLayout(20, 20, 20, 20),
		container.NewVBox(
			container.NewCenter(outerBox),
		),
	)
}

func formatTime12Hr(time24 string) string {
	var h, m int
	if _, err := fmt.Sscanf(time24, "%d:%d", &h, &m); err != nil {
		return time24
	}
	ampm := "AM"
	if h >= 12 {
		ampm = "PM"
	}
	h12 := h % 12
	if h12 == 0 {
		h12 = 12
	}
	return fmt.Sprintf("%d:%02d %s", h12, m, ampm)
}
