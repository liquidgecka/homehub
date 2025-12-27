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
	gcalendar "google.golang.org/api/calendar/v3"
)

// CachedEvents stores the most recently fetched calendar events.
var CachedEvents []*gcalendar.Event

// CalendarEventsUpdatedChan is a channel to signal when calendar events have been updated.
var CalendarEventsUpdatedChan = make(chan bool, 1) // Buffered channel to avoid blocking sender

// SetCachedEvents updates the global cache of calendar events.
var SetCachedEvents = func(events []*gcalendar.Event) {
	CachedEvents = events
}

// NotifyEventsUpdated sends a signal on the channel that calendar events have been updated.
var NotifyEventsUpdated = func() {
	select {
	case CalendarEventsUpdatedChan <- true:
		// Sent successfully
	default:
		// Channel is full, meaning the receiver hasn't processed the previous signal yet.
		// This is fine, we just want to signal that an update is needed.
	}
}
