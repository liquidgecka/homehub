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

package dpms

import (
	"context"
	"log"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/liquidgecka/homehub/config"
	"github.com/liquidgecka/homehub/logging"
)

const reassertInterval = 60 * time.Second

var (
	timeNow         = time.Now
	checkAndSetDPMS = _checkAndSetDPMS

	stateMu         sync.RWMutex
	displayOn       = true
	stateKnown      = false
	lastCommandTime time.Time
	stateListeners  []func(on bool)
)

// IsDisplayOn returns true if DPMS currently considers the display on.
func IsDisplayOn() bool {
	stateMu.RLock()
	defer stateMu.RUnlock()
	return displayOn
}

// InitDisplayState sets the initial display power state without marking the
// schedule state as known or notifying listeners.
func InitDisplayState(on bool) {
	stateMu.Lock()
	defer stateMu.Unlock()
	displayOn = on
}

// SetDisplayOn updates the tracked display power state and notifies registered
// listeners if the state changed or was not yet known.
func SetDisplayOn(on bool) {
	stateMu.Lock()
	changed := !stateKnown || displayOn != on
	displayOn = on
	stateKnown = true
	var listeners []func(on bool)
	if changed && len(stateListeners) > 0 {
		listeners = make([]func(on bool), len(stateListeners))
		copy(listeners, stateListeners)
	}
	stateMu.Unlock()

	for _, fn := range listeners {
		if fn != nil {
			fn(on)
		}
	}
}

// RegisterStateListener registers a callback invoked when display power state
// changes.
func RegisterStateListener(fn func(on bool)) {
	if fn == nil {
		return
	}
	stateMu.Lock()
	defer stateMu.Unlock()
	stateListeners = append(stateListeners, fn)
}

// ClearStateListeners removes all registered state listeners.
func ClearStateListeners() {
	stateMu.Lock()
	defer stateMu.Unlock()
	stateListeners = nil
}

// ResetState restores default DPMS tracking state and clears listeners.
func ResetState() {
	stateMu.Lock()
	defer stateMu.Unlock()
	displayOn = true
	stateKnown = false
	lastCommandTime = time.Time{}
	stateListeners = nil
}

// StartScheduler starts a ticker that checks the time periodically and
// enables or disables DPMS based on the schedule in the config file.
func StartScheduler(
	parentCtx context.Context, cfg *config.DPMSConfig,
) context.CancelFunc {
	log.Println("DPMS: Starting scheduler...")
	ctx, cancel := context.WithCancel(parentCtx)
	interval := time.Duration(cfg.CheckIntervalSeconds) * time.Second
	if interval <= 0 {
		interval = 15 * time.Second
	}
	ticker := time.NewTicker(interval)
	go func() {
		defer log.Println("DPMS scheduler goroutine terminated.")
		// Check DPMS state immediately at startup
		checkAndSetDPMS(cfg)
		for {
			select {
			case <-ctx.Done():
				ticker.Stop()
				return
			case <-ticker.C:
				checkAndSetDPMS(cfg)
			}
		}
	}()
	return cancel
}

// ShouldBeOn evaluates whether the display should be powered on at the given
// time according to the configured DPMS schedule.
func ShouldBeOn(cfg *config.DPMSConfig, now time.Time) bool {
	if cfg == nil {
		return true
	}

	logging.Debugf(
		"DPMS: Checking schedule at current time: %s",
		now.Format("15:04:05"),
	)
	for _, period := range cfg.OnPeriods {
		if len(period) != 2 {
			log.Printf(
				"DPMS: Invalid on_period entry: %v. Must have 2 elements.",
				period,
			)
			continue
		}
		logging.Debugf("DPMS: Evaluating period: %v", period)
		startTime, err := time.Parse("15:04", period[0])
		if err != nil {
			log.Printf("DPMS: Invalid start time in on_period: %v", err)
			continue
		}
		endTime, err := time.Parse("15:04", period[1])
		if err != nil {
			log.Printf("DPMS: Invalid end time in on_period: %v", err)
			continue
		}

		startTime = time.Date(
			now.Year(), now.Month(), now.Day(),
			startTime.Hour(), startTime.Minute(), 0, 0, now.Location(),
		)
		endTime = time.Date(
			now.Year(), now.Month(), now.Day(),
			endTime.Hour(), endTime.Minute(), 0, 0, now.Location(),
		)
		logging.Debugf(
			"DPMS: Comparing with start: %s, end: %s",
			startTime.Format("15:04:05"), endTime.Format("15:04:05"),
		)

		// Check if the period spans overnight
		if startTime.After(endTime) {
			// e.g., Start: 22:00, End: 07:00
			// "On" period is after the start time OR before the end time.
			if now.After(startTime) || now.Before(endTime) {
				return true
			}
		} else {
			// Same-day period, e.g., Start: 07:00, End: 22:00
			if now.After(startTime) && now.Before(endTime) {
				return true
			}
		}
	}

	return false
}

func _checkAndSetDPMS(cfg *config.DPMSConfig) {
	now := timeNow()
	shouldBeOn := ShouldBeOn(cfg, now)

	stateMu.Lock()
	changed := !stateKnown || displayOn != shouldBeOn
	shouldRunCmd := changed ||
		lastCommandTime.IsZero() ||
		now.Sub(lastCommandTime) >= reassertInterval ||
		now.Before(lastCommandTime)
	var listeners []func(on bool)
	if changed {
		displayOn = shouldBeOn
		stateKnown = true
		if len(stateListeners) > 0 {
			listeners = make([]func(on bool), len(stateListeners))
			copy(listeners, stateListeners)
		}
	}
	if shouldRunCmd {
		lastCommandTime = now
	}
	stateMu.Unlock()

	if changed {
		stateLabel := "OFF"
		if shouldBeOn {
			stateLabel = "ON"
		}
		log.Printf("DPMS: Display state -> %s", stateLabel)
	}

	if shouldRunCmd {
		if shouldBeOn {
			setDPMS(cfg.OnCommand)
		} else {
			setDPMS(cfg.OffCommand)
		}
	}

	if changed {
		for _, fn := range listeners {
			if fn != nil {
				fn(shouldBeOn)
			}
		}
	}
}

var setDPMS = func(cmdArgs []string) {
	if len(cmdArgs) == 0 {
		logging.Debugf("DPMS: No command configured.")
		return
	}

	cmd := exec.Command(cmdArgs[0], cmdArgs[1:]...)
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	logging.Debugf("DPMS: Executing command: %s", cmd.String())
	err := cmd.Run()
	outStr, errStr := stdout.String(), stderr.String()

	if err != nil {
		log.Printf("DPMS: Failed to execute command. Error: %v", err)
		log.Printf("DPMS: Command output (stdout): %s", outStr)
		log.Printf("DPMS: Command output (stderr): %s", errStr)
		return
	}

	logging.Debugf("DPMS: Successfully executed command.")
	if len(outStr) > 0 {
		logging.Debugf("DPMS: Command output (stdout): %s", outStr)
	}
	if len(errStr) > 0 {
		logging.Debugf("DPMS: Command output (stderr): %s", errStr)
	}
}
