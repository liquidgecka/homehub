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
	"fmt"
	"log"

	"github.com/liquidgecka/homehub/config"
	"github.com/liquidgecka/homehub/database"
)

func triggerSyncIfEnabled() {
	if config.GetConfig().Shopping.GoogleTasks.Enabled {
		go syncAllStores()
	}
}

// AddItem adds a new shopping item to the database.
var AddItem = func(item database.ShoppingItem) error {
	_, err := AddItemWithID(item)
	return err
}

// AddItemWithID adds a new shopping item to the database and returns its ID.
var AddItemWithID = func(item database.ShoppingItem) (int, error) {
	id, err := database.AddShoppingItem(item) // Call database function
	if err != nil {
		return 0, fmt.Errorf(
			"failed to add shopping item to database: %w", err,
		)
	}
	log.Printf("Added item to database with ID: %d, Item: %+v", id, item)
	triggerSyncIfEnabled()
	return id, nil
}

// GetItems returns the current list of shopping items from the database.
func GetItems() ([]database.ShoppingItem, error) {
	items, err := database.GetShoppingItems() // Call database function
	if err != nil {
		return nil, fmt.Errorf("failed to get shopping items from database: %w", err)
	}
	return items, nil
}

// GetShoppingItemByID retrieves a single shopping item by its ID from the
// database.
var GetShoppingItemByID = func(id int) (database.ShoppingItem, error) {
	item, err := database.GetShoppingItemByIDDB(id)
	if err != nil {
		return database.ShoppingItem{}, fmt.Errorf(
			"failed to get shopping item by ID %d from database: %w", id, err,
		)
	}
	return item, nil
}

// UpdateItem updates an existing shopping item in the database.
var UpdateItem = func(item database.ShoppingItem) error {
	err := database.UpdateShoppingItem(item) // Call database function
	if err != nil {
		return fmt.Errorf("failed to update shopping item in database: %w", err)
	}
	log.Printf("Updated item in database: %+v", item)
	triggerSyncIfEnabled()
	return nil
}

// DeleteItem deletes a shopping item from the database.
var DeleteItem = func(id int) error {
	err := database.DeleteShoppingItem(id) // Call database function
	if err != nil {
		return fmt.Errorf("failed to delete shopping item from database: %w", err)
	}
	log.Printf("Deleted item from database with ID: %d", id)
	triggerSyncIfEnabled()
	return nil
}
