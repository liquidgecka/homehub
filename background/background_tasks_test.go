// Copyright 2026 - Brady Catherman
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
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
	"testing"
	"time"

	"github.com/liquidgecka/homehub/config"
	"github.com/liquidgecka/homehub/task"
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
