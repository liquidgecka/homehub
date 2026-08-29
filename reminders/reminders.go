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
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/liquidgecka/homehub/database"
)

var (
	listeners   []func()
	listenersMu sync.Mutex
)

// RegisterChangeListener adds a callback function when reminder states change.
func RegisterChangeListener(fn func()) {
	listenersMu.Lock()
	defer listenersMu.Unlock()
	listeners = append(listeners, fn)
}

// ClearListeners clears registered change listeners (useful for testing).
func ClearListeners() {
	listenersMu.Lock()
	defer listenersMu.Unlock()
	listeners = nil
}

// NotifyListeners notifies all registered change listeners.
func NotifyListeners() {
	listenersMu.Lock()
	callbacks := make([]func(), len(listeners))
	copy(callbacks, listeners)
	listenersMu.Unlock()

	for _, fn := range callbacks {
		if fn != nil {
			fn()
		}
	}
}

// ShouldTrigger checks if a reminder should be triggered given the
// reference time.
func ShouldTrigger(r database.Reminder, now time.Time) bool {
	if !r.Enabled {
		return false
	}

	// Check day of week
	if !DayMatches(r.Days, now.Weekday()) {
		return false
	}

	// Parse configured time HH:MM
	var hour, min int
	if _, err := fmt.Sscanf(r.Time, "%d:%d", &hour, &min); err != nil {
		return false
	}

	nowHour, nowMin, _ := now.Clock()
	nowMinutes := nowHour*60 + nowMin
	scheduledMinutes := hour*60 + min

	// Must be at or past the scheduled time today
	if nowMinutes < scheduledMinutes {
		return false
	}

	// If it has already been triggered today (same calendar date), don't
	// trigger again
	if !r.LastTriggered.IsZero() {
		lastY, lastM, lastD := r.LastTriggered.In(now.Location()).Date()
		nowY, nowM, nowD := now.Date()
		if lastY == nowY && lastM == nowM && lastD == nowD {
			return false
		}
	}

	return true
}

// DayMatches checks whether the weekday matches the days string setting.
func DayMatches(daysStr string, day time.Weekday) bool {
	daysStr = strings.TrimSpace(daysStr)
	if daysStr == "" ||
		strings.EqualFold(daysStr, "Everyday") ||
		strings.EqualFold(daysStr, "Daily") {
		return true
	}

	shortName := day.String()[:3] // e.g. "Mon", "Tue"
	fullOfWeek := day.String()    // e.g. "Monday"

	if strings.EqualFold(daysStr, "Weekdays") {
		return day >= time.Monday && day <= time.Friday
	}
	if strings.EqualFold(daysStr, "Weekends") {
		return day == time.Saturday || day == time.Sunday
	}

	parts := strings.Split(daysStr, ",")
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if strings.EqualFold(p, shortName) ||
			strings.EqualFold(p, fullOfWeek) {
			return true
		}
	}

	return false
}

// CheckAndTriggerReminders checks all reminders and triggers any that are due.
func CheckAndTriggerReminders(now time.Time) error {
	reminders, err := database.GetRemindersDB()
	if err != nil {
		return fmt.Errorf("failed to get reminders: %w", err)
	}

	triggeredAny := false
	for _, r := range reminders {
		if ShouldTrigger(r, now) {
			log.Printf(
				"Triggering reminder ID %d: %s at %s",
				r.ID, r.Title, now.Format("15:04"),
			)
			if err := database.SetReminderTriggeredDB(r.ID, now); err != nil {
				log.Printf(
					"ERROR: Failed to set reminder %d triggered: %v",
					r.ID, err,
				)
			} else {
				triggeredAny = true
			}
		}
	}

	if triggeredAny {
		NotifyListeners()
	}
	return nil
}

// GetPendingReminders returns all active reminders that have been triggered but
// not yet acknowledged.
func GetPendingReminders() ([]database.Reminder, error) {
	reminders, err := database.GetRemindersDB()
	if err != nil {
		return nil, err
	}

	var pending []database.Reminder
	for _, r := range reminders {
		if r.Enabled && !r.Acknowledged && !r.LastTriggered.IsZero() {
			pending = append(pending, r)
		}
	}
	return pending, nil
}

// AcknowledgeReminder marks a reminder as acknowledged.
func AcknowledgeReminder(id int) error {
	err := database.SetReminderAcknowledgedDB(id, true, time.Now())
	if err == nil {
		NotifyListeners()
	}
	return err
}

// StartBackgroundChecker returns a function suitable for task.Task execution.
func StartBackgroundChecker() func(ctx context.Context) {
	return func(ctx context.Context) {
		if err := CheckAndTriggerReminders(time.Now()); err != nil {
			log.Printf("Error checking reminders: %v", err)
		}
	}
}
