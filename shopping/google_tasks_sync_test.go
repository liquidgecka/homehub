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
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"google.golang.org/api/option"
	"google.golang.org/api/tasks/v1"

	"github.com/liquidgecka/homehub/config"
	"github.com/liquidgecka/homehub/database"
	"github.com/liquidgecka/homehub/testutils"
)

func newMockTasksService(
	t *testing.T, handler http.Handler,
) (*tasks.Service, *httptest.Server) {
	ts := httptest.NewServer(handler)
	srv, err := tasks.NewService(
		context.Background(),
		option.WithEndpoint(ts.URL),
		option.WithHTTPClient(ts.Client()),
	)
	if err != nil {
		t.Fatalf("unable to create tasks service: %v", err)
	}
	return srv, ts
}

func TestGetTaskList(t *testing.T) {
	testutils.SetupLogCapture(t)

	var createdList *tasks.TaskList
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/lists") {
			listResp := &tasks.TaskLists{
				Items: []*tasks.TaskList{
					{Id: "list-1", Title: "Existing List"},
				},
			}
			_ = json.NewEncoder(w).Encode(listResp)
			return
		}
		if r.Method == http.MethodPost && strings.Contains(r.URL.Path, "/lists") {
			var tl tasks.TaskList
			_ = json.NewDecoder(r.Body).Decode(&tl)
			tl.Id = "list-new"
			createdList = &tl
			_ = json.NewEncoder(w).Encode(&tl)
			return
		}
		http.NotFound(w, r)
	})

	srv, ts := newMockTasksService(t, handler)
	defer ts.Close()

	// 1. Existing list
	tl, err := getTaskList(srv, "Existing List")
	if err != nil {
		t.Fatalf("getTaskList failed: %v", err)
	}
	if tl.Id != "list-1" || tl.Title != "Existing List" {
		t.Errorf("unexpected task list: %+v", tl)
	}

	// 2. Non-existent list -> creates new
	tlNew, err := getTaskList(srv, "New Store")
	if err != nil {
		t.Fatalf("getTaskList creation failed: %v", err)
	}
	if tlNew.Id != "list-new" || tlNew.Title != "New Store" {
		t.Errorf("unexpected created task list: %+v", tlNew)
	}
	if createdList == nil || createdList.Title != "New Store" {
		t.Errorf("expected server to receive list creation for New Store")
	}

	// 3. Error case with failing server
	errHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "internal server error", http.StatusInternalServerError)
	})
	errSrv, errTs := newMockTasksService(t, errHandler)
	defer errTs.Close()

	_, err = getTaskList(errSrv, "Any List")
	if err == nil {
		t.Error("expected error when server returns 500")
	}
}

func TestSyncStore(t *testing.T) {
	testutils.SetupLogCapture(t)

	var mu sync.Mutex
	var createdRemoteTasks []*tasks.Task
	var createdLocalItems []database.ShoppingItem
	var updatedLocalItems []database.ShoppingItem

	origAdd := database.AddShoppingItem
	origGetByStore := database.GetShoppingItemsByStore
	origUpdate := database.UpdateShoppingItem
	defer func() {
		database.AddShoppingItem = origAdd
		database.GetShoppingItemsByStore = origGetByStore
		database.UpdateShoppingItem = origUpdate
	}()

	database.AddShoppingItem = func(item database.ShoppingItem) (int, error) {
		mu.Lock()
		defer mu.Unlock()
		createdLocalItems = append(createdLocalItems, item)
		return len(createdLocalItems), nil
	}
	database.UpdateShoppingItem = func(item database.ShoppingItem) error {
		mu.Lock()
		defer mu.Unlock()
		updatedLocalItems = append(updatedLocalItems, item)
		return nil
	}
	database.GetShoppingItemsByStore = func(
		storeID int,
	) ([]database.ShoppingItem, error) {
		return []database.ShoppingItem{
			{ID: 1, Name: "Apples", StoreID: 1, Checked: false},
			{ID: 2, Name: "Bananas", StoreID: 1, Checked: false},
		}, nil
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		w.Header().Set("Content-Type", "application/json")

		// Tasklists List
		if r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/lists") {
			_ = json.NewEncoder(w).Encode(&tasks.TaskLists{
				Items: []*tasks.TaskList{
					{Id: "tl-store1", Title: "Store1"},
				},
			})
			return
		}

		// Tasks List
		if r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/tasks") {
			_ = json.NewEncoder(w).Encode(&tasks.Tasks{
				Items: []*tasks.Task{
					{Id: "t1", Title: "Bananas", Status: "completed"},
					{Id: "t2", Title: "Milk", Status: "needsAction"},
				},
			})
			return
		}

		// Tasks Insert (remote creation)
		if r.Method == http.MethodPost && strings.Contains(r.URL.Path, "/tasks") {
			var task tasks.Task
			_ = json.NewDecoder(r.Body).Decode(&task)
			task.Id = fmt.Sprintf("t-new-%d", len(createdRemoteTasks)+1)
			createdRemoteTasks = append(createdRemoteTasks, &task)
			_ = json.NewEncoder(w).Encode(&task)
			return
		}

		http.NotFound(w, r)
	})

	srv, ts := newMockTasksService(t, handler)
	defer ts.Close()

	origNewTasksService := newTasksService
	newTasksService = func() (*tasks.Service, error) {
		return srv, nil
	}
	defer func() { newTasksService = origNewTasksService }()

	cfgCleanup := config.SetMockConfig(config.Config{
		Shopping: config.ShoppingConfig{
			GoogleTasks: config.GoogleTasksConfig{
				Enabled: true,
				ListMapping: map[string]string{
					"My Store": "Store1",
				},
			},
		},
	})
	defer cfgCleanup()

	err := syncStore(1, "My Store")
	if err != nil {
		t.Fatalf("syncStore failed: %v", err)
	}

	// Local-only item "Apples" should have been created remotely
	if len(createdRemoteTasks) != 1 || createdRemoteTasks[0].Title != "Apples" {
		t.Errorf("expected remote task for Apples, got: %+v", createdRemoteTasks)
	}

	// Remote-only task "Milk" should have been created locally
	if len(createdLocalItems) != 1 || createdLocalItems[0].Name != "Milk" {
		t.Errorf("expected local item for Milk, got: %+v", createdLocalItems)
	}

	// Item "Bananas" status differs (remote completed, local unchecked)
	if len(updatedLocalItems) != 1 || updatedLocalItems[0].Name != "Bananas" ||
		!updatedLocalItems[0].Checked {
		t.Errorf(
			"expected local update for Bananas (Checked=true), got: %+v",
			updatedLocalItems,
		)
	}
}

func TestSyncStore_ServiceInitError(t *testing.T) {
	testutils.SetupLogCapture(t)

	origNewTasksService := newTasksService
	newTasksService = func() (*tasks.Service, error) {
		return nil, fmt.Errorf("mock init error")
	}
	defer func() { newTasksService = origNewTasksService }()

	err := syncStore(1, "Groceries")
	if err == nil {
		t.Error("expected error when newTasksService fails")
	}
}

func TestSyncAllStores(t *testing.T) {
	testutils.SetupLogCapture(t)

	// Disabled config
	cfgDisabled := config.SetMockConfig(config.Config{
		Shopping: config.ShoppingConfig{
			GoogleTasks: config.GoogleTasksConfig{
				Enabled: false,
			},
		},
	})
	syncAllStores() // should be a no-op
	cfgDisabled()

	// Enabled config with mock service
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.HasSuffix(r.URL.Path, "/lists") {
			_ = json.NewEncoder(w).Encode(&tasks.TaskLists{})
			return
		}
		if strings.Contains(r.URL.Path, "/tasks") {
			_ = json.NewEncoder(w).Encode(&tasks.Tasks{})
			return
		}
	})
	srv, ts := newMockTasksService(t, handler)
	defer ts.Close()

	origNew := newTasksService
	newTasksService = func() (*tasks.Service, error) { return srv, nil }
	defer func() { newTasksService = origNew }()

	cfgCleanup := config.SetMockConfig(config.Config{
		Shopping: config.ShoppingConfig{
			GoogleTasks: config.GoogleTasksConfig{
				Enabled: true,
			},
			Store: []config.StoreConfig{
				{Name: "Store 1", Disabled: false},
				{Name: "Store 2 (Disabled)", Disabled: true},
			},
		},
	})
	defer cfgCleanup()

	// Should sync Store 1 without panic or error
	syncAllStores()
}

func TestStartGoogleTasksSync_Enabled(t *testing.T) {
	testutils.SetupLogCapture(t)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.HasSuffix(r.URL.Path, "/lists") {
			_ = json.NewEncoder(w).Encode(&tasks.TaskLists{})
			return
		}
		if strings.Contains(r.URL.Path, "/tasks") {
			_ = json.NewEncoder(w).Encode(&tasks.Tasks{})
			return
		}
	})
	srv, ts := newMockTasksService(t, handler)
	defer ts.Close()

	origNew := newTasksService
	newTasksService = func() (*tasks.Service, error) { return srv, nil }
	defer func() { newTasksService = origNew }()

	syncDone := make(chan struct{})
	var syncOnce sync.Once
	origSyncAll := syncAllStores
	syncAllStores = func() {
		syncOnce.Do(func() {
			close(syncDone)
		})
	}
	defer func() { syncAllStores = origSyncAll }()

	cfgCleanup := config.SetMockConfig(config.Config{
		Shopping: config.ShoppingConfig{
			GoogleTasks: config.GoogleTasksConfig{
				Enabled:        true,
				RefreshMinutes: 1,
			},
			Store: []config.StoreConfig{
				{Name: "Trader Joes", Disabled: false},
			},
		},
	})
	defer cfgCleanup()

	ctx, cancelCtx := context.WithCancel(context.Background())
	cancel := StartGoogleTasksSync(ctx)
	if cancel == nil {
		t.Fatal("expected non-nil cancel function")
	}

	select {
	case <-syncDone:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for initial sync")
	}
	cancel()
	cancelCtx()
}
