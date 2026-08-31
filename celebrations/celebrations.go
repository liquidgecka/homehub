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
	"log"
	"math/rand"
	"sync"
	"time"

	"github.com/liquidgecka/homehub/database"
)

var (
	listeners        []func()
	listenersMu      sync.Mutex
	displayHandlers  []func(c database.Celebration)
	displayHandlerMu sync.Mutex
)

// RegisterChangeListener registers a callback when celebrations are modified.
func RegisterChangeListener(fn func()) {
	listenersMu.Lock()
	defer listenersMu.Unlock()
	listeners = append(listeners, fn)
}

// ClearListeners clears all change listeners (useful for testing).
func ClearListeners() {
	listenersMu.Lock()
	defer listenersMu.Unlock()
	listeners = nil
}

// NotifyListeners notifies all change listeners that celebrations changed.
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

// RegisterDisplayHandler registers a callback invoked when a celebration
// should be presented on screen.
func RegisterDisplayHandler(fn func(c database.Celebration)) {
	displayHandlerMu.Lock()
	defer displayHandlerMu.Unlock()
	displayHandlers = append(displayHandlers, fn)
}

// ClearDisplayHandlers clears all display handlers (useful for testing).
func ClearDisplayHandlers() {
	displayHandlerMu.Lock()
	defer displayHandlerMu.Unlock()
	displayHandlers = nil
}

// TriggerCelebration immediately triggers the presentation of a celebration.
func TriggerCelebration(c database.Celebration) {
	displayHandlerMu.Lock()
	handlers := make([]func(c database.Celebration), len(displayHandlers))
	copy(handlers, displayHandlers)
	displayHandlerMu.Unlock()

	for _, h := range handlers {
		if h != nil {
			h(c)
		}
	}
}

// ShouldCelebrate returns true if the celebration is active on the given date.
func ShouldCelebrate(c database.Celebration, now time.Time) bool {
	if !c.Enabled {
		return false
	}
	if int(now.Month()) != c.Month || now.Day() != c.Day {
		return false
	}
	if c.Year > 0 && now.Year() != c.Year {
		return false
	}
	return true
}

// GetActiveCelebrations returns all celebrations that match the given date.
func GetActiveCelebrations(now time.Time) ([]database.Celebration, error) {
	all, err := database.GetCelebrationsDB()
	if err != nil {
		return nil, err
	}

	var active []database.Celebration
	for _, c := range all {
		if ShouldCelebrate(c, now) {
			active = append(active, c)
		}
	}
	return active, nil
}

// TriggerRandomCelebration checks for active celebrations and triggers one at
// random if available. Returns true if a celebration was triggered.
func TriggerRandomCelebration(now time.Time) bool {
	active, err := GetActiveCelebrations(now)
	if err != nil {
		log.Printf("ERROR: Failed to retrieve active celebrations: %v", err)
		return false
	}
	if len(active) == 0 {
		return false
	}

	chosen := active[rand.Intn(len(active))]
	log.Printf(
		"Triggering celebration overlay: '%s' (%s)",
		chosen.Title, chosen.Type,
	)
	TriggerCelebration(chosen)
	return true
}

// SchedulerConfig controls intervals for random celebration triggers.
type SchedulerConfig struct {
	MinInterval time.Duration
	MaxInterval time.Duration
}

// DefaultSchedulerConfig ensures random triggers happen at least once every
// 10 minutes.
var DefaultSchedulerConfig = SchedulerConfig{
	MinInterval: 2 * time.Minute,
	MaxInterval: 8 * time.Minute,
}

// StartScheduler starts a background loop that randomly triggers celebration
// overlays throughout celebration days, at least once every 10 minutes.
func StartScheduler(
	parentCtx context.Context, cfg ...SchedulerConfig,
) context.CancelFunc {
	config := DefaultSchedulerConfig
	if len(cfg) > 0 {
		config = cfg[0]
	}
	if config.MaxInterval <= config.MinInterval {
		config.MaxInterval = config.MinInterval + time.Minute
	}

	ctx, cancel := context.WithCancel(parentCtx)

	go func() {
		log.Println("Celebrations random scheduler started.")
		defer log.Println("Celebrations random scheduler stopped.")

		for {
			delta := config.MaxInterval - config.MinInterval
			randomOffset := time.Duration(0)
			if delta > 0 {
				randomOffset = time.Duration(rand.Int63n(int64(delta)))
			}
			waitDuration := config.MinInterval + randomOffset

			select {
			case <-ctx.Done():
				return
			case <-time.After(waitDuration):
				TriggerRandomCelebration(time.Now())
			}
		}
	}()

	return cancel
}
