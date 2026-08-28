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
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"google.golang.org/api/tasks/v1"

	"github.com/liquidgecka/homehub/config"
	"github.com/liquidgecka/homehub/database"
	"github.com/liquidgecka/homehub/google"
)

// StartGoogleTasksSync initializes and runs a goroutine for periodic Google Tasks
// synchronization for shopping lists.
func StartGoogleTasksSync(parentCtx context.Context) context.CancelFunc {
	cfg := config.GetConfig()
	if !cfg.Shopping.GoogleTasks.Enabled {
		log.Println("Google Tasks sync for shopping lists is disabled.")
		return func() {} // Return a no-op cancel function
	}

	ctx, cancel := context.WithCancel(parentCtx)
	refreshMinutes := cfg.Google.Calendar.CalendarRefreshMinutes
	if refreshMinutes <= 0 {
		refreshMinutes = 5
	}
	interval := time.Duration(refreshMinutes) * time.Minute

	// Run once immediately in a goroutine to avoid blocking startup
	go func() {
		defer log.Println("Initial Google Tasks shopping list sync goroutine terminated.")
		select {
		case <-ctx.Done():
			return
		default:
			log.Println("Performing initial Google Tasks shopping list sync.")
			syncAllStores()
		}
	}()

	// Start periodic checks
	ticker := time.NewTicker(interval)
	go func() {
		defer log.Println("Periodic Google Tasks shopping list sync goroutine terminated.")
		for {
			select {
			case <-ctx.Done():
				ticker.Stop()
				return
			case <-ticker.C:
				log.Printf("Performing periodic Google Tasks shopping list sync (every %d minutes).", refreshMinutes)
				syncAllStores()
			}
		}
	}()
	return cancel
}

// syncAllStores iterates through all configured, non-disabled stores and
// triggers a sync for each one.
func syncAllStores() {
	cfg := config.GetConfig()
	for i, store := range cfg.Shopping.Store {
		if !store.Disabled {
			storeID := i + 1
			if err := syncStore(storeID, store.Name); err != nil {
				log.Printf("Error syncing shopping list for store '%s': %v", store.Name, err)
			}
		}
	}
}

// syncStore handles the two-way synchronization of a single shopping list
// with its corresponding Google Tasks list.
func syncStore(storeID int, storeName string) error {
	log.Printf("Starting sync for store: %s (ID: %d)", storeName, storeID)

	tasksService, err := google.NewTasksService()
	if err != nil {
		return fmt.Errorf("unable to create tasks service: %w", err)
	}

	// 1. Find or create the Google Tasks list.
	listName := storeName // Default to store name
	if mappedName, ok := config.GetConfig().Shopping.GoogleTasks.ListMapping[storeName]; ok {
		listName = mappedName
	}
	taskList, err := getTaskList(tasksService, listName)
	if err != nil {
		return fmt.Errorf("failed to get or create task list '%s': %w", listName, err)
	}
	log.Printf("Found task list '%s' with ID: %s", taskList.Title, taskList.Id)

	// 2. Fetch all tasks from Google Tasks and all items from the local DB.
	remoteTasks, err := tasksService.Tasks.List(taskList.Id).Do()
	if err != nil {
		return fmt.Errorf("failed to retrieve tasks for list '%s': %w", taskList.Title, err)
	}
	localItems, err := database.GetShoppingItemsByStore(storeID)
	if err != nil {
		return fmt.Errorf("failed to retrieve local items for store ID %d: %w", storeID, err)
	}

	// 3. Reconcile the lists (this is a simplified version).
	localMap := make(map[string]database.ShoppingItem)
	for _, item := range localItems {
		localMap[strings.ToLower(item.Name)] = item
	}

	remoteMap := make(map[string]*tasks.Task)
	for _, task := range remoteTasks.Items {
		remoteMap[strings.ToLower(task.Title)] = task
	}

	// Reconcile: local -> remote
	for _, item := range localItems {
		if _, exists := remoteMap[strings.ToLower(item.Name)]; !exists {
			log.Printf("Sync: Creating remote task for local-only item: '%s'", item.Name)
			newTask := &tasks.Task{
				Title: item.Name,
			}
			if item.Checked {
				newTask.Status = "completed"
			} else {
				newTask.Status = "needsAction"
			}
			_, err := tasksService.Tasks.Insert(taskList.Id, newTask).Do()
			if err != nil {
				log.Printf("Error creating task for '%s': %v", item.Name, err)
				// Continue, don't let one failure stop the whole sync
			}
		}
	}

	// Reconcile: remote -> local
	for _, task := range remoteTasks.Items {
		if _, exists := localMap[strings.ToLower(task.Title)]; !exists {
			log.Printf("Sync: Creating local item for remote-only task: '%s'", task.Title)
			newItem := database.ShoppingItem{
				Name:    task.Title,
				StoreID: storeID,
				Checked: task.Status == "completed",
			}
			if _, err := database.AddShoppingItem(newItem); err != nil {
				log.Printf("Error creating local shopping item for task '%s': %v", task.Title, err)
			}
		}
	}

	// Reconcile: Sync status for items that exist in both
	for _, task := range remoteTasks.Items {
		taskTitleLower := strings.ToLower(task.Title)
		if localItem, exists := localMap[taskTitleLower]; exists {
			remoteCompleted := task.Status == "completed"
			if localItem.Checked != remoteCompleted {
				// If statuses differ, decide which one to keep.
				// A simple rule: the remote task's `updated` time vs. a local timestamp.
				// Since we don't have a local timestamp, we can default to
				// updating the local item to match the remote, or vice-versa.
				// Let's assume for now the Google Tasks is the source of truth if different.
				log.Printf("Sync: Status differs for '%s'. Local: %t, Remote: %t. Updating local.",
					task.Title, localItem.Checked, remoteCompleted)
				localItem.Checked = remoteCompleted
				if err := database.UpdateShoppingItem(localItem); err != nil {
					log.Printf("Error updating local item status for '%s': %v", localItem.Name, err)
				}
			}
		}
	}

	log.Printf("Finished sync for store: %s. Found %d local items and %d remote tasks.",
		storeName, len(localItems), len(remoteTasks.Items))

	return nil
}

// getTaskList finds a Google Tasks list by its title. If it doesn't exist,
// it creates a new one.
func getTaskList(srv *tasks.Service, title string) (*tasks.TaskList, error) {
	lists, err := srv.Tasklists.List().Do()
	if err != nil {
		return nil, fmt.Errorf("unable to retrieve task lists: %w", err)
	}
	for _, list := range lists.Items {
		if list.Title == title {
			return list, nil
		}
	}

	// If we reach here, the list was not found, so create it.
	log.Printf("Task list '%s' not found, creating it.", title)
	newList := &tasks.TaskList{
		Title: title,
	}
	return srv.Tasklists.Insert(newList).Do()
}
