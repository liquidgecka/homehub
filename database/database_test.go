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
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// setupTestDB is a helper function to create a new in-memory database for testing.
func setupTestDB(t *testing.T) (*sql.DB, func()) {
	t.Helper()
	db, cleanup, err := NewTestDB()
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	return db, cleanup
}

func TestInitDB(t *testing.T) {
	t.Run("Nil DB", func(t *testing.T) {
		// Temporarily set global db to nil
		originalDB := GetDB()
		SetDB(nil)
		defer SetDB(originalDB)

		err := InitDB()
		if err == nil {
			t.Fatal("Expected error when InitDB is called with a nil database, but got nil")
		}
	})

	t.Run("Success", func(t *testing.T) {
		db, err := sql.Open("sqlite3", ":memory:")
		if err != nil {
			t.Fatalf("Failed to open in-memory db: %v", err)
		}
		defer db.Close()
		SetDB(db)

		if err := InitDB(); err != nil {
			t.Fatalf("InitDB failed: %v", err)
		}

		// Check if a table was created
		_, err = db.Exec(`SELECT * FROM shopping_items;`)
		if err != nil {
			t.Errorf("shopping_items table was not created: %v", err)
		}
	})
}

func TestNewTestDB(t *testing.T) {
	db, cleanup, err := NewTestDB()
	if err != nil {
		t.Fatalf("NewTestDB failed: %v", err)
	}
	defer cleanup()

	if db == nil {
		t.Fatal("Expected a non-nil database connection")
	}

	// Check if the global db is set
	if GetDB() != db {
		t.Fatal("NewTestDB did not set the global db instance")
	}
}

func TestOpenFileDB(t *testing.T) {
	// Mock osUserHomeDir to return a temporary directory
	tempDir := t.TempDir()
	originalHomeDirFunc := osUserHomeDir
	osUserHomeDir = func() (string, error) { return tempDir, nil }
	defer func() { osUserHomeDir = originalHomeDirFunc }()

	// Ensure db is nil initially
	SetDB(nil)

	if err := OpenFileDB(); err != nil {
		t.Fatalf("OpenFileDB() failed on first call: %v", err)
	}
	if GetDB() == nil {
		t.Fatal("db should not be nil after OpenFileDB()")
	}
	// Ping the database to force the file to be created on disk.
	if err := GetDB().Ping(); err != nil {
		t.Fatalf("Failed to ping database: %v", err)
	}
	dbPath := filepath.Join(tempDir, ".local", "homehub", "homehub.db")
	if _, err := os.Stat(dbPath); err != nil {
		t.Fatalf("Database file was not created at %s: %v", dbPath, err)
	}

	// Test that calling it again doesn't cause issues
	if err := OpenFileDB(); err != nil {
		t.Fatalf("OpenFileDB() failed on second call: %v", err)
	}

	CloseDB() // Close the connection for the next test
}

func TestCloseDB(t *testing.T) {
	// This test is a bit tricky because it manipulates global state.
	// Ensure it runs in isolation if needed.
	db, cleanup, err := NewTestDB()
	if err != nil {
		t.Fatalf("Failed to create test DB: %v", err)
	}
	defer cleanup() // This will run after the test, ensuring the db is reset.

	// The db that NewTestDB returns is the same as the global one.
	CloseDB()

	// Now, pinging the original db connection should fail.
	if err := db.Ping(); err == nil {
		t.Error("Expected ping to fail on a closed DB, but it succeeded.")
	}

	// Set the global db to nil so subsequent calls to OpenFileDB in other tests will work
	SetDB(nil)
}

func TestStorageFunctions(t *testing.T) {
	_, cleanup := setupTestDB(t)
	defer cleanup()

	t.Run("Set and Get", func(t *testing.T) {
		key, value := "test_key", "test_value"
		if err := SetStorageValue(key, value); err != nil {
			t.Fatalf("SetStorageValue failed: %v", err)
		}

		retrieved, err := GetStorageValue(key)
		if err != nil {
			t.Fatalf("GetStorageValue failed: %v", err)
		}
		if retrieved != value {
			t.Errorf("Got '%s', want '%s'", retrieved, value)
		}
	})

	t.Run("Get Non-existent", func(t *testing.T) {
		_, err := GetStorageValue("non_existent_key")
		if err != sql.ErrNoRows {
			t.Errorf("Expected sql.ErrNoRows for non-existent key, got %v", err)
		}
	})

	t.Run("Delete", func(t *testing.T) {
		key, value := "delete_key", "delete_value"
		SetStorageValue(key, value) // Don't care about error here

		if err := DeleteStorageValue(key); err != nil {
			t.Fatalf("DeleteStorageValue failed: %v", err)
		}

		_, err := GetStorageValue(key)
		if err != sql.ErrNoRows {
			t.Errorf("Expected sql.ErrNoRows after deleting key, got %v", err)
		}
	})

	t.Run("List with Prefix", func(t *testing.T) {
		SetStorageValue("prefix_1", "val1")
		SetStorageValue("prefix_2", "val2")
		SetStorageValue("other_1", "val3")

		keys, err := ListStorageKeysWithPrefix("prefix_")
		if err != nil {
			t.Fatalf("ListStorageKeysWithPrefix failed: %v", err)
		}
		if len(keys) != 2 {
			t.Errorf("Expected 2 keys with prefix, got %d", len(keys))
		}
	})
}

func TestTokenFunctions(t *testing.T) {
	_, cleanup := setupTestDB(t)
	defer cleanup()

	serviceName := "test_service"
	token := "test_refresh_token"

	if err := StoreRefreshToken(serviceName, token); err != nil {
		t.Fatalf("StoreRefreshToken failed: %v", err)
	}

	retrieved, err := GetRefreshToken(serviceName)
	if err != nil {
		t.Fatalf("GetRefreshToken failed: %v", err)
	}
	if retrieved != token {
		t.Errorf("Got token '%s', want '%s'", retrieved, token)
	}

	if err := DeleteRefreshToken(serviceName); err != nil {
		t.Fatalf("DeleteRefreshToken failed: %v", err)
	}

	_, err = GetRefreshToken(serviceName)
	if err != sql.ErrNoRows {
		t.Errorf("Expected sql.ErrNoRows after deleting token, got %v", err)
	}
}

func TestShoppingFunctions(t *testing.T) {
	_, cleanup := setupTestDB(t)
	defer cleanup()

	// 1. Add items
	item1 := ShoppingItem{Name: "Milk", Quantity: 1, StoreID: 1}
	item2 := ShoppingItem{Name: "Bread", Quantity: 2, StoreID: 1}
	item3 := ShoppingItem{Name: "Apples", Quantity: 5, StoreID: 2}

	id1, err := AddShoppingItem(item1)
	if err != nil {
		t.Fatalf("AddShoppingItem failed for item1: %v", err)
	}
	item1.ID = id1

	id2, err := AddShoppingItem(item2)
	if err != nil {
		t.Fatalf("AddShoppingItem failed for item2: %v", err)
	}
	item2.ID = id2

	id3, err := AddShoppingItem(item3)
	if err != nil {
		t.Fatalf("AddShoppingItem failed for item3: %v", err)
	}
	item3.ID = id3

	// 2. Get items by store
	store1Items, err := GetShoppingItemsByStore(1)
	if err != nil {
		t.Fatalf("GetShoppingItemsByStore(1) failed: %v", err)
	}
	if len(store1Items) != 2 {
		t.Fatalf("Expected 2 items for store 1, got %d", len(store1Items))
	}

	// 3. Update an item
	item2.Checked = true
	if err := UpdateShoppingItem(item2); err != nil {
		t.Fatalf("UpdateShoppingItem failed: %v", err)
	}

	updatedItems, _ := GetShoppingItemsByStore(1)
	found := false
	for _, item := range updatedItems {
		if item.ID == id2 {
			found = true
			if !item.Checked {
				t.Error("Expected item to be checked, but it wasn't")
			}
		}
	}
	if !found {
		t.Error("Could not find updated item")
	}

	// 4. Delete an item
	if err := DeleteShoppingItem(id1); err != nil {
		t.Fatalf("DeleteShoppingItem failed: %v", err)
	}
	remainingItems, _ := GetShoppingItemsByStore(1)
	if len(remainingItems) != 1 {
		t.Errorf("Expected 1 item to remain for store 1, got %d", len(remainingItems))
	}

	// 5. Test store metadata
	now := time.Now().Truncate(time.Second)
	if err := AddOrUpdateShoppingStoreMetadata(1, now); err != nil {
		t.Fatalf("AddOrUpdateShoppingStoreMetadata failed: %v", err)
	}

	expired, err := GetExpiredShoppingStoreIDs(now.Add(time.Second))
	if err != nil {
		t.Fatalf("GetExpiredShoppingStoreIDs failed: %v", err)
	}
	if len(expired) != 1 || expired[0] != 1 {
		t.Errorf("Expected to find 1 expired store, got %v", expired)
	}

	if err := DeleteShoppingStoreMetadata(1); err != nil {
		t.Fatalf("DeleteShoppingStoreMetadata failed: %v", err)
	}
	expired, _ = GetExpiredShoppingStoreIDs(now.Add(time.Second))
	if len(expired) != 0 {
		t.Errorf("Expected 0 expired stores after deletion, got %d", len(expired))
	}

	// 6. Test DeleteShoppingItemsByStore
	if err := DeleteShoppingItemsByStore(1); err != nil {
		t.Fatalf("DeleteShoppingItemsByStore failed: %v", err)
	}
	finalItems, _ := GetShoppingItemsByStore(1)
	if len(finalItems) != 0 {
		t.Errorf("Expected 0 items for store 1 after deletion, got %d", len(finalItems))
	}
}

func TestLedgerFunctions(t *testing.T) {
	_, cleanup := setupTestDB(t)
	defer cleanup()

	// 1. Add an account
	accName := "Test Account"
	accInitialBalance := 100.0
	accID, err := AddAccountDB(accName, accInitialBalance)
	if err != nil {
		t.Fatalf("AddAccountDB failed: %v", err)
	}

	// 2. Get accounts and verify
	accounts, err := GetAccountsDB()
	if err != nil {
		t.Fatalf("GetAccountsDB failed: %v", err)
	}
	if len(accounts) != 1 {
		t.Fatalf("Expected 1 account, got %d", len(accounts))
	}
	if accounts[0].Name != accName || accounts[0].ID != accID {
		t.Errorf("Account mismatch: got %+v", accounts[0])
	}

	// 3. Add ledger records
	rec1 := LedgerRecord{AccountID: accID, Description: "Paycheck", Amount: 500, Type: Credit, Balance: 600, Timestamp: time.Now()}
	rec2 := LedgerRecord{AccountID: accID, Description: "Groceries", Amount: 50, Type: Debit, Balance: 550, Timestamp: time.Now()}
	rec1ID, err := AddLedgerRecordDB(rec1)
	if err != nil {
		t.Fatalf("AddLedgerRecordDB failed for rec1: %v", err)
	}
	rec2ID, err := AddLedgerRecordDB(rec2)
	if err != nil {
		t.Fatalf("AddLedgerRecordDB failed for rec2: %v", err)
	}

	// 4. Get ledger records and verify
	records, err := GetLedgerRecordsDB(accID)
	if err != nil {
		t.Fatalf("GetLedgerRecordsDB failed: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("Expected 2 ledger records, got %d", len(records))
	}

	// 5. Update a record
	rec2.Description = "Snacks"
	rec2.ID = rec2ID
	if err := UpdateLedgerRecordDB(rec2); err != nil {
		t.Fatalf("UpdateLedgerRecordDB failed: %v", err)
	}
	records, _ = GetLedgerRecordsDB(accID)
	found := false
	for _, rec := range records {
		if rec.ID == rec2ID {
			found = true
			if rec.Description != "Snacks" {
				t.Error("Expected record description to be updated")
			}
		}
	}
	if !found {
		t.Error("Could not find updated record")
	}

	// 6. Delete a record
	if err := DeleteLedgerRecordDB(rec1ID); err != nil {
		t.Fatalf("DeleteLedgerRecordDB failed: %v", err)
	}
	records, _ = GetLedgerRecordsDB(accID)
	if len(records) != 1 {
		t.Errorf("Expected 1 record to remain, got %d", len(records))
	}

	// 7. Update and Delete Account
	acc := accounts[0]
	acc.Name = "Updated Name"
	if err := UpdateAccountDB(acc); err != nil {
		t.Fatalf("UpdateAccountDB failed: %v", err)
	}
	updatedAccounts, _ := GetAccountsDB()
	if updatedAccounts[0].Name != "Updated Name" {
		t.Error("Account name was not updated")
	}

	if err := DeleteAccountDB(accID); err != nil {
		t.Fatalf("DeleteAccountDB failed: %v", err)
	}
	finalAccounts, _ := GetAccountsDB()
	if len(finalAccounts) != 0 {
		t.Errorf("Expected 0 accounts after deletion, got %d", len(finalAccounts))
	}
}

func TestReminderFunctions(t *testing.T) {
	_, cleanup := setupTestDB(t)
	defer cleanup()

	// 1. Add reminder
	rem := Reminder{
		Title:        "Feed the dogs",
		Time:         "08:00",
		Days:         "Everyday",
		Enabled:      true,
		Acknowledged: true,
	}
	id, err := AddReminderDB(rem)
	if err != nil {
		t.Fatalf("AddReminderDB failed: %v", err)
	}
	if id <= 0 {
		t.Fatalf("Expected positive ID, got %d", id)
	}

	// 2. Get all reminders
	list, err := GetRemindersDB()
	if err != nil {
		t.Fatalf("GetRemindersDB failed: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("Expected 1 reminder, got %d", len(list))
	}
	if list[0].Title != "Feed the dogs" {
		t.Errorf("Title mismatch: got %s, want Feed the dogs", list[0].Title)
	}

	// 3. Get reminder by ID
	fetched, err := GetReminderByIDDB(id)
	if err != nil {
		t.Fatalf("GetReminderByIDDB failed: %v", err)
	}
	if fetched.Time != "08:00" {
		t.Errorf("Time mismatch: got %s, want 08:00", fetched.Time)
	}

	// 4. Update reminder
	fetched.Title = "Feed dogs & cats"
	fetched.Time = "08:30"
	if err := UpdateReminderDB(fetched); err != nil {
		t.Fatalf("UpdateReminderDB failed: %v", err)
	}

	updated, err := GetReminderByIDDB(id)
	if err != nil {
		t.Fatalf("GetReminderByIDDB failed: %v", err)
	}
	if updated.Title != "Feed dogs & cats" || updated.Time != "08:30" {
		t.Errorf("Updated values mismatch: %+v", updated)
	}

	// 5. Trigger reminder
	now := time.Now().Truncate(time.Second)
	if err := SetReminderTriggeredDB(id, now); err != nil {
		t.Fatalf("SetReminderTriggeredDB failed: %v", err)
	}
	triggered, _ := GetReminderByIDDB(id)
	if triggered.Acknowledged {
		t.Error("Expected Acknowledged to be false after trigger")
	}
	if triggered.LastTriggered.IsZero() {
		t.Error("Expected LastTriggered to be non-zero after trigger")
	}

	// 6. Acknowledge reminder
	ackTime := time.Now().Truncate(time.Second)
	if err := SetReminderAcknowledgedDB(id, true, ackTime); err != nil {
		t.Fatalf("SetReminderAcknowledgedDB failed: %v", err)
	}
	acked, _ := GetReminderByIDDB(id)
	if !acked.Acknowledged {
		t.Error("Expected Acknowledged to be true")
	}

	// 7. Delete reminder
	if err := DeleteReminderDB(id); err != nil {
		t.Fatalf("DeleteReminderDB failed: %v", err)
	}
	finalList, _ := GetRemindersDB()
	if len(finalList) != 0 {
		t.Errorf("Expected 0 reminders after deletion, got %d", len(finalList))
	}
}

func TestStorageValue(t *testing.T) {
	_, cleanup, err := NewTestDB()
	if err != nil {
		t.Fatalf("NewTestDB failed: %v", err)
	}
	defer cleanup()

	if err := SetStorageValue("test_key", "test_val"); err != nil {
		t.Fatalf("SetStorageValue failed: %v", err)
	}

	val, err := GetStorageValue("test_key")
	if err != nil {
		t.Fatalf("GetStorageValue failed: %v", err)
	}
	if val != "test_val" {
		t.Errorf("GetStorageValue = %q, want test_val", val)
	}
}

func TestOpenFileDBAndCloseDB(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	if err := OpenFileDB(); err != nil {
		t.Fatalf("OpenFileDB failed: %v", err)
	}
	// Calling OpenFileDB when already open should return nil
	if err := OpenFileDB(); err != nil {
		t.Errorf("Second OpenFileDB failed: %v", err)
	}
	CloseDB()
}

func TestNilDBErrors(t *testing.T) {
	SetDB(nil)

	if err := SetStorageValue("a", "b"); err == nil {
		t.Error("Expected error when db is nil")
	}
	if _, err := GetStorageValue("a"); err == nil {
		t.Error("Expected error when db is nil")
	}
	if _, err := GetRemindersDB(); err == nil {
		t.Error("Expected error when db is nil")
	}
}
