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
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/liquidgecka/homehub/config"
)

func TestCheckAndSetDPMS(t *testing.T) {
	originalSetDPMS := setDPMS
	defer func() { setDPMS = originalSetDPMS }()
	defer ResetState()

	var lastCmd []string
	setDPMS = func(cmdArgs []string) {
		lastCmd = cmdArgs
	}

	onCmd := []string{"on"}
	offCmd := []string{"off"}

	testCases := []struct {
		name        string
		cfg         *config.DPMSConfig
		now         time.Time
		expectedCmd []string
	}{
		{
			name: "Same day, during on-period",
			cfg: &config.DPMSConfig{
				OnPeriods:  [][2]string{{"07:00", "22:00"}},
				OnCommand:  onCmd,
				OffCommand: offCmd,
			},
			now:         time.Date(2023, 1, 1, 15, 0, 0, 0, time.Local),
			expectedCmd: onCmd,
		},
		{
			name: "Same day, before on-period",
			cfg: &config.DPMSConfig{
				OnPeriods:  [][2]string{{"07:00", "22:00"}},
				OnCommand:  onCmd,
				OffCommand: offCmd,
			},
			now:         time.Date(2023, 1, 1, 6, 0, 0, 0, time.Local),
			expectedCmd: offCmd,
		},
		{
			name: "Same day, after on-period",
			cfg: &config.DPMSConfig{
				OnPeriods:  [][2]string{{"07:00", "22:00"}},
				OnCommand:  onCmd,
				OffCommand: offCmd,
			},
			now:         time.Date(2023, 1, 1, 23, 0, 0, 0, time.Local),
			expectedCmd: offCmd,
		},
		{
			name: "Overnight, during on-period (after start)",
			cfg: &config.DPMSConfig{
				OnPeriods:  [][2]string{{"22:00", "07:00"}},
				OnCommand:  onCmd,
				OffCommand: offCmd,
			},
			now:         time.Date(2023, 1, 1, 23, 0, 0, 0, time.Local),
			expectedCmd: onCmd,
		},
		{
			name: "Overnight, during on-period (before end)",
			cfg: &config.DPMSConfig{
				OnPeriods:  [][2]string{{"22:00", "07:00"}},
				OnCommand:  onCmd,
				OffCommand: offCmd,
			},
			now:         time.Date(2023, 1, 1, 2, 0, 0, 0, time.Local),
			expectedCmd: onCmd,
		},
		{
			name: "Overnight, during off-period",
			cfg: &config.DPMSConfig{
				OnPeriods:  [][2]string{{"22:00", "07:00"}},
				OnCommand:  onCmd,
				OffCommand: offCmd,
			},
			now:         time.Date(2023, 1, 1, 12, 0, 0, 0, time.Local),
			expectedCmd: offCmd,
		},
		{
			name: "Multiple periods, in first period",
			cfg: &config.DPMSConfig{
				OnPeriods:  [][2]string{{"06:00", "09:00"}, {"17:00", "23:00"}},
				OnCommand:  onCmd,
				OffCommand: offCmd,
			},
			now:         time.Date(2023, 1, 1, 8, 0, 0, 0, time.Local),
			expectedCmd: onCmd,
		},
		{
			name: "Multiple periods, in second period",
			cfg: &config.DPMSConfig{
				OnPeriods:  [][2]string{{"06:00", "09:00"}, {"17:00", "23:00"}},
				OnCommand:  onCmd,
				OffCommand: offCmd,
			},
			now:         time.Date(2023, 1, 1, 18, 0, 0, 0, time.Local),
			expectedCmd: onCmd,
		},
		{
			name: "Multiple periods, between periods",
			cfg: &config.DPMSConfig{
				OnPeriods:  [][2]string{{"06:00", "09:00"}, {"17:00", "23:00"}},
				OnCommand:  onCmd,
				OffCommand: offCmd,
			},
			now:         time.Date(2023, 1, 1, 12, 0, 0, 0, time.Local),
			expectedCmd: offCmd,
		},
		{
			name: "Invalid period length",
			cfg: &config.DPMSConfig{
				OnPeriods:  [][2]string{{"07:00"}},
				OnCommand:  onCmd,
				OffCommand: offCmd,
			},
			now:         time.Date(2023, 1, 1, 8, 0, 0, 0, time.Local),
			expectedCmd: offCmd, // Should default to off
		},
		{
			name: "Invalid start time",
			cfg: &config.DPMSConfig{
				OnPeriods:  [][2]string{{"bad-time", "22:00"}},
				OnCommand:  onCmd,
				OffCommand: offCmd,
			},
			now:         time.Date(2023, 1, 1, 15, 0, 0, 0, time.Local),
			expectedCmd: offCmd, // Should default to off
		},
		{
			name: "Invalid end time",
			cfg: &config.DPMSConfig{
				OnPeriods:  [][2]string{{"07:00", "bad-time"}},
				OnCommand:  onCmd,
				OffCommand: offCmd,
			},
			now:         time.Date(2023, 1, 1, 15, 0, 0, 0, time.Local),
			expectedCmd: offCmd, // Should default to off
		},
	}

	originalNow := timeNow
	defer func() { timeNow = originalNow }()

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ResetState()
			lastCmd = nil
			timeNow = func() time.Time { return tc.now }
			checkAndSetDPMS(tc.cfg)
			assert.Equal(t, tc.expectedCmd, lastCmd)
		})
	}
}

func TestStartScheduler(t *testing.T) {
	// Mock checkAndSetDPMS to count calls
	var callCount int32
	originalCheck := checkAndSetDPMS
	checkAndSetDPMS = func(cfg *config.DPMSConfig) {
		atomic.AddInt32(&callCount, 1)
	}
	defer func() { checkAndSetDPMS = originalCheck }()

	cfg := &config.DPMSConfig{
		CheckIntervalSeconds: 1, // Use a short interval for testing
	}

	ctx, cancel := context.WithCancel(context.Background())
	_ = StartScheduler(ctx, cfg) // Start the scheduler

	// Wait for a short period to allow the ticker to fire at least once
	time.Sleep(1500 * time.Millisecond)
	cancel() // Stop the scheduler

	// The function should be called immediately, and then at least once
	// by the ticker.
	if atomic.LoadInt32(&callCount) < 2 {
		t.Errorf(
			"Expected checkAndSetDPMS called >= 2 times, got %d",
			callCount,
		)
	}
}

func TestSetDPMS(t *testing.T) {
	// Keep the original setDPMS function
	originalSetDPMS := setDPMS
	defer func() { setDPMS = originalSetDPMS }()

	t.Run("No Command", func(t *testing.T) {
		// This test ensures calling with no command doesn't panic.
		originalSetDPMS(nil)
		originalSetDPMS([]string{})
	})

	t.Run("Success", func(t *testing.T) {
		// Using "true" which is a command that should always succeed with no output.
		cmdArgs := []string{"true"}
		originalSetDPMS(cmdArgs)
		// No error means success, can't assert much more without log capture.
	})

	t.Run("Error", func(t *testing.T) {
		// Using a command that should not exist.
		cmdArgs := []string{"a-very-unlikely-command-to-exist"}
		originalSetDPMS(cmdArgs)
		// This should log an error. We can't assert it without log capture,
		// but we ensure it doesn't panic.
	})
}

func TestShouldBeOn(t *testing.T) {
	now := time.Date(2026, 5, 1, 12, 0, 0, 0, time.Local)
	assert.True(t, ShouldBeOn(nil, now))

	sameDayCfg := &config.DPMSConfig{
		OnPeriods: [][2]string{{"07:00", "22:00"}},
	}
	assert.True(
		t,
		ShouldBeOn(
			sameDayCfg,
			time.Date(2026, 5, 1, 12, 0, 0, 0, time.Local),
		),
	)
	assert.False(
		t,
		ShouldBeOn(
			sameDayCfg,
			time.Date(2026, 5, 1, 23, 0, 0, 0, time.Local),
		),
	)

	overnightCfg := &config.DPMSConfig{
		OnPeriods: [][2]string{{"22:00", "07:00"}},
	}
	assert.True(
		t,
		ShouldBeOn(
			overnightCfg,
			time.Date(2026, 5, 1, 23, 30, 0, 0, time.Local),
		),
	)
	assert.True(
		t,
		ShouldBeOn(
			overnightCfg,
			time.Date(2026, 5, 1, 6, 30, 0, 0, time.Local),
		),
	)
	assert.False(
		t,
		ShouldBeOn(
			overnightCfg,
			time.Date(2026, 5, 1, 12, 0, 0, 0, time.Local),
		),
	)
}

func TestDPMSStateAndListeners(t *testing.T) {
	originalSetDPMS := setDPMS
	originalNow := timeNow
	defer func() {
		setDPMS = originalSetDPMS
		timeNow = originalNow
		ResetState()
	}()
	setDPMS = func(cmdArgs []string) {}

	ResetState()
	assert.True(t, IsDisplayOn())

	var events []bool
	var seenInsideListener []bool
	RegisterStateListener(func(on bool) {
		events = append(events, on)
		seenInsideListener = append(seenInsideListener, IsDisplayOn())
	})

	// InitDisplayState updates state without marking stateKnown or notifying.
	InitDisplayState(false)
	assert.False(t, IsDisplayOn())
	assert.Empty(t, events)

	// SetDisplayOn with same value fires once because stateKnown was false.
	SetDisplayOn(false)
	assert.False(t, IsDisplayOn())
	assert.Equal(t, []bool{false}, events)
	assert.Equal(t, []bool{false}, seenInsideListener)

	// Repeating the same state should not re-notify listeners.
	SetDisplayOn(false)
	assert.Equal(t, []bool{false}, events)

	// Transitioning to ON via _checkAndSetDPMS notifies listeners.
	cfg := &config.DPMSConfig{
		OnPeriods:  [][2]string{{"07:00", "22:00"}},
		OnCommand:  []string{"on"},
		OffCommand: []string{"off"},
	}
	timeNow = func() time.Time {
		return time.Date(2026, 5, 1, 12, 0, 0, 0, time.Local)
	}
	_checkAndSetDPMS(cfg)
	assert.True(t, IsDisplayOn())
	assert.Equal(t, []bool{false, true}, events)
	assert.Equal(t, []bool{false, true}, seenInsideListener)

	// Re-checking while still ON should not notify listeners again.
	_checkAndSetDPMS(cfg)
	assert.Equal(t, []bool{false, true}, events)

	// Transitioning to OFF via _checkAndSetDPMS notifies listeners.
	timeNow = func() time.Time {
		return time.Date(2026, 5, 1, 23, 0, 0, 0, time.Local)
	}
	_checkAndSetDPMS(cfg)
	assert.False(t, IsDisplayOn())
	assert.Equal(t, []bool{false, true, false}, events)
	assert.Equal(t, []bool{false, true, false}, seenInsideListener)
}

func TestDPMSCommandThrottling(t *testing.T) {
	originalSetDPMS := setDPMS
	originalNow := timeNow
	defer func() {
		setDPMS = originalSetDPMS
		timeNow = originalNow
		ResetState()
	}()

	ResetState()

	var cmdCalls [][]string
	setDPMS = func(cmdArgs []string) {
		cmdCalls = append(cmdCalls, cmdArgs)
	}

	var listenerCalls int
	RegisterStateListener(func(on bool) {
		listenerCalls++
	})

	cfg := &config.DPMSConfig{
		OnPeriods:  [][2]string{{"07:00", "22:00"}},
		OnCommand:  []string{"on"},
		OffCommand: []string{"off"},
	}

	baseTime := time.Date(2026, 5, 1, 12, 0, 0, 0, time.Local)
	current := baseTime
	timeNow = func() time.Time { return current }

	// Initial check runs command and notifies listeners.
	_checkAndSetDPMS(cfg)
	assert.Len(t, cmdCalls, 1)
	assert.Equal(t, []string{"on"}, cmdCalls[0])
	assert.Equal(t, 1, listenerCalls)

	// 15s later with unchanged state: skips command and listener.
	current = baseTime.Add(15 * time.Second)
	_checkAndSetDPMS(cfg)
	assert.Len(t, cmdCalls, 1)
	assert.Equal(t, 1, listenerCalls)

	// 60s later with unchanged state: reasserts command without notifying.
	current = baseTime.Add(60 * time.Second)
	_checkAndSetDPMS(cfg)
	assert.Len(t, cmdCalls, 2)
	assert.Equal(t, []string{"on"}, cmdCalls[1])
	assert.Equal(t, 1, listenerCalls)

	// 15s later with state transition to OFF: runs immediately and notifies.
	current = time.Date(2026, 5, 1, 22, 30, 0, 0, time.Local)
	_checkAndSetDPMS(cfg)
	assert.Len(t, cmdCalls, 3)
	assert.Equal(t, []string{"off"}, cmdCalls[2])
	assert.Equal(t, 2, listenerCalls)
}

func TestAwayMode(t *testing.T) {
	originalSetDPMS := setDPMS
	originalNow := timeNow
	defer func() {
		setDPMS = originalSetDPMS
		timeNow = originalNow
		ResetState()
	}()

	cfg := &config.DPMSConfig{
		OnPeriods:  [][2]string{{"07:00", "22:00"}},
		OnCommand:  []string{"on"},
		OffCommand: []string{"off"},
	}

	t.Run("ShouldBeOn respects away mode", func(t *testing.T) {
		ResetState()
		onTime := time.Date(2026, 5, 1, 12, 0, 0, 0, time.Local)

		assert.True(t, ShouldBeOn(nil, onTime))
		assert.True(t, ShouldBeOn(cfg, onTime))

		InitAwayMode(true)
		assert.True(t, IsAway())
		assert.False(t, ShouldBeOn(nil, onTime))
		assert.False(t, ShouldBeOn(cfg, onTime))

		InitAwayMode(false)
		assert.False(t, IsAway())
		assert.True(t, ShouldBeOn(cfg, onTime))
	})

	t.Run(
		"SetAwayMode toggles during OnPeriod and persists across ticks",
		func(t *testing.T) {
			ResetState()
			SetConfig(cfg)

			var cmds [][]string
			setDPMS = func(cmdArgs []string) {
				cmds = append(cmds, cmdArgs)
			}

			current := time.Date(2026, 5, 1, 12, 0, 0, 0, time.Local)
			timeNow = func() time.Time { return current }

			var awayEvents []bool
			var stateEvents []bool
			RegisterAwayListener(func(away bool) {
				awayEvents = append(awayEvents, away)
			})
			RegisterStateListener(func(on bool) {
				stateEvents = append(stateEvents, on)
			})

			_checkAndSetDPMS(cfg)
			assert.True(t, IsDisplayOn())
			assert.Equal(t, [][]string{{"on"}}, cmds)
			assert.Equal(t, []bool{true}, stateEvents)

			// Enable away mode during active OnPeriod.
			SetAwayMode(true)
			assert.True(t, IsAway())
			assert.False(t, IsDisplayOn())
			assert.Equal(t, [][]string{{"on"}, {"off"}}, cmds)
			assert.Equal(t, []bool{true}, awayEvents)
			assert.Equal(t, []bool{true, false}, stateEvents)

			// Subsequent scheduler ticks during OnPeriod must keep display OFF.
			current = current.Add(15 * time.Second)
			_checkAndSetDPMS(cfg)
			assert.False(t, IsDisplayOn())
			assert.Equal(t, []bool{true, false}, stateEvents)

			current = current.Add(reassertInterval)
			_checkAndSetDPMS(cfg)
			assert.False(t, IsDisplayOn())
			assert.Equal(t, [][]string{{"on"}, {"off"}, {"off"}}, cmds)

			// Disable away mode during active OnPeriod: turns display back ON.
			SetAwayMode(false)
			assert.False(t, IsAway())
			assert.True(t, IsDisplayOn())
			assert.Equal(t, []bool{true, false}, awayEvents)
			assert.Equal(t, []bool{true, false, true}, stateEvents)
			assert.Equal(
				t,
				[][]string{{"on"}, {"off"}, {"off"}, {"on"}},
				cmds,
			)
		})

	t.Run("SetAwayMode outside OnPeriods", func(t *testing.T) {
		ResetState()
		SetConfig(cfg)

		var cmds [][]string
		setDPMS = func(cmdArgs []string) {
			cmds = append(cmds, cmdArgs)
		}

		current := time.Date(2026, 5, 1, 23, 0, 0, 0, time.Local)
		timeNow = func() time.Time { return current }

		var awayEvents []bool
		RegisterAwayListener(func(away bool) {
			awayEvents = append(awayEvents, away)
		})

		_checkAndSetDPMS(cfg)
		assert.False(t, IsDisplayOn())
		assert.Equal(t, [][]string{{"off"}}, cmds)

		// Enabling away during scheduled off hours still forces OffCommand.
		SetAwayMode(true)
		assert.True(t, IsAway())
		assert.False(t, IsDisplayOn())
		assert.Equal(t, []bool{true}, awayEvents)
		assert.Equal(t, [][]string{{"off"}, {"off"}}, cmds)

		// Disabling away during scheduled off hours keeps display OFF.
		SetAwayMode(false)
		assert.False(t, IsAway())
		assert.False(t, IsDisplayOn())
		assert.Equal(t, []bool{true, false}, awayEvents)
		assert.Equal(t, [][]string{{"off"}, {"off"}, {"off"}}, cmds)
	})

	t.Run(
		"ReassertOff throttles rapid calls and skips when home",
		func(t *testing.T) {
			ResetState()
			SetConfig(cfg)

			var cmds [][]string
			setDPMS = func(cmdArgs []string) {
				cmds = append(cmds, cmdArgs)
			}

			current := time.Date(2026, 5, 1, 12, 0, 0, 0, time.Local)
			timeNow = func() time.Time { return current }

			// No-op when not away.
			ReassertOff()
			assert.Empty(t, cmds)

			SetAwayMode(true)
			assert.Len(t, cmds, 1)
			assert.Equal(t, []string{"off"}, cmds[0])

			// Within cooldown: skipped.
			current = current.Add(500 * time.Millisecond)
			ReassertOff()
			assert.Len(t, cmds, 1)

			// Past cooldown: runs OffCommand.
			current = current.Add(touchReassertCooldown)
			ReassertOff()
			assert.Len(t, cmds, 2)
			assert.Equal(t, []string{"off"}, cmds[1])
		})

	t.Run("SetAwayMode without activeCfg does not panic", func(t *testing.T) {
		ResetState()
		setDPMS = func(cmdArgs []string) {
			t.Fatalf("did not expect setDPMS to be called with nil config")
		}
		SetAwayMode(true)
		assert.True(t, IsAway())
		assert.False(t, IsDisplayOn())

		SetAwayMode(false)
		assert.False(t, IsAway())
		assert.True(t, IsDisplayOn())
	})
}
