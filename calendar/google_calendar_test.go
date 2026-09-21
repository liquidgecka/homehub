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

package calendar

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	gcalendar "google.golang.org/api/calendar/v3"

	"github.com/liquidgecka/homehub/config"
)

func TestGetCalendarIDsToFetch(t *testing.T) {
	// Empty config
	cfgEmpty := config.GoogleCalendarConfig{}
	ids, err := getCalendarIDsToFetch(nil, cfgEmpty)
	if err != nil || len(ids) != 0 {
		t.Errorf("expected empty IDs for empty config, got %v (err: %v)", ids, err)
	}

	// Placeholder config
	cfgPlaceholder := config.GoogleCalendarConfig{
		CalendarIDs: []string{"YOUR_CALENDAR_ID_1"},
	}
	ids, err = getCalendarIDsToFetch(nil, cfgPlaceholder)
	if err != nil || len(ids) != 0 {
		t.Errorf("expected empty IDs for placeholder config, got %v", ids)
	}

	// Valid config
	cfgValid := config.GoogleCalendarConfig{
		CalendarIDs: []string{"cal1@gmail.com", "cal2@gmail.com"},
	}
	ids, err = getCalendarIDsToFetch(nil, cfgValid)
	if err != nil || len(ids) != 2 {
		t.Errorf("expected 2 IDs for valid config, got %v", ids)
	}
}

func TestGetEventsForTodayAndWeek(t *testing.T) {
	mockEvents := &gcalendar.Events{
		Items: []*gcalendar.Event{
			{
				Id:      "ev1",
				Summary: "Morning Meeting",
				Start:   &gcalendar.EventDateTime{DateTime: time.Now().Format(time.RFC3339)},
				End:     &gcalendar.EventDateTime{DateTime: time.Now().Add(time.Hour).Format(time.RFC3339)},
			},
		},
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(mockEvents)
	})

	srv, ts := newMockCalendarService(t, handler)
	defer ts.Close()

	cfg := config.GoogleCalendarConfig{
		CalendarIDs:            []string{"primary"},
		CalendarRefreshMinutes: 5,
	}

	// Reset cache
	calendarCache = &CalendarCache{}

	// Test GetEventsForToday
	events, err := GetEventsForToday(srv, cfg)
	if err != nil {
		t.Fatalf("GetEventsForToday error: %v", err)
	}
	if len(events) != 1 || events[0].Summary != "Morning Meeting" {
		t.Errorf("unexpected events: %v", events)
	}

	// Verify cache hit
	cachedEvents, err := GetEventsForToday(srv, cfg)
	if err != nil || len(cachedEvents) != 1 {
		t.Errorf("cache hit failed: %v", err)
	}

	// Test GetEventsForWeek
	weekEvents, err := GetEventsForWeek(srv, cfg, time.Now())
	if err != nil {
		t.Fatalf("GetEventsForWeek error: %v", err)
	}
	if len(weekEvents) != 1 {
		t.Errorf("expected 1 week event, got %d", len(weekEvents))
	}

	// Test GetEventsForMonth
	monthEvents, err := GetEventsForMonth(srv, cfg, time.Now())
	if err != nil {
		t.Fatalf("GetEventsForMonth error: %v", err)
	}
	if len(monthEvents) != 1 {
		t.Errorf("expected 1 month event, got %d", len(monthEvents))
	}
}

func TestAddEvent(t *testing.T) {
	createdEvent := &gcalendar.Event{
		Id:      "new-id",
		Summary: "Dentist",
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(createdEvent)
	})

	srv, ts := newMockCalendarService(t, handler)
	defer ts.Close()

	ev, err := AddEvent(srv, "primary", &gcalendar.Event{Summary: "Dentist"})
	if err != nil {
		t.Fatalf("AddEvent failed: %v", err)
	}
	if ev.Id != "new-id" || ev.Summary != "Dentist" {
		t.Errorf("unexpected created event: %v", ev)
	}
}
