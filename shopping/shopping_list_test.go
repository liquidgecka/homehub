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

package shopping

import (
	"database/sql"
	"fmt"

	// "os" // Not used in this test file directly anymore
	"sync"
	"testing"

	"github.com/liquidgecka/homehub/database"
	"github.com/liquidgecka/homehub/testutils"
)

// Mock database functions
var (
	mockAddShoppingItem    func(item database.ShoppingItem) (int, error)
	mockGetShoppingItems   func() ([]database.ShoppingItem, error)
	mockUpdateShoppingItem func(item database.ShoppingItem) error
	mockDeleteShoppingItem func(id int) error
)

// Override actual DB functions with mocks for testing
// (These are now handled by assigning to the global vars in database.go)

func TestShoppingItem(t *testing.T) {
	testutils.SetupLogCapture(t)
	item := database.ShoppingItem{Name: "Milk", Quantity: 1, Checked: false, StoreID: 0}

	if item.Name != "Milk" {
		t.Errorf("Expected item name 'Milk', got '%s'", item.Name)
	}
	if item.Quantity != 1 {
		t.Errorf("Expected item quantity 1, got %d", item.Quantity)
	}
	if item.Checked != false {
		t.Errorf("Expected item checked status to be false, got %t", item.Checked)
	}

	item.Checked = true
	if item.Checked != true {
		t.Errorf("Expected item checked status to be true after change, got %t", item.Checked)
	}
}

func TestAddItem(t *testing.T) {
	testutils.SetupLogCapture(t)
	var mu sync.Mutex // Protect mock calls
	var addedItems []database.ShoppingItem

	// Store original DB functions
	originalAddShoppingItem := database.AddShoppingItem
	originalGetShoppingItems := database.GetShoppingItems

	// Setup mock
	database.AddShoppingItem = func(item database.ShoppingItem) (int, error) {
		mu.Lock()
		defer mu.Unlock()
		item.ID = len(addedItems) + 1 // Assign a mock ID
		addedItems = append(addedItems, item)
		return item.ID, nil
	}
	database.GetShoppingItems = func() ([]database.ShoppingItem, error) {
		mu.Lock()
		defer mu.Unlock()
		return addedItems, nil
	}

	defer func() { // Restore original functions after test
		database.AddShoppingItem = originalAddShoppingItem
		database.GetShoppingItems = originalGetShoppingItems
	}()

	item := database.ShoppingItem{Name: "Bread", Quantity: 2, StoreID: 1}
	err := AddItem(item)
	if err != nil {
		t.Fatalf("AddItem failed: %v", err)
	}

	items, err := GetItems()
	if err != nil {
		t.Fatalf("GetItems failed: %v", err)
	}
	if len(items) != 1 || items[0].Name != "Bread" || items[0].Quantity != 2 {
		t.Errorf("Expected 1 item 'Bread' (qty 2), got: %+v", items)
	}

	item2 := database.ShoppingItem{Name: "Milk", Quantity: 1, StoreID: 0}
	err = AddItem(item2)
	if err != nil {
		t.Fatalf("AddItem failed: %v", err)
	}

	items, err = GetItems()
	if err != nil {
		t.Fatalf("GetItems failed: %v", err)
	}
	if len(items) != 2 {
		t.Errorf("Expected 2 items, got %d", len(items))
	}
}

func TestGetItems(t *testing.T) {
	testutils.SetupLogCapture(t)
	// Store original DB functions
	originalGetShoppingItems := database.GetShoppingItems

	// Setup mock to return predefined items
	database.GetShoppingItems = func() ([]database.ShoppingItem, error) {
		return []database.ShoppingItem{
			{ID: 1, Name: "Apples", Quantity: 3},
			{ID: 2, Name: "Oranges", Quantity: 2},
		}, nil
	}

	defer func() { database.GetShoppingItems = originalGetShoppingItems }()

	items, err := GetItems()
	if err != nil {
		t.Fatalf("GetItems failed: %v", err)
	}
	if len(items) != 2 || items[0].Name != "Apples" || items[1].Name != "Oranges" {
		t.Errorf("Expected 2 specific items, got: %+v", items)
	}

	// Test case for no items
	database.GetShoppingItems = func() ([]database.ShoppingItem, error) { return []database.ShoppingItem{}, nil }
	items, err = GetItems()
	if err != nil {
		t.Fatalf("GetItems (empty) failed: %v", err)
	}
	if len(items) != 0 {
		t.Errorf("Expected 0 items, got %d", len(items))
	}
}

func TestUpdateItem(t *testing.T) {
	testutils.SetupLogCapture(t)
	// Store original DB functions
	originalUpdateShoppingItem := database.UpdateShoppingItem
	originalGetShoppingItems := database.GetShoppingItems

	// Assume an item exists
	var storedItem database.ShoppingItem = database.ShoppingItem{ID: 1, Name: "Old Name", Quantity: 1, Checked: false}
	database.UpdateShoppingItem = func(item database.ShoppingItem) error {
		if item.ID != storedItem.ID {
			return fmt.Errorf("item ID mismatch")
		}
		storedItem = item // "Update" the stored item
		return nil
	}
	database.GetShoppingItems = func() ([]database.ShoppingItem, error) { return []database.ShoppingItem{storedItem}, nil }

	defer func() {
		database.UpdateShoppingItem = originalUpdateShoppingItem
		database.GetShoppingItems = originalGetShoppingItems
	}()

	updated := database.ShoppingItem{ID: 1, Name: "New Name", Quantity: 5, Checked: true}
	err := UpdateItem(updated)
	if err != nil {
		t.Fatalf("UpdateItem failed: %v", err)
	}

	// Verify update
	if storedItem.Name != "New Name" || storedItem.Quantity != 5 || storedItem.Checked != true {
		t.Errorf("Item not updated correctly: got %+v", storedItem)
	}

	// Test non-existent item (mock can return error or do nothing)
	database.UpdateShoppingItem = func(item database.ShoppingItem) error { return sql.ErrNoRows } // Simulate no rows affected
	err = UpdateItem(database.ShoppingItem{ID: 99, Name: "NonExistent"})
	if err == nil {
		t.Error("Expected error for updating non-existent item, got nil")
	}
}

func TestDeleteItem(t *testing.T) {
	testutils.SetupLogCapture(t)
	// Store original DB functions
	originalDeleteShoppingItem := database.DeleteShoppingItem
	originalGetShoppingItems := database.GetShoppingItems

	var itemExists bool = true
	database.DeleteShoppingItem = func(id int) error {
		if id == 1 {
			itemExists = false
			return nil
		}
		return fmt.Errorf("item not found")
	}
	database.GetShoppingItems = func() ([]database.ShoppingItem, error) {
		if itemExists {
			return []database.ShoppingItem{{ID: 1}}, nil
		}
		return []database.ShoppingItem{}, nil
	}

	defer func() {
		database.DeleteShoppingItem = originalDeleteShoppingItem
		database.GetShoppingItems = originalGetShoppingItems
	}()

	err := DeleteItem(1)
	if err != nil {
		t.Fatalf("DeleteItem failed: %v", err)
	}

	// Verify deletion
	items, _ := GetItems()
	if len(items) != 0 {
		t.Errorf("Expected item to be deleted, found %d items", len(items))
	}

	// Test deleting non-existent item
	err = DeleteItem(99)
	if err == nil {
		t.Error("Expected error for deleting non-existent item, got nil")
	}
}
