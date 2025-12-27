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
	"testing"

	"github.com/liquidgecka/homehub/database"
)

func TestAddShoppingItemDB(t *testing.T) {
	_, cleanup, err := database.NewTestDB()
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer cleanup()

	item := database.ShoppingItem{Name: "Test Item", Quantity: 1, StoreID: 1}
	_, err = database.AddShoppingItem(item)
	if err != nil {
		t.Fatalf("AddItem failed: %v", err)
	}

	var items []database.ShoppingItem
	items, err = database.GetShoppingItemsByStore(1)
	if err != nil {
		t.Fatalf("GetShoppingItemsByStore failed: %v", err)
	}

	if len(items) != 1 {
		t.Fatalf("Expected 1 item, got %d", len(items))
	}

	if items[0].Name != "Test Item" {
		t.Errorf("Expected item name 'Test Item', got '%s'", items[0].Name)
	}
}
