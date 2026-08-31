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
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/liquidgecka/homehub/database"
)

func TestShouldCelebrate(t *testing.T) {
	tests := []struct {
		name        string
		celebration database.Celebration
		checkTime   time.Time
		want        bool
	}{
		{
			name: "Enabled and matching month and day (annual)",
			celebration: database.Celebration{
				Month:   8,
				Day:     30,
				Year:    0,
				Enabled: true,
			},
			checkTime: time.Date(2026, time.August, 30, 10, 0, 0, 0, time.UTC),
			want:      true,
		},
		{
			name: "Disabled celebration",
			celebration: database.Celebration{
				Month:   8,
				Day:     30,
				Year:    0,
				Enabled: false,
			},
			checkTime: time.Date(2026, time.August, 30, 10, 0, 0, 0, time.UTC),
			want:      false,
		},
		{
			name: "Different day",
			celebration: database.Celebration{
				Month:   8,
				Day:     30,
				Year:    0,
				Enabled: true,
			},
			checkTime: time.Date(2026, time.August, 31, 10, 0, 0, 0, time.UTC),
			want:      false,
		},
		{
			name: "Different month",
			celebration: database.Celebration{
				Month:   8,
				Day:     30,
				Year:    0,
				Enabled: true,
			},
			checkTime: time.Date(2026, time.September, 30, 10, 0, 0, 0, time.UTC),
			want:      false,
		},
		{
			name: "Specific year matching",
			celebration: database.Celebration{
				Month:   6,
				Day:     15,
				Year:    2026,
				Enabled: true,
			},
			checkTime: time.Date(2026, time.June, 15, 12, 0, 0, 0, time.UTC),
			want:      true,
		},
		{
			name: "Specific year mismatched",
			celebration: database.Celebration{
				Month:   6,
				Day:     15,
				Year:    2025,
				Enabled: true,
			},
			checkTime: time.Date(2026, time.June, 15, 12, 0, 0, 0, time.UTC),
			want:      false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ShouldCelebrate(tc.celebration, tc.checkTime)
			if got != tc.want {
				t.Errorf("ShouldCelebrate() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestListeners(t *testing.T) {
	ClearListeners()
	var count int32

	RegisterChangeListener(func() {
		atomic.AddInt32(&count, 1)
	})

	NotifyListeners()
	if atomic.LoadInt32(&count) != 1 {
		t.Fatalf("Expected listener call count 1, got %d", count)
	}

	ClearListeners()
	NotifyListeners()
	if atomic.LoadInt32(&count) != 1 {
		t.Fatalf("Expected listener call count to remain 1 after ClearListeners, got %d", count)
	}
}

func TestDisplayHandlersAndTrigger(t *testing.T) {
	ClearDisplayHandlers()
	var received database.Celebration

	RegisterDisplayHandler(func(c database.Celebration) {
		received = c
	})

	testCelebration := database.Celebration{
		ID:      42,
		Title:   "Birthday Bash",
		Type:    "birthday",
		Message: "Happy Birthday!",
		Enabled: true,
	}

	TriggerCelebration(testCelebration)

	if received.ID != 42 || received.Title != "Birthday Bash" {
		t.Fatalf("Expected received celebration ID 42, got %+v", received)
	}

	ClearDisplayHandlers()
}

func TestGetActiveCelebrationsAndTriggerRandom(t *testing.T) {
	_, cleanup, err := database.NewTestDB()
	if err != nil {
		t.Fatalf("Failed to create test DB: %v", err)
	}
	defer cleanup()

	targetDate := time.Date(2026, time.August, 30, 14, 0, 0, 0, time.UTC)

	// Add celebrations: one matching today, one for tomorrow, one disabled
	_, _ = database.AddCelebrationDB(database.Celebration{
		Title:   "Brady's Birthday",
		Type:    "birthday",
		Month:   8,
		Day:     30,
		Year:    0,
		Message: "Happy Birthday Brady!",
		Enabled: true,
	})
	_, _ = database.AddCelebrationDB(database.Celebration{
		Title:   "Tomorrow's Event",
		Type:    "party",
		Month:   8,
		Day:     31,
		Year:    0,
		Message: "Tomorrow party",
		Enabled: true,
	})
	_, _ = database.AddCelebrationDB(database.Celebration{
		Title:   "Disabled Celebration",
		Type:    "anniversary",
		Month:   8,
		Day:     30,
		Year:    0,
		Message: "Disabled",
		Enabled: false,
	})

	active, err := GetActiveCelebrations(targetDate)
	if err != nil {
		t.Fatalf("GetActiveCelebrations failed: %v", err)
	}
	if len(active) != 1 {
		t.Fatalf("Expected 1 active celebration, got %d", len(active))
	}
	if active[0].Title != "Brady's Birthday" {
		t.Errorf("Unexpected active celebration: %+v", active[0])
	}

	ClearDisplayHandlers()
	var triggeredCount int32
	RegisterDisplayHandler(func(c database.Celebration) {
		atomic.AddInt32(&triggeredCount, 1)
	})

	// Trigger on today should succeed
	ok := TriggerRandomCelebration(targetDate)
	if !ok {
		t.Error("Expected TriggerRandomCelebration to return true")
	}
	if atomic.LoadInt32(&triggeredCount) != 1 {
		t.Errorf("Expected display handler triggered once, got %d", triggeredCount)
	}

	// Trigger on another day should return false
	anotherDay := time.Date(2026, time.September, 1, 10, 0, 0, 0, time.UTC)
	okNoEvents := TriggerRandomCelebration(anotherDay)
	if okNoEvents {
		t.Error("Expected TriggerRandomCelebration to return false on day without events")
	}
}

func TestStartScheduler(t *testing.T) {
	_, cleanup, err := database.NewTestDB()
	if err != nil {
		t.Fatalf("Failed to create test DB: %v", err)
	}
	defer cleanup()

	now := time.Now()
	_, _ = database.AddCelebrationDB(database.Celebration{
		Title:   "Quick Scheduler Test",
		Type:    "birthday",
		Month:   int(now.Month()),
		Day:     now.Day(),
		Year:    0,
		Message: "Party Time!",
		Enabled: true,
	})

	ClearDisplayHandlers()
	var count int32
	RegisterDisplayHandler(func(c database.Celebration) {
		atomic.AddInt32(&count, 1)
	})

	cfg := SchedulerConfig{
		MinInterval: 10 * time.Millisecond,
		MaxInterval: 25 * time.Millisecond,
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	stopSched := StartScheduler(ctx, cfg)
	defer stopSched()

	time.Sleep(100 * time.Millisecond)

	if atomic.LoadInt32(&count) == 0 {
		t.Error("Expected scheduler to trigger celebration at least once")
	}
}

func TestGetBannerTextAndIcons(t *testing.T) {
	b1 := GetBannerText(database.Celebration{Type: "birthday"})
	if b1 != "🎈 HAPPY BIRTHDAY! 🎈" {
		t.Errorf("Unexpected banner for birthday: %s", b1)
	}

	b2 := GetBannerText(database.Celebration{Type: "anniversary"})
	if b2 != "💍 HAPPY ANNIVERSARY! 💍" {
		t.Errorf("Unexpected banner for anniversary: %s", b2)
	}

	b3 := GetBannerText(database.Celebration{Type: "party"})
	if b3 != "🎉 CELEBRATION! 🎉" {
		t.Errorf("Unexpected banner for party: %s", b3)
	}

	b4 := GetBannerText(database.Celebration{Type: "holiday"})
	if b4 != "🌟 HAPPY HOLIDAYS! 🌟" {
		t.Errorf("Unexpected banner for holiday: %s", b4)
	}

	b5 := GetBannerText(database.Celebration{Type: "graduation"})
	if b5 != "🎓 CONGRATULATIONS GRADUATE! 🎓" {
		t.Errorf("Unexpected banner for graduation: %s", b5)
	}

	b6 := GetBannerText(database.Celebration{Type: "school"})
	if b6 != "🎒 FIRST DAY OF SCHOOL! 🎒" {
		t.Errorf("Unexpected banner for school: %s", b6)
	}

	b7 := GetBannerText(database.Celebration{Type: "other"})
	if b7 != "🎉 CELEBRATION 🎉" {
		t.Errorf("Unexpected banner for custom: %s", b7)
	}

	res1 := GetIconForType("birthday")
	if res1 == nil {
		t.Error("Expected icon for birthday")
	}
	res2 := GetIconForType("anniversary")
	if res2 == nil {
		t.Error("Expected icon for anniversary")
	}
	res3 := GetIconForType("graduation")
	if res3 == nil {
		t.Error("Expected icon for graduation")
	}
	res4 := GetIconForType("school")
	if res4 == nil {
		t.Error("Expected icon for school")
	}
	res5 := GetIconForType("unknown")
	if res5 == nil {
		t.Error("Expected icon for unknown fallback")
	}
}
