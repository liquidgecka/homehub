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
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	gcalendar "google.golang.org/api/calendar/v3"

	"github.com/liquidgecka/homehub/calendar"
	"github.com/liquidgecka/homehub/config"
	"github.com/liquidgecka/homehub/database"
	"github.com/liquidgecka/homehub/dpms"
	"github.com/liquidgecka/homehub/task"
	"github.com/liquidgecka/homehub/weather"
)

func TestManager(t *testing.T) {
	m := NewManager()

	var wg sync.WaitGroup
	wg.Add(1)

	m.scheduler.AddTask(&task.Task{
		Name:         "test task",
		InitialDelay: 0,
		Interval:     10 * time.Millisecond,
		Task: func(ctx context.Context) {
			wg.Done()
		},
	})

	m.Start()
	wg.Wait()
	m.Stop()
}

func TestManager_Init(t *testing.T) {
	m := NewManager()
	m.Init()
	// Verify that tasks were registered in scheduler
	if m.scheduler == nil {
		t.Fatal("Expected non-nil scheduler after Init")
	}
}

func TestDatabaseBackupTask(t *testing.T) {
	tempHome, err := os.MkdirTemp("", "homehub_bg_test_*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempHome)

	cfg := &config.Config{
		Database: config.DatabaseConfig{
			BackupDirectory:     filepath.Join(tempHome, "backups"),
			BackupIntervalHours: 24,
			BackupRetentionDays: 30,
		},
	}

	taskFn := databaseBackupTask(cfg)
	// Execute task function
	taskFn(context.Background())
}

func TestPhotoCleanupTask(t *testing.T) {
	tempDir := t.TempDir()
	cfg := &config.Config{
		LocalPhotos: config.LocalPhotosConfig{
			Directory: tempDir,
		},
	}

	taskFn := photoCleanupTask(cfg)
	taskFn(context.Background())
}

func TestLoadCalendarEventsTask(t *testing.T) {
	origGetMonth := calendar.GetEventsForMonth
	defer func() { calendar.GetEventsForMonth = origGetMonth }()

	calendar.GetEventsForMonth = func(
		srv *gcalendar.Service,
		c config.GoogleCalendarConfig,
		m time.Time,
	) ([]*gcalendar.Event, error) {
		return []*gcalendar.Event{
			{Summary: "Event 1"},
		}, nil
	}

	cfg := &config.Config{}
	taskFn := loadCalendarEventsTask(cfg)
	taskFn(context.Background())
}

func TestLoadWeatherDataTask(t *testing.T) {
	cfg := &config.Config{}
	taskFn := loadWeatherDataTask(cfg)
	taskFn(context.Background())
}

func TestShoppingStoreCleanupTask(t *testing.T) {
	_, cleanupDB, err := database.NewTestDB()
	if err != nil {
		t.Fatalf("NewTestDB failed: %v", err)
	}
	defer cleanupDB()

	taskFn := shoppingStoreCleanupTask()
	taskFn(context.Background())
}

func TestDisplayTasks_SkipWhenDPMSOff(t *testing.T) {
	dpms.ResetState()
	defer dpms.ResetState()

	origGetMonth := calendar.GetEventsForMonth
	origGetWeather := weather.GetWeather
	defer func() {
		calendar.GetEventsForMonth = origGetMonth
		weather.GetWeather = origGetWeather
	}()

	var calCalls, weatherCalls int32
	calendar.GetEventsForMonth = func(
		srv *gcalendar.Service,
		c config.GoogleCalendarConfig,
		m time.Time,
	) ([]*gcalendar.Event, error) {
		atomic.AddInt32(&calCalls, 1)
		return nil, nil
	}
	weather.GetWeather = func(cfg *config.Config) (*weather.OpenWeather, error) {
		atomic.AddInt32(&weatherCalls, 1)
		return &weather.OpenWeather{}, nil
	}

	cfg := &config.Config{}
	calTask := loadCalendarEventsTask(cfg)
	weatherTask := loadWeatherDataTask(cfg)

	dpms.SetDisplayOn(false)
	calTask(context.Background())
	weatherTask(context.Background())

	if atomic.LoadInt32(&calCalls) != 0 {
		t.Errorf("expected 0 calendar calls when DPMS off, got %d", calCalls)
	}
	if atomic.LoadInt32(&weatherCalls) != 0 {
		t.Errorf("expected 0 weather calls when DPMS off, got %d", weatherCalls)
	}

	dpms.SetDisplayOn(true)
	calTask(context.Background())
	weatherTask(context.Background())

	if atomic.LoadInt32(&calCalls) != 1 {
		t.Errorf("expected 1 calendar call when DPMS on, got %d", calCalls)
	}
	if atomic.LoadInt32(&weatherCalls) != 1 {
		t.Errorf("expected 1 weather call when DPMS on, got %d", weatherCalls)
	}
}

func TestManager_RefreshDisplayData(t *testing.T) {
	dpms.ResetState()
	defer dpms.ResetState()

	origGetMonth := calendar.GetEventsForMonth
	origGetWeather := weather.GetWeather
	defer func() {
		calendar.GetEventsForMonth = origGetMonth
		weather.GetWeather = origGetWeather
	}()

	calCh := make(chan struct{}, 5)
	weatherCh := make(chan struct{}, 5)
	var calCalls, weatherCalls int32

	calendar.GetEventsForMonth = func(
		srv *gcalendar.Service,
		c config.GoogleCalendarConfig,
		m time.Time,
	) ([]*gcalendar.Event, error) {
		atomic.AddInt32(&calCalls, 1)
		select {
		case calCh <- struct{}{}:
		default:
		}
		return nil, nil
	}
	weather.GetWeather = func(cfg *config.Config) (*weather.OpenWeather, error) {
		atomic.AddInt32(&weatherCalls, 1)
		select {
		case weatherCh <- struct{}{}:
		default:
		}
		return &weather.OpenWeather{}, nil
	}

	m := NewManager()
	m.Init()

	dpms.SetDisplayOn(false)
	m.RefreshDisplayData(context.Background())
	time.Sleep(50 * time.Millisecond)
	if atomic.LoadInt32(&calCalls) != 0 || atomic.LoadInt32(&weatherCalls) != 0 {
		t.Fatalf(
			"expected no refresh when DPMS off, got cal=%d weather=%d",
			calCalls, weatherCalls,
		)
	}

	dpms.SetDisplayOn(true)
	m.RefreshDisplayData(context.Background())

	select {
	case <-calCh:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for calendar refresh on wake")
	}
	select {
	case <-weatherCh:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for weather refresh on wake")
	}
}
