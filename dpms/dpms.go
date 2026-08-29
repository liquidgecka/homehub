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
	"time"

	"github.com/liquidgecka/homehub/config"
)

var (
	timeNow         = time.Now
	checkAndSetDPMS = _checkAndSetDPMS
)

// StartScheduler starts a ticker that checks the time every minute and
// enables or disables DPMS based on the schedule in the config file.
func StartScheduler(
	parentCtx context.Context, cfg *config.DPMSConfig,
) context.CancelFunc {
	log.Println("DPMS: Starting scheduler...")
	ctx, cancel := context.WithCancel(parentCtx)
	interval := time.Duration(cfg.CheckIntervalSeconds) * time.Second
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

func _checkAndSetDPMS(cfg *config.DPMSConfig) {
	now := timeNow()
	shouldBeOn := false

	log.Printf(
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
		log.Printf("DPMS: Evaluating period: %v", period)
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
		log.Printf(
			"DPMS: Comparing with start: %s, end: %s",
			startTime.Format("15:04:05"), endTime.Format("15:04:05"),
		)

		// Check if the period spans overnight
		if startTime.After(endTime) {
			// e.g., Start: 22:00, End: 07:00
			// "On" period is after the start time OR before the end time.
			if now.After(startTime) || now.Before(endTime) {
				shouldBeOn = true
				break // Found a matching period, no need to check others.
			}
		} else {
			// Same-day period, e.g., Start: 07:00, End: 22:00
			if now.After(startTime) && now.Before(endTime) {
				shouldBeOn = true
				break // Found a matching period, no need to check others.
			}
		}
	}

	if shouldBeOn {
		log.Println("DPMS: Decision: ON")
		setDPMS(cfg.OnCommand)
	} else {
		log.Println("DPMS: Decision: OFF")
		setDPMS(cfg.OffCommand)
	}
}

var setDPMS = func(cmdArgs []string) {
	if len(cmdArgs) == 0 {
		log.Println("DPMS: No command configured.")
		return
	}

	cmd := exec.Command(cmdArgs[0], cmdArgs[1:]...)
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	log.Printf("DPMS: Executing command: %s", cmd.String())
	err := cmd.Run()
	outStr, errStr := stdout.String(), stderr.String()

	if err != nil {
		log.Printf("DPMS: Failed to execute command. Error: %v", err)
		log.Printf("DPMS: Command output (stdout): %s", outStr)
		log.Printf("DPMS: Command output (stderr): %s", errStr)
		return
	}

	log.Printf("DPMS: Successfully executed command.")
	if len(outStr) > 0 {
		log.Printf("DPMS: Command output (stdout): %s", outStr)
	}
	if len(errStr) > 0 {
		log.Printf("DPMS: Command output (stderr): %s", errStr)
	}
}
