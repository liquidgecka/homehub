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
	"testing"
	"time"

	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"

	"github.com/liquidgecka/homehub/database"
)

func setupTestDB(t *testing.T) func() {
	t.Helper()
	_, cleanup, err := database.NewTestDB()
	if err != nil {
		t.Fatalf("Failed to initialize test db: %v", err)
	}
	ClearListeners()
	return cleanup
}

func TestDayMatches(t *testing.T) {
	monday := time.Date(2026, 7, 27, 10, 0, 0, 0, time.UTC) // Monday
	sunday := time.Date(2026, 7, 26, 10, 0, 0, 0, time.UTC) // Sunday

	tests := []struct {
		daysStr  string
		weekday  time.Weekday
		expected bool
	}{
		{"Everyday", monday.Weekday(), true},
		{"Daily", sunday.Weekday(), true},
		{"Weekdays", monday.Weekday(), true},
		{"Weekdays", sunday.Weekday(), false},
		{"Weekends", monday.Weekday(), false},
		{"Weekends", sunday.Weekday(), true},
		{"Mon,Wed,Fri", monday.Weekday(), true},
		{"Tue,Thu", monday.Weekday(), false},
		{"Monday", monday.Weekday(), true},
	}

	for _, tt := range tests {
		got := DayMatches(tt.daysStr, tt.weekday)
		if got != tt.expected {
			t.Errorf(
				"DayMatches(%q, %v) = %v, want %v",
				tt.daysStr, tt.weekday, got, tt.expected,
			)
		}
	}
}

func TestShouldTrigger(t *testing.T) {
	now := time.Date(2026, 7, 26, 8, 30, 0, 0, time.Local)

	// Disabled reminder
	rDisabled := database.Reminder{
		Enabled: false,
		Time:    "08:00",
		Days:    "Everyday",
	}
	if ShouldTrigger(rDisabled, now) {
		t.Error("Disabled reminder should not trigger")
	}

	// Active reminder before scheduled time
	rFuture := database.Reminder{
		Enabled: true,
		Time:    "09:00",
		Days:    "Everyday",
	}
	if ShouldTrigger(rFuture, now) {
		t.Error("Future reminder should not trigger")
	}

	// Active reminder at/after scheduled time
	rDue := database.Reminder{
		Enabled: true,
		Time:    "08:00",
		Days:    "Everyday",
	}
	if !ShouldTrigger(rDue, now) {
		t.Error("Due reminder should trigger")
	}

	// Already triggered today
	rAlreadyTriggered := database.Reminder{
		Enabled:       true,
		Time:          "08:00",
		Days:          "Everyday",
		LastTriggered: time.Date(2026, 7, 26, 8, 0, 0, 0, time.Local),
	}
	if ShouldTrigger(rAlreadyTriggered, now) {
		t.Error("Already triggered today reminder should not trigger again")
	}

	// Already triggered today (with LastTriggered in UTC, e.g. after local
	// evening conversion)
	locMDT := time.FixedZone("MDT", -6*3600)
	nowEvening := time.Date(2026, 7, 26, 20, 30, 0, 0, locMDT)
	// 20:00 MDT on July 26 = 02:00 UTC July 27
	lastUTC := time.Date(2026, 7, 27, 2, 0, 0, 0, time.UTC)
	rAlreadyTriggeredUTC := database.Reminder{
		Enabled:       true,
		Time:          "20:00",
		Days:          "Everyday",
		LastTriggered: lastUTC,
	}
	if ShouldTrigger(rAlreadyTriggeredUTC, nowEvening) {
		t.Error(
			"Already triggered today reminder with UTC timestamp " +
				"should not trigger again",
		)
	}

	// Triggered yesterday, due today
	rTriggeredYesterday := database.Reminder{
		Enabled:       true,
		Time:          "08:00",
		Days:          "Everyday",
		LastTriggered: time.Date(2026, 7, 25, 8, 0, 0, 0, time.Local),
	}
	if !ShouldTrigger(rTriggeredYesterday, now) {
		t.Error("Triggered yesterday reminder should trigger today")
	}
}

func TestCheckAndTriggerAndAcknowledge(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	var listenerCalled bool
	RegisterChangeListener(func() {
		listenerCalled = true
	})

	id, err := database.AddReminderDB(database.Reminder{
		Title:        "Feed the dogs",
		Time:         "08:00",
		Days:         "Everyday",
		Enabled:      true,
		Acknowledged: true,
	})
	if err != nil {
		t.Fatalf("AddReminderDB failed: %v", err)
	}

	now := time.Date(2026, 7, 26, 8, 15, 0, 0, time.Local)
	if err := CheckAndTriggerReminders(now); err != nil {
		t.Fatalf("CheckAndTriggerReminders failed: %v", err)
	}

	if !listenerCalled {
		t.Error("Expected change listener to be called when reminder triggers")
	}

	pending, err := GetPendingReminders()
	if err != nil {
		t.Fatalf("GetPendingReminders failed: %v", err)
	}
	if len(pending) != 1 {
		t.Fatalf("Expected 1 pending reminder, got %d", len(pending))
	}
	if pending[0].ID != id || pending[0].Title != "Feed the dogs" {
		t.Errorf("Pending reminder mismatch: %+v", pending[0])
	}

	// Acknowledge reminder
	listenerCalled = false
	if err := AcknowledgeReminder(id); err != nil {
		t.Fatalf("AcknowledgeReminder failed: %v", err)
	}
	if !listenerCalled {
		t.Error(
			"Expected change listener to be called when reminder acknowledged",
		)
	}

	pendingAfter, err := GetPendingReminders()
	if err != nil {
		t.Fatalf("GetPendingReminders failed: %v", err)
	}
	if len(pendingAfter) != 0 {
		t.Errorf(
			"Expected 0 pending reminders after acknowledge, got %d",
			len(pendingAfter),
		)
	}
}

func TestFormatTime12Hr(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"00:00", "12:00 AM"},
		{"08:30", "8:30 AM"},
		{"12:00", "12:00 PM"},
		{"18:45", "6:45 PM"},
		{"invalid", "invalid"},
	}

	for _, tt := range tests {
		got := formatTime12Hr(tt.input)
		if got != tt.expected {
			t.Errorf(
				"formatTime12Hr(%q) = %q, want %q",
				tt.input, got, tt.expected,
			)
		}
	}
}

func TestStartBackgroundChecker(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	checker := StartBackgroundChecker()
	if checker == nil {
		t.Fatal("StartBackgroundChecker returned nil")
	}
	checker(nil)
}

func TestCreateRemindersViewAndOverlay(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	// Pre-populate multiple enabled reminders
	_, _ = database.AddReminderDB(database.Reminder{
		Title:   "Morning Reminder",
		Time:    "07:00",
		Days:    "Everyday",
		Enabled: true,
	})
	_, _ = database.AddReminderDB(database.Reminder{
		Title:   "Afternoon Reminder",
		Time:    "12:00",
		Days:    "Everyday",
		Enabled: true,
	})

	app := test.NewApp()
	_ = app
	win := test.NewWindow(widget.NewLabel("Test Reminders"))
	defer win.Close()

	mainContent := container.NewMax()

	// Ensure creating the view doesn't trigger spurious change notifications
	// or infinite loops
	view, _ := CreateRemindersView(win, mainContent)
	if view == nil {
		t.Error("CreateRemindersView returned nil view")
	}

	v := NewRemindersView(win, mainContent)
	if v == nil || v.Content() == nil {
		t.Error("NewRemindersView returned invalid view")
	}
	v.Refresh()

	overlayObj := CreatePhotoOverlayView()
	if overlayObj == nil {
		t.Error("CreatePhotoOverlayView returned nil container")
	}
}
