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

package database

import (
	"archive/zip"
	"os"
	"path/filepath"
	"testing"

	"github.com/liquidgecka/homehub/config"
)

func setupTestHome(t *testing.T) (string, func()) {
	tempHome, err := os.MkdirTemp("", "homehub_test_home_*")
	if err != nil {
		t.Fatalf("Failed to create temp home dir: %v", err)
	}

	origHomeDir := osUserHomeDir
	osUserHomeDir = func() (string, error) {
		return tempHome, nil
	}

	cleanup := func() {
		CloseDB()
		osUserHomeDir = origHomeDir
		os.RemoveAll(tempHome)
	}

	return tempHome, cleanup
}

func TestResolveBackupDirectory(t *testing.T) {
	tempHome, cleanup := setupTestHome(t)
	defer cleanup()

	// Default / empty
	dir, err := ResolveBackupDirectory("")
	if err != nil {
		t.Fatalf("ResolveBackupDirectory failed: %v", err)
	}
	expectedDefault := filepath.Join(tempHome, ".local", "homehub", "backups")
	if dir != expectedDefault {
		t.Errorf("Expected %s, got %s", expectedDefault, dir)
	}

	// Tilde expansion
	dir, err = ResolveBackupDirectory("~/custom/backups")
	if err != nil {
		t.Fatalf("ResolveBackupDirectory failed: %v", err)
	}
	expectedCustom := filepath.Join(tempHome, "custom", "backups")
	if dir != expectedCustom {
		t.Errorf("Expected %s, got %s", expectedCustom, dir)
	}

	// Absolute path
	dir, err = ResolveBackupDirectory("/var/backups/homehub")
	if err != nil {
		t.Fatalf("ResolveBackupDirectory failed: %v", err)
	}
	if dir != "/var/backups/homehub" {
		t.Errorf("Expected /var/backups/homehub, got %s", dir)
	}
}

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		bytes    int64
		expected string
	}{
		{500, "500 B"},
		{1024, "1.0 KB"},
		{1536, "1.5 KB"},
		{1048576, "1.0 MB"},
		{104857600, "100.0 MB"},
	}

	for _, tc := range tests {
		got := FormatBytes(tc.bytes)
		if got != tc.expected {
			t.Errorf(
				"FormatBytes(%d) = %s, expected %s",
				tc.bytes, got, tc.expected,
			)
		}
	}
}

func TestCreateAndListBackups(t *testing.T) {
	tempHome, cleanup := setupTestHome(t)
	defer cleanup()

	// Initialize DB with sample data
	if err := OpenFileDB(); err != nil {
		t.Fatalf("OpenFileDB failed: %v", err)
	}
	if err := InitDB(); err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}

	item := ShoppingItem{
		Name:     "Apples",
		Quantity: 5,
		StoreID:  1,
	}
	if _, err := AddShoppingItem(item); err != nil {
		t.Fatalf("AddShoppingItem failed: %v", err)
	}

	backupDir := filepath.Join(tempHome, "backups")

	// Create backup
	info, err := CreateBackup(backupDir)
	if err != nil {
		t.Fatalf("CreateBackup failed: %v", err)
	}
	if info == nil {
		t.Fatal("Expected non-nil BackupInfo")
	}

	if _, err := os.Stat(info.Path); err != nil {
		t.Fatalf("Backup file does not exist on disk: %v", err)
	}

	// List backups
	backups, err := ListBackups(backupDir)
	if err != nil {
		t.Fatalf("ListBackups failed: %v", err)
	}
	if len(backups) != 1 {
		t.Fatalf("Expected 1 backup, got %d", len(backups))
	}
	if backups[0].Filename != info.Filename {
		t.Errorf(
			"Expected filename %s, got %s",
			info.Filename, backups[0].Filename,
		)
	}
}

func TestPruneOldBackups(t *testing.T) {
	tempHome, cleanup := setupTestHome(t)
	defer cleanup()

	backupDir := filepath.Join(tempHome, "backups")
	if err := os.MkdirAll(backupDir, 0700); err != nil {
		t.Fatal(err)
	}

	// Create a recent backup zip
	recentZip := filepath.Join(backupDir, "homehub_backup_20260828_120000.zip")
	createDummyZip(t, recentZip)

	// Create an old backup zip (60 days ago)
	oldZip := filepath.Join(backupDir, "homehub_backup_20260101_120000.zip")
	createDummyZip(t, oldZip)

	backups, err := ListBackups(backupDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(backups) != 2 {
		t.Fatalf("Expected 2 backups before pruning, got %d", len(backups))
	}

	pruned, err := PruneOldBackups(backupDir, 30)
	if err != nil {
		t.Fatalf("PruneOldBackups failed: %v", err)
	}
	if pruned != 1 {
		t.Errorf("Expected 1 backup pruned, got %d", pruned)
	}

	remaining, err := ListBackups(backupDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(remaining) != 1 {
		t.Fatalf("Expected 1 backup remaining, got %d", len(remaining))
	}
	if remaining[0].Filename != filepath.Base(recentZip) {
		t.Errorf(
			"Expected recent backup retained, got %s",
			remaining[0].Filename,
		)
	}
}

func TestRestoreBackup(t *testing.T) {
	tempHome, cleanup := setupTestHome(t)
	defer cleanup()

	// Initialize DB with sample data
	if err := OpenFileDB(); err != nil {
		t.Fatalf("OpenFileDB failed: %v", err)
	}
	if err := InitDB(); err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}

	item := ShoppingItem{
		Name:     "Oranges",
		Quantity: 3,
		StoreID:  1,
	}
	itemID, err := AddShoppingItem(item)
	if err != nil {
		t.Fatalf("AddShoppingItem failed: %v", err)
	}

	backupDir := filepath.Join(tempHome, "backups")
	info, err := CreateBackup(backupDir)
	if err != nil {
		t.Fatalf("CreateBackup failed: %v", err)
	}

	// Now modify the DB state (delete shopping item)
	if err := DeleteShoppingItem(itemID); err != nil {
		t.Fatalf("DeleteShoppingItem failed: %v", err)
	}
	items, err := GetShoppingItemsByStore(1)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 0 {
		t.Fatalf("Expected 0 items after delete, got %d", len(items))
	}

	// Restore from backup
	if err := RestoreBackup(info.Path); err != nil {
		t.Fatalf("RestoreBackup failed: %v", err)
	}

	// Verify item is restored
	itemsRestored, err := GetShoppingItemsByStore(1)
	if err != nil {
		t.Fatalf("GetShoppingItemsByStore failed after restore: %v", err)
	}
	if len(itemsRestored) != 1 {
		t.Fatalf("Expected 1 restored item, got %d", len(itemsRestored))
	}
	if itemsRestored[0].Name != "Oranges" {
		t.Errorf(
			"Expected restored item 'Oranges', got %s",
			itemsRestored[0].Name,
		)
	}
}

func TestRestoreBackup_CorruptExistingDB(t *testing.T) {
	tempHome, cleanup := setupTestHome(t)
	defer cleanup()

	// Initialize DB with sample data
	if err := OpenFileDB(); err != nil {
		t.Fatalf("OpenFileDB failed: %v", err)
	}
	if err := InitDB(); err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}

	reminder := Reminder{
		Title:   "Take vitamins",
		Time:    "08:00",
		Days:    "Everyday",
		Enabled: true,
	}
	if _, err := AddReminderDB(reminder); err != nil {
		t.Fatalf("AddReminderDB failed: %v", err)
	}

	backupDir := filepath.Join(tempHome, "backups")
	info, err := CreateBackup(backupDir)
	if err != nil {
		t.Fatalf("CreateBackup failed: %v", err)
	}

	// Corrupt existing DB by overwriting with garbage bytes
	CloseDB()
	dbPath, _ := GetDBPath()
	corruptData := []byte("NOT A VALID SQLITE FILE")
	if err := os.WriteFile(dbPath, corruptData, 0644); err != nil {
		t.Fatal(err)
	}

	// Restore backup over corrupted database
	if err := RestoreBackup(info.Path); err != nil {
		t.Fatalf("RestoreBackup over corrupted DB failed: %v", err)
	}

	// Verify DB is functioning and reminder is intact
	rems, err := GetRemindersDB()
	if err != nil {
		t.Fatalf("GetRemindersDB failed after restore: %v", err)
	}
	if len(rems) != 1 || rems[0].Title != "Take vitamins" {
		t.Errorf("Unexpected reminders after restore: %+v", rems)
	}
}

func TestScheduledBackupTask(t *testing.T) {
	tempHome, cleanup := setupTestHome(t)
	defer cleanup()

	if err := OpenFileDB(); err != nil {
		t.Fatalf("OpenFileDB failed: %v", err)
	}
	if err := InitDB(); err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}

	cfg := &config.Config{
		Database: config.DatabaseConfig{
			BackupDirectory:     filepath.Join(tempHome, "backups"),
			BackupIntervalHours: 24,
			BackupRetentionDays: 30,
		},
	}

	if err := ScheduledBackupTask(cfg); err != nil {
		t.Fatalf("ScheduledBackupTask failed: %v", err)
	}

	backups, err := ListBackups(cfg.Database.BackupDirectory)
	if err != nil {
		t.Fatal(err)
	}
	if len(backups) != 1 {
		t.Fatalf("Expected 1 backup created, got %d", len(backups))
	}
}

func createDummyZip(t *testing.T, path string) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	zw := zip.NewWriter(f)
	w, err := zw.Create("homehub.db")
	if err != nil {
		t.Fatal(err)
	}
	// Write minimal SQLite header so it passes validation
	sqliteHeader := []byte("SQLite format 3\x00extra data")
	if _, err := w.Write(sqliteHeader); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
}
