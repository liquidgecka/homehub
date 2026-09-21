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
	"context"
	"net/http"
	"testing"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
	gcalendar "google.golang.org/api/calendar/v3"

	"github.com/liquidgecka/homehub/config"
)

func TestGetEventStartAndEndTime(t *testing.T) {
	// RFC3339 DateTime
	dtStart := "2026-09-20T10:00:00Z"
	dtEnd := "2026-09-20T11:30:00Z"
	evDateTime := &gcalendar.Event{
		Start: &gcalendar.EventDateTime{DateTime: dtStart},
		End:   &gcalendar.EventDateTime{DateTime: dtEnd},
	}

	st := getEventStartTime(evDateTime)
	et := getEventEndTime(evDateTime)
	if st.Format(time.RFC3339) != dtStart {
		t.Errorf("expected start %s, got %s", dtStart, st.Format(time.RFC3339))
	}
	if et.Format(time.RFC3339) != dtEnd {
		t.Errorf("expected end %s, got %s", dtEnd, et.Format(time.RFC3339))
	}
	dur := getEventDuration(evDateTime)
	if dur != 90*time.Minute {
		t.Errorf("expected duration 90m, got %v", dur)
	}

	// Date only
	dStart := "2026-09-20"
	dEnd := "2026-09-21"
	evDate := &gcalendar.Event{
		Start: &gcalendar.EventDateTime{Date: dStart},
		End:   &gcalendar.EventDateTime{Date: dEnd},
	}
	stDate := getEventStartTime(evDate)
	etDate := getEventEndTime(evDate)
	if stDate.Format("2006-01-02") != dStart {
		t.Errorf("expected start %s, got %s", dStart, stDate.Format("2006-01-02"))
	}
	if etDate.Format("2006-01-02") != dEnd {
		t.Errorf("expected end %s, got %s", dEnd, etDate.Format("2006-01-02"))
	}

	// Nil / empty times
	evEmpty := &gcalendar.Event{}
	stEmpty := getEventStartTime(evEmpty)
	if !stEmpty.IsZero() {
		t.Errorf("expected zero time for empty event start, got %v", stEmpty)
	}
	etEmpty := getEventEndTime(evEmpty)
	if etEmpty.Sub(stEmpty) != time.Hour {
		t.Errorf("expected default 1 hour end time, got %v", etEmpty)
	}

	// Malformed (end before start)
	evMalformed := &gcalendar.Event{
		Start: &gcalendar.EventDateTime{DateTime: "2026-09-20T12:00:00Z"},
		End:   &gcalendar.EventDateTime{DateTime: "2026-09-20T10:00:00Z"},
	}
	if getEventDuration(evMalformed) != 0 {
		t.Errorf(
			"expected duration 0 for malformed event, got %v",
			getEventDuration(evMalformed),
		)
	}
}

func TestEventsOverlapAndDaysInMonth(t *testing.T) {
	cfg := &config.Config{}
	e1 := newEventBox(&gcalendar.Event{
		Summary: "Event 1",
		Start:   &gcalendar.EventDateTime{DateTime: "2026-09-20T10:00:00Z"},
		End:     &gcalendar.EventDateTime{DateTime: "2026-09-20T12:00:00Z"},
	}, cfg)
	e2 := newEventBox(&gcalendar.Event{
		Summary: "Event 2",
		Start:   &gcalendar.EventDateTime{DateTime: "2026-09-20T11:00:00Z"},
		End:     &gcalendar.EventDateTime{DateTime: "2026-09-20T13:00:00Z"},
	}, cfg)
	e3 := newEventBox(&gcalendar.Event{
		Summary: "Event 3",
		Start:   &gcalendar.EventDateTime{DateTime: "2026-09-20T13:00:00Z"},
		End:     &gcalendar.EventDateTime{DateTime: "2026-09-20T14:00:00Z"},
	}, cfg)

	if !eventsOverlap(e1, e2) {
		t.Errorf("expected e1 and e2 to overlap")
	}
	if eventsOverlap(e1, e3) {
		t.Errorf("expected e1 and e3 not to overlap")
	}

	// daysInMonth
	if daysInMonth(2024, time.February) != 29 {
		t.Errorf(
			"expected 29 days in Feb 2024 (leap), got %d",
			daysInMonth(2024, time.February),
		)
	}
	if daysInMonth(2026, time.February) != 28 {
		t.Errorf(
			"expected 28 days in Feb 2026, got %d",
			daysInMonth(2026, time.February),
		)
	}
	if daysInMonth(2026, time.September) != 30 {
		t.Errorf(
			"expected 30 days in Sep 2026, got %d",
			daysInMonth(2026, time.September),
		)
	}
}

func TestHourLinesAndLayout(t *testing.T) {
	test.NewApp()

	hl := newHourLines(40.0)
	r := hl.CreateRenderer()
	minSize := r.MinSize()
	if minSize.Width != 0 || minSize.Height != 0 {
		t.Errorf("expected min size (0, 0), got %v", minSize)
	}
	r.Layout(fyne.NewSize(200, 40.0*24))
	r.Refresh()
	if len(r.Objects()) != 25 {
		t.Errorf(
			"expected 25 lines (24 hours + current time), got %d",
			len(r.Objects()),
		)
	}
	r.Destroy()

	whLayout := &weekHeaderLayout{}
	lbl1 := widget.NewLabel("L1")
	lbl2 := widget.NewLabel("L2")
	objects := []fyne.CanvasObject{lbl1, lbl2}
	whMin := whLayout.MinSize(objects)
	if whMin.Width <= 0 || whMin.Height <= 0 {
		t.Errorf("invalid weekHeaderLayout min size: %v", whMin)
	}
	whLayout.Layout(objects, fyne.NewSize(300, 50))

	ug := newUnexpandingGrid(lbl1)
	ugR := ug.CreateRenderer()
	ugMin := ugR.MinSize()
	if ugMin.Width <= 0 && ugMin.Height <= 0 {
		t.Errorf("expected non-empty ugMin")
	}

	dl := &dayLayout{hourHeight: 40}
	dlMin := dl.MinSize([]fyne.CanvasObject{lbl1})
	if dlMin.Height != 40*24 {
		t.Errorf("expected dayLayout min height %v, got %v", 40*24, dlMin.Height)
	}
}

func TestGenerateGridsAndCalendarView(t *testing.T) {
	test.NewApp()
	win := test.NewWindow(widget.NewLabel("Calendar Test"))
	defer win.Close()

	cfg := &config.Config{
		Google: config.GoogleConfig{
			Calendar: config.GoogleCalendarConfig{
				CalendarIDs: []string{"test-cal"},
				TimeFormat:  "3:04 PM",
			},
		},
	}

	now := time.Now()
	testEvents := []*gcalendar.Event{
		{
			Summary: "Team Standup",
			Start:   &gcalendar.EventDateTime{DateTime: now.Format(time.RFC3339)},
			End:     &gcalendar.EventDateTime{DateTime: now.Add(time.Hour).Format(time.RFC3339)},
		},
		{
			Summary: "Lunch",
			Start:   &gcalendar.EventDateTime{DateTime: now.Add(2 * time.Hour).Format(time.RFC3339)},
			End:     &gcalendar.EventDateTime{DateTime: now.Add(3 * time.Hour).Format(time.RFC3339)},
		},
	}
	CachedEvents = testEvents

	weekObj := generateWeekGrid(context.Background(), now, nil, cfg)
	if weekObj == nil {
		t.Fatal("expected non-nil week object from generateWeekGrid")
	}

	origGetMonth := GetEventsForMonth
	defer func() { GetEventsForMonth = origGetMonth }()
	GetEventsForMonth = func(
		srv *gcalendar.Service,
		c config.GoogleCalendarConfig,
		m time.Time,
	) ([]*gcalendar.Event, error) {
		return testEvents, nil
	}

	monthObj := generateCalendarGrid(now, nil, cfg)
	if monthObj == nil {
		t.Fatal("expected non-nil month object from generateCalendarGrid")
	}

	// Test nil calService view
	nilView, _, _ := CreateCalendarView(nil)
	if nilView == nil {
		t.Fatal("expected non-nil view for nil calService")
	}

	// Test with mock calService
	srv, ts := newMockCalendarService(
		t,
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}),
	)
	defer ts.Close()

	calView, cancel, _ := CreateCalendarView(srv)
	if calView == nil {
		t.Fatal("expected non-nil calendar view from CreateCalendarView")
	}
	time.Sleep(100 * time.Millisecond)
	if cancel != nil {
		cancel()
	}
}

func TestShowEventDetailsDialog(t *testing.T) {
	test.NewApp()
	win := test.NewWindow(widget.NewLabel("Event Details Test"))
	defer win.Close()

	now := time.Now()
	ev := &gcalendar.Event{
		Summary:     "Test Event",
		Description: "Event Description",
		Location:    "Online",
		Start:       &gcalendar.EventDateTime{DateTime: now.Format(time.RFC3339)},
		End:         &gcalendar.EventDateTime{DateTime: now.Add(time.Hour).Format(time.RFC3339)},
	}
	showEventDetailsDialog(win, ev)
}
