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

	"github.com/liquidgecka/homehub/config"
	"github.com/stretchr/testify/assert"
)

func TestCheckAndSetDPMS(t *testing.T) {
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

	// The function should be called immediately, and then at least once by the ticker.
	if atomic.LoadInt32(&callCount) < 2 {
		t.Errorf("Expected checkAndSetDPMS to be called at least 2 times, but it was called %d times", callCount)
	}
}

func TestSetDPMS(t *testing.T) {
	// Keep the original setDPMS function
	originalSetDPMS := setDPMS
	defer func() { setDPMS = originalSetDPMS }()

	t.Run("No Command", func(t *testing.T) {
		// This test is simple: it just ensures that calling with no command doesn't panic.
		// We can't easily capture the log output without a more complex setup.
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
