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
	"fmt"
	"log"
	"sync"
	"time"

	"google.golang.org/api/calendar/v3"

	"github.com/liquidgecka/homehub/config"
	"github.com/liquidgecka/homehub/google"
)

// CalendarCache stores cached calendar event data and the time it was last fetched.
type CalendarCache struct {
	data        []*calendar.Event
	lastFetched time.Time
	mutex       sync.RWMutex
}

var calendarCache = &CalendarCache{}
var calendarService *calendar.Service

// InitGoogleCalendarClient initializes the Google Calendar API client using the unified OAuth2 client.
func InitGoogleCalendarClient() (*calendar.Service, error) {
	client, err := google.GetGoogleHTTPClient()
	if err != nil {
		return nil, fmt.Errorf("unable to get unified Google client for Calendar: %w", err)
	}

	srv, err := calendar.New(client)
	if err != nil {
		return nil, fmt.Errorf("Unable to retrieve Calendar client: %v", err)
	}
	calendarService = srv
	return srv, nil
}

// GetCalendarService returns the cached calendar service.
func GetCalendarService() *calendar.Service {
	return calendarService
}

// getCalendarIDsToFetch determines which calendar IDs to fetch based on configuration.
func getCalendarIDsToFetch(srv *calendar.Service, cfg config.GoogleCalendarConfig) ([]string, error) {
	hasIDs := len(cfg.CalendarIDs) > 0 && cfg.CalendarIDs[0] != "YOUR_CALENDAR_ID_1"

	if !hasIDs {
		log.Println("No `calendar_ids` configured in config.toml. When using a service account, you must explicitly specify the IDs of the calendars you want to access. Automatic discovery of shared calendars is not supported by the Google Calendar API for service accounts.")
		return []string{}, nil
	}

	log.Printf("Loading events from configured calendars: %v\n", cfg.CalendarIDs)
	return cfg.CalendarIDs, nil
}

// GetEventsForToday fetches events for the current day.
func GetEventsForToday(srv *calendar.Service, cfg config.GoogleCalendarConfig) ([]*calendar.Event, error) {
	// Check cache validity
	calendarCache.mutex.RLock()
	if calendarCache.data != nil && time.Since(calendarCache.lastFetched).Minutes() < float64(cfg.CalendarRefreshMinutes) {
		defer calendarCache.mutex.RUnlock()
		return calendarCache.data, nil
	}
	calendarCache.mutex.RUnlock()

	// Cache is expired or empty, fetch new data
	calendarCache.mutex.Lock()
	defer calendarCache.mutex.Unlock()

	// Re-check cache in case another goroutine already updated it while we were waiting for the Lock
	if calendarCache.data != nil && time.Since(calendarCache.lastFetched).Minutes() < float64(cfg.CalendarRefreshMinutes) {
		return calendarCache.data, nil
	}

	calendarIDs, err := getCalendarIDsToFetch(srv, cfg)
	if err != nil {
		return nil, err
	}
	if len(calendarIDs) == 0 {
		return []*calendar.Event{}, nil // No calendars to fetch from
	}

	t := time.Now()
	timeMin := t.Format(time.RFC3339)
	timeMax := t.Add(24 * time.Hour).Format(time.RFC3339)

	var allEvents []*calendar.Event
	for _, calendarID := range calendarIDs {
		events, err := srv.Events.List(calendarID).ShowDeleted(false).SingleEvents(true).
			TimeMin(timeMin).TimeMax(timeMax).OrderBy("startTime").Do()
		if err != nil {
			log.Printf("Unable to retrieve events for calendar %s: %v", calendarID, err)
			continue
		}
		allEvents = append(allEvents, events.Items...)
	}

	// Update cache
	calendarCache.data = allEvents
	calendarCache.lastFetched = time.Now()

	return allEvents, nil
}

// GetEventsForWeek fetches events for the week containing the given date.
func GetEventsForWeek(srv *calendar.Service, cfg config.GoogleCalendarConfig, dayInWeek time.Time) ([]*calendar.Event, error) {
	calendarIDs, err := getCalendarIDsToFetch(srv, cfg)
	if err != nil {
		return nil, err
	}
	if len(calendarIDs) == 0 {
		return []*calendar.Event{}, nil
	}

	year, month, day := dayInWeek.Date()
	today := time.Date(year, month, day, 0, 0, 0, 0, dayInWeek.Location())
	startOfWeek := today.AddDate(0, 0, -int(today.Weekday())) // Assumes Sunday is the start of the week
	endOfWeek := startOfWeek.AddDate(0, 0, 7).Add(-time.Second)

	var allEvents []*calendar.Event
	for _, calendarID := range calendarIDs {
		events, err := srv.Events.List(calendarID).ShowDeleted(false).SingleEvents(true).
			TimeMin(startOfWeek.Format(time.RFC3339)).TimeMax(endOfWeek.Format(time.RFC3339)).OrderBy("startTime").Do()
		if err != nil {
			log.Printf("Unable to retrieve events for calendar %s: %v", calendarID, err)
			continue
		}
		allEvents = append(allEvents, events.Items...)
	}
	return allEvents, nil
}

// GetEventsForMonth fetches events for the given month.
var GetEventsForMonth = func(srv *calendar.Service, cfg config.GoogleCalendarConfig, month time.Time) ([]*calendar.Event, error) {
	calendarIDs, err := getCalendarIDsToFetch(srv, cfg)
	if err != nil {
		return nil, err
	}
	if len(calendarIDs) == 0 {
		return []*calendar.Event{}, nil // No calendars to fetch from
	}

	year, m, _ := month.Date()
	startOfMonth := time.Date(year, m, 1, 0, 0, 0, 0, month.Location())
	endOfMonth := startOfMonth.AddDate(0, 1, 0).Add(-time.Second)

	var allEvents []*calendar.Event
	for _, calendarID := range calendarIDs {
		events, err := srv.Events.List(calendarID).ShowDeleted(false).SingleEvents(true).
			TimeMin(startOfMonth.Format(time.RFC3339)).TimeMax(endOfMonth.Format(time.RFC3339)).OrderBy("startTime").Do()
		if err != nil {
			log.Printf("Unable to retrieve events for calendar %s: %v", calendarID, err)
			continue
		}
		allEvents = append(allEvents, events.Items...)
	}

	return allEvents, nil
}

// AddEvent adds a new event to a specified calendar.
func AddEvent(srv *calendar.Service, calendarID string, event *calendar.Event) (*calendar.Event, error) {
	newEvent, err := srv.Events.Insert(calendarID, event).Do()
	if err != nil {
		return nil, fmt.Errorf("Unable to add event to calendar %s: %v", calendarID, err)
	}
	return newEvent, nil
}
