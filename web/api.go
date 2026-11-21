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

package web

import (
	"crypto/subtle"
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/liquidgecka/homehub/config"
	"github.com/liquidgecka/homehub/database"
	"github.com/liquidgecka/homehub/dpms"
	"github.com/liquidgecka/homehub/homeassistant"
	"github.com/liquidgecka/homehub/reminders"
	"github.com/liquidgecka/homehub/shopping"
)

var (
	apiHooksMu       sync.RWMutex
	nextPhotoHandler func()
)

// SetNextPhotoHandler registers the callback triggered by POST
// /api/v1/next-photo.
func SetNextPhotoHandler(fn func()) {
	apiHooksMu.Lock()
	defer apiHooksMu.Unlock()
	nextPhotoHandler = fn
}

func registerAPIRoutes() {
	http.HandleFunc("/api/v1/status", handleAPIStatus)
	http.HandleFunc("/api/v1/away", handleAPIAway)
	http.HandleFunc("/api/v1/restart", handleAPIRestart)
	http.HandleFunc("/api/v1/next-photo", handleAPINextPhoto)
	http.HandleFunc("/api/v1/alert", handleAPIAlert)
	http.HandleFunc("/api/v1/shopping", handleAPIShopping)
	http.HandleFunc("/api/v1/shopping/items", handleAPIShoppingItems)
	http.HandleFunc("/api/v1/shopping/items/", handleAPIShoppingItemByID)
	http.HandleFunc("/api/v1/reminders", handleAPIReminders)
	http.HandleFunc("/api/v1/reminders/", handleAPIReminderAction)
}

func checkAPIAuth(w http.ResponseWriter, r *http.Request) bool {
	expected := config.GetConfig().App.APIToken
	if expected == "" {
		return true
	}

	var provided string
	if auth := r.Header.Get("Authorization"); auth != "" {
		const prefix = "bearer "
		if len(auth) >= len(prefix) &&
			strings.EqualFold(auth[:len(prefix)], prefix) {
			provided = strings.TrimSpace(auth[len(prefix):])
		}
	}
	if provided == "" {
		provided = strings.TrimSpace(r.Header.Get("X-API-Key"))
	}

	if subtle.ConstantTimeCompare([]byte(provided), []byte(expected)) != 1 {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return false
	}
	return true
}

func writeJSON(w http.ResponseWriter, status int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeJSONError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

type apiStoreSummary struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type apiShoppingItem struct {
	ID       int    `json:"id"`
	Name     string `json:"name"`
	Quantity int    `json:"quantity"`
	Checked  bool   `json:"checked"`
	StoreID  int    `json:"store_id"`
}

type apiShoppingStore struct {
	ID    int               `json:"id"`
	Name  string            `json:"name"`
	Items []apiShoppingItem `json:"items"`
}

func activeStoresMap() (map[int]string, []apiStoreSummary) {
	cfg := config.GetConfig()
	byID := make(map[int]string)
	summaries := make([]apiStoreSummary, 0)
	for i, s := range cfg.Shopping.Store {
		if s.Disabled {
			continue
		}
		id := i + 1
		byID[id] = s.Name
		summaries = append(summaries, apiStoreSummary{
			ID:   id,
			Name: s.Name,
		})
	}
	return byID, summaries
}

func handleAPIStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if !checkAPIAuth(w, r) {
		return
	}

	pendingCount := 0
	if pending, err := reminders.GetPendingReminders(); err == nil {
		pendingCount = len(pending)
	}
	_, stores := activeStoresMap()

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":            "ok",
		"away":              dpms.IsAway(),
		"display_on":        dpms.IsDisplayOn(),
		"active_view":       homeassistant.GetActiveView(),
		"pending_reminders": pendingCount,
		"stores":            stores,
	})
}

func handleAPIAway(w http.ResponseWriter, r *http.Request) {
	if !checkAPIAuth(w, r) {
		return
	}

	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, awayStatusPayload())
	case http.MethodPost:
		var req struct {
			Away *bool `json:"away"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil ||
			req.Away == nil {
			writeJSONError(
				w, http.StatusBadRequest, "'away' boolean is required",
			)
			return
		}
		if err := database.SetAwayMode(*req.Away); err != nil {
			log.Printf("API: Failed to persist away mode: %v", err)
			writeJSONError(
				w, http.StatusInternalServerError,
				"failed to persist away mode",
			)
			return
		}
		dpms.SetAwayMode(*req.Away)
		writeJSON(w, http.StatusOK, awayStatusPayload())
	default:
		writeJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func handleAPIRestart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if !checkAPIAuth(w, r) {
		return
	}
	handleAppRestart(w, r)
}

func handleAPINextPhoto(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if !checkAPIAuth(w, r) {
		return
	}
	apiHooksMu.RLock()
	fn := nextPhotoHandler
	apiHooksMu.RUnlock()
	if fn != nil {
		fn()
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func handleAPIAlert(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if !checkAPIAuth(w, r) {
		return
	}

	var req struct {
		Title           string  `json:"title"`
		Message         string  `json:"message"`
		Level           string  `json:"level"`
		DurationSeconds float64 `json:"duration_seconds"`
		Dismiss         bool    `json:"dismiss"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid JSON payload")
		return
	}

	if req.Dismiss {
		homeassistant.DismissAlert()
		writeJSON(w, http.StatusOK, map[string]string{"status": "dismissed"})
		return
	}

	title := strings.TrimSpace(req.Title)
	msg := strings.TrimSpace(req.Message)
	if title == "" && msg == "" {
		writeJSONError(
			w, http.StatusBadRequest, "title or message is required",
		)
		return
	}
	if title == "" {
		title = "Home Assistant"
	}
	if msg == "" {
		msg = title
	}
	dur := time.Duration(req.DurationSeconds * float64(time.Second))
	homeassistant.TriggerAlert(homeassistant.Alert{
		Title:    title,
		Message:  msg,
		Level:    req.Level,
		Duration: dur,
	})
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func handleAPIShopping(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if !checkAPIAuth(w, r) {
		return
	}

	allItems, err := database.GetShoppingItems()
	if err != nil {
		writeJSONError(
			w, http.StatusInternalServerError, "failed to get shopping items",
		)
		return
	}

	_, summaries := activeStoresMap()
	byStore := make(map[int][]apiShoppingItem, len(summaries))
	for _, item := range allItems {
		byStore[item.StoreID] = append(byStore[item.StoreID], apiShoppingItem{
			ID:       item.ID,
			Name:     item.Name,
			Quantity: item.Quantity,
			Checked:  item.Checked,
			StoreID:  item.StoreID,
		})
	}

	stores := make([]apiShoppingStore, 0, len(summaries))
	for _, s := range summaries {
		items := byStore[s.ID]
		if items == nil {
			items = []apiShoppingItem{}
		}
		stores = append(stores, apiShoppingStore{
			ID:    s.ID,
			Name:  s.Name,
			Items: items,
		})
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{"stores": stores})
}

func handleAPIShoppingItems(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if !checkAPIAuth(w, r) {
		return
	}

	var req struct {
		StoreID  int    `json:"store_id"`
		Name     string `json:"name"`
		Quantity int    `json:"quantity"`
		Checked  bool   `json:"checked"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid JSON payload")
		return
	}

	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		writeJSONError(w, http.StatusBadRequest, "item name is required")
		return
	}
	if req.Quantity <= 0 {
		req.Quantity = 1
	}

	storesByID, _ := activeStoresMap()
	if _, ok := storesByID[req.StoreID]; !ok {
		writeJSONError(w, http.StatusBadRequest, "invalid or disabled store_id")
		return
	}

	item := database.ShoppingItem{
		Name:     req.Name,
		Quantity: req.Quantity,
		Checked:  req.Checked,
		StoreID:  req.StoreID,
	}
	id, err := shopping.AddItemWithID(item)
	if err != nil {
		writeJSONError(
			w, http.StatusInternalServerError, "failed to add shopping item",
		)
		return
	}
	item.ID = id

	writeJSON(w, http.StatusCreated, apiShoppingItem{
		ID:       item.ID,
		Name:     item.Name,
		Quantity: item.Quantity,
		Checked:  item.Checked,
		StoreID:  item.StoreID,
	})
}

func handleAPIShoppingItemByID(w http.ResponseWriter, r *http.Request) {
	if !checkAPIAuth(w, r) {
		return
	}

	idStr := strings.TrimPrefix(r.URL.Path, "/api/v1/shopping/items/")
	id, err := strconv.Atoi(idStr)
	if err != nil || id <= 0 {
		writeJSONError(w, http.StatusBadRequest, "invalid item ID")
		return
	}

	item, err := shopping.GetShoppingItemByID(id)
	if err != nil {
		writeJSONError(w, http.StatusNotFound, "shopping item not found")
		return
	}

	switch r.Method {
	case http.MethodDelete:
		if err := shopping.DeleteItem(id); err != nil {
			writeJSONError(
				w, http.StatusInternalServerError,
				"failed to delete shopping item",
			)
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"status": "deleted",
			"id":     id,
		})
	case http.MethodPatch, http.MethodPut:
		var req struct {
			Name     *string `json:"name"`
			Quantity *int    `json:"quantity"`
			Checked  *bool   `json:"checked"`
			StoreID  *int    `json:"store_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSONError(w, http.StatusBadRequest, "invalid JSON payload")
			return
		}
		if req.Name != nil {
			trimmed := strings.TrimSpace(*req.Name)
			if trimmed == "" {
				writeJSONError(
					w, http.StatusBadRequest, "item name cannot be empty",
				)
				return
			}
			item.Name = trimmed
		}
		if req.Quantity != nil {
			if *req.Quantity <= 0 {
				writeJSONError(
					w, http.StatusBadRequest, "quantity must be positive",
				)
				return
			}
			item.Quantity = *req.Quantity
		}
		if req.Checked != nil {
			item.Checked = *req.Checked
		}
		if req.StoreID != nil {
			storesByID, _ := activeStoresMap()
			if _, ok := storesByID[*req.StoreID]; !ok {
				writeJSONError(
					w, http.StatusBadRequest, "invalid or disabled store_id",
				)
				return
			}
			item.StoreID = *req.StoreID
		}
		if err := shopping.UpdateItem(item); err != nil {
			writeJSONError(
				w, http.StatusInternalServerError,
				"failed to update shopping item",
			)
			return
		}
		writeJSON(w, http.StatusOK, apiShoppingItem{
			ID:       item.ID,
			Name:     item.Name,
			Quantity: item.Quantity,
			Checked:  item.Checked,
			StoreID:  item.StoreID,
		})
	default:
		writeJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func handleAPIReminders(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if !checkAPIAuth(w, r) {
		return
	}

	rems, err := database.GetRemindersDB()
	if err != nil {
		writeJSONError(
			w, http.StatusInternalServerError, "failed to get reminders",
		)
		return
	}
	if rems == nil {
		rems = []database.Reminder{}
	}

	pendingCount := 0
	for _, rem := range rems {
		if rem.Enabled && !rem.Acknowledged && !rem.LastTriggered.IsZero() {
			pendingCount++
		}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"reminders":     rems,
		"pending_count": pendingCount,
	})
}

func handleAPIReminderAction(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if !checkAPIAuth(w, r) {
		return
	}

	rest := strings.TrimPrefix(r.URL.Path, "/api/v1/reminders/")
	parts := strings.Split(strings.Trim(rest, "/"), "/")
	if len(parts) != 2 || parts[1] != "acknowledge" {
		writeJSONError(w, http.StatusNotFound, "unknown reminder action")
		return
	}
	id, err := strconv.Atoi(parts[0])
	if err != nil || id <= 0 {
		writeJSONError(w, http.StatusBadRequest, "invalid reminder ID")
		return
	}

	if _, err := database.GetReminderByIDDB(id); err != nil {
		writeJSONError(w, http.StatusNotFound, "reminder not found")
		return
	}

	if err := reminders.AcknowledgeReminder(id); err != nil {
		writeJSONError(
			w, http.StatusInternalServerError,
			"failed to acknowledge reminder",
		)
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status": "acknowledged",
		"id":     id,
	})
}
