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

package background

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/liquidgecka/homehub/calendar"
	"github.com/liquidgecka/homehub/config"
	"github.com/liquidgecka/homehub/database"
	"github.com/liquidgecka/homehub/photomanager"
	"github.com/liquidgecka/homehub/reminders"
	"github.com/liquidgecka/homehub/task"
	"github.com/liquidgecka/homehub/weather"
)

var (
	// osRemove is a package-level variable that can be reassigned for testing os.Remove.
	osRemove = os.Remove
)

// Manager holds the task scheduler and manages all background tasks.
type Manager struct {
	scheduler *task.Scheduler
}

// NewManager creates a new background task manager.
func NewManager() *Manager {
	return &Manager{
		scheduler: task.NewScheduler(),
	}
}

// Init initializes all the background tasks.
func (m *Manager) Init() {
	cfg := config.GetConfig()

	// Photo Cleanup Task
	m.scheduler.AddTask(&task.Task{
		Name:         "Photo Cleanup",
		InitialDelay: 5 * time.Second,
		Interval:     12 * time.Hour,
		Task:         photoCleanupTask(cfg),
	})

	// Calendar Events Task
	m.scheduler.AddTask(&task.Task{
		Name:         "Calendar Events",
		InitialDelay: 0,
		Interval:     time.Duration(cfg.Google.Calendar.CalendarRefreshMinutes) * time.Minute,
		Task:         loadCalendarEventsTask(cfg),
	})

	// Weather Data Task
	m.scheduler.AddTask(&task.Task{
		Name:         "Weather Data",
		InitialDelay: 0,
		Interval:     time.Duration(cfg.OpenWeather.RefreshMinutes) * time.Minute,
		Task:         loadWeatherDataTask(cfg),
	})

	// Shopping Store Cleanup Task
	m.scheduler.AddTask(&task.Task{
		Name:         "Shopping Store Cleanup",
		InitialDelay: 10 * time.Second,
		Interval:     24 * time.Hour,
		Task:         shoppingStoreCleanupTask(),
	})

	// Reminders Task
	m.scheduler.AddTask(&task.Task{
		Name:         "Reminders Check",
		InitialDelay: 0,
		Interval:     15 * time.Second,
		Task:         reminders.StartBackgroundChecker(),
	})
}

// Start starts all background tasks.
func (m *Manager) Start() {
	m.scheduler.Start()
}

// Stop stops all background tasks.
func (m *Manager) Stop() {
	m.scheduler.Stop()
}

func photoCleanupTask(cfg *config.Config) task.Func {
	return func(ctx context.Context) {
		log.Println("Starting hidden photo cleanup task...")
		photomanager.CleanupHiddenPhotos(cfg.LocalPhotos.Directory)
	}
}

func loadCalendarEventsTask(cfg *config.Config) task.Func {
	return func(ctx context.Context) {
		log.Println("Loading calendar events...")
		events, err := calendar.GetEventsForMonth(
			calendar.GetCalendarService(), cfg.Google.Calendar, time.Now())
		if err != nil {
			log.Printf("Failed to fetch calendar events: %v", err)
		} else {
			calendar.SetCachedEvents(events)
			log.Println("Calendar events loaded.")
			calendar.NotifyEventsUpdated()
		}
	}
}

func loadWeatherDataTask(cfg *config.Config) task.Func {
	return func(ctx context.Context) {
		if _, err := weather.GetWeather(cfg); err != nil {
			log.Printf("Failed to fetch weather data: %v", err)
		} else {
			log.Println("Weather data loaded.")
		}
	}
}

func shoppingStoreCleanupTask() task.Func {
	return func(ctx context.Context) {
		log.Println("Starting shopping store cleanup task...")
		threshold := time.Now().Add(-30 * 24 * time.Hour)
		expiredStoreIDs, err := database.GetExpiredShoppingStoreIDs(threshold)
		if err != nil {
			log.Printf("ERROR: Failed to get expired shopping store IDs: %v", err)
		} else {
			for _, storeID := range expiredStoreIDs {
				log.Printf("Deleting expired store with ID %d", storeID)
				if err := database.DeleteShoppingItemsByStore(storeID); err != nil {
					log.Printf("ERROR: Failed to delete shopping items for store %d: %v", storeID, err)
				}
				if err := database.DeleteShoppingStoreMetadata(storeID); err != nil {
					log.Printf("ERROR: Failed to delete shopping store metadata for store %d: %v", storeID, err)
				}
			}
		}
	}
}
