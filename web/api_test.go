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
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/liquidgecka/homehub/config"
	"github.com/liquidgecka/homehub/database"
	"github.com/liquidgecka/homehub/dpms"
	"github.com/liquidgecka/homehub/homeassistant"
	"github.com/liquidgecka/homehub/reminders"
)

func TestAPIAuth(t *testing.T) {
	cleanupCfg := config.SetMockConfig(config.Config{
		App: config.AppConfig{APIToken: "super-secret-token"},
	})
	defer cleanupCfg()

	t.Run("Missing token returns 401", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/status", nil)
		rr := httptest.NewRecorder()
		handleAPIStatus(rr, req)
		if rr.Code != http.StatusUnauthorized {
			t.Errorf("got %d, want 401", rr.Code)
		}
	})

	t.Run("Wrong token returns 401", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/status", nil)
		req.Header.Set("Authorization", "Bearer wrong")
		rr := httptest.NewRecorder()
		handleAPIStatus(rr, req)
		if rr.Code != http.StatusUnauthorized {
			t.Errorf("got %d, want 401", rr.Code)
		}
	})

	t.Run("Bearer token succeeds", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/status", nil)
		req.Header.Set("Authorization", "Bearer super-secret-token")
		rr := httptest.NewRecorder()
		handleAPIStatus(rr, req)
		if rr.Code != http.StatusOK {
			t.Errorf("got %d, want 200", rr.Code)
		}
	})

	t.Run("X-API-Key token succeeds", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/status", nil)
		req.Header.Set("X-API-Key", "super-secret-token")
		rr := httptest.NewRecorder()
		handleAPIStatus(rr, req)
		if rr.Code != http.StatusOK {
			t.Errorf("got %d, want 200", rr.Code)
		}
	})
}

func TestAPIStatusAwayAndControls(t *testing.T) {
	dpms.ResetState()
	defer dpms.ResetState()
	homeassistant.ClearAlertHandlers()
	defer homeassistant.ClearAlertHandlers()
	reminders.ClearListeners()
	defer reminders.ClearListeners()

	_, cleanupDB, err := database.NewTestDB()
	if err != nil {
		t.Fatalf("NewTestDB failed: %v", err)
	}
	defer cleanupDB()

	cleanupCfg := config.SetMockConfig(config.Config{
		Shopping: config.ShoppingConfig{
			Store: []config.StoreConfig{
				{Name: "Costco"},
				{Name: "Old Store", Disabled: true},
				{Name: "Trader Joe's"},
			},
		},
	})
	defer cleanupCfg()

	_, _ = database.AddReminderDB(database.Reminder{
		Title:         "Trash day",
		Time:          "18:00",
		Days:          "Everyday",
		Enabled:       true,
		Acknowledged:  false,
		LastTriggered: time.Now(),
	})

	homeassistant.SetActiveView("calendar")
	defer homeassistant.SetActiveView("home")

	// Status.
	reqStatus := httptest.NewRequest("GET", "/api/v1/status", nil)
	rrStatus := httptest.NewRecorder()
	handleAPIStatus(rrStatus, reqStatus)
	if rrStatus.Code != http.StatusOK {
		t.Fatalf("status got %d, want 200", rrStatus.Code)
	}
	var statusResp struct {
		Status           string            `json:"status"`
		Away             bool              `json:"away"`
		DisplayOn        bool              `json:"display_on"`
		ActiveView       string            `json:"active_view"`
		PendingReminders int               `json:"pending_reminders"`
		Stores           []apiStoreSummary `json:"stores"`
	}
	if err := json.NewDecoder(rrStatus.Body).Decode(&statusResp); err != nil {
		t.Fatalf("decode status: %v", err)
	}
	if statusResp.Status != "ok" || statusResp.ActiveView != "calendar" ||
		statusResp.PendingReminders != 1 || len(statusResp.Stores) != 2 ||
		statusResp.Stores[0].ID != 1 || statusResp.Stores[1].ID != 3 {
		t.Errorf("unexpected status payload: %+v", statusResp)
	}

	// Away toggle via /api/v1/away.
	reqAway := httptest.NewRequest(
		"POST", "/api/v1/away", strings.NewReader(`{"away":true}`),
	)
	rrAway := httptest.NewRecorder()
	handleAPIAway(rrAway, reqAway)
	if rrAway.Code != http.StatusOK || !dpms.IsAway() {
		t.Errorf("expected away enabled, code=%d, away=%v",
			rrAway.Code, dpms.IsAway())
	}

	reqGetAway := httptest.NewRequest("GET", "/api/v1/away", nil)
	rrGetAway := httptest.NewRecorder()
	handleAPIAway(rrGetAway, reqGetAway)
	if rrGetAway.Code != http.StatusOK ||
		!strings.Contains(rrGetAway.Body.String(), `"away":true`) {
		t.Errorf("unexpected GET /api/v1/away: %s", rrGetAway.Body.String())
	}

	// Next photo callback.
	called := false
	SetNextPhotoHandler(func() { called = true })
	defer SetNextPhotoHandler(nil)
	reqNext := httptest.NewRequest("POST", "/api/v1/next-photo", nil)
	rrNext := httptest.NewRecorder()
	handleAPINextPhoto(rrNext, reqNext)
	if rrNext.Code != http.StatusOK || !called {
		t.Errorf("next-photo code=%d called=%v", rrNext.Code, called)
	}

	// Alert trigger & dismiss.
	var gotAlerts []homeassistant.Alert
	homeassistant.RegisterAlertHandler(func(a homeassistant.Alert) {
		gotAlerts = append(gotAlerts, a)
	})
	reqAlert := httptest.NewRequest(
		"POST", "/api/v1/alert",
		strings.NewReader(`{"title":"HA","message":"Doorbell","level":"warn"}`),
	)
	rrAlert := httptest.NewRecorder()
	handleAPIAlert(rrAlert, reqAlert)
	if rrAlert.Code != http.StatusOK {
		t.Fatalf("alert got %d, want 200", rrAlert.Code)
	}
	reqDismiss := httptest.NewRequest(
		"POST", "/api/v1/alert", strings.NewReader(`{"dismiss":true}`),
	)
	rrDismiss := httptest.NewRecorder()
	handleAPIAlert(rrDismiss, reqDismiss)
	if rrDismiss.Code != http.StatusOK || len(gotAlerts) != 2 ||
		gotAlerts[0].Level != "warning" || !gotAlerts[1].Dismiss {
		t.Errorf("unexpected alert results: code=%d alerts=%+v",
			rrDismiss.Code, gotAlerts)
	}
}

func TestAPIShoppingCRUD(t *testing.T) {
	_, cleanupDB, err := database.NewTestDB()
	if err != nil {
		t.Fatalf("NewTestDB failed: %v", err)
	}
	defer cleanupDB()

	cleanupCfg := config.SetMockConfig(config.Config{
		Shopping: config.ShoppingConfig{
			Store: []config.StoreConfig{
				{Name: "Costco"},
				{Name: "Closed", Disabled: true},
			},
		},
	})
	defer cleanupCfg()

	// Create item.
	reqCreate := httptest.NewRequest(
		"POST", "/api/v1/shopping/items",
		strings.NewReader(`{"store_id":1,"name":"Coffee","quantity":2}`),
	)
	rrCreate := httptest.NewRecorder()
	handleAPIShoppingItems(rrCreate, reqCreate)
	if rrCreate.Code != http.StatusCreated {
		t.Fatalf("create got %d: %s", rrCreate.Code, rrCreate.Body.String())
	}
	var created apiShoppingItem
	if err := json.NewDecoder(rrCreate.Body).Decode(&created); err != nil {
		t.Fatalf("decode created: %v", err)
	}
	if created.ID <= 0 || created.Name != "Coffee" || created.Quantity != 2 {
		t.Fatalf("unexpected created item: %+v", created)
	}

	// Reject disabled store.
	reqDisabled := httptest.NewRequest(
		"POST", "/api/v1/shopping/items",
		strings.NewReader(`{"store_id":2,"name":"Milk"}`),
	)
	rrDisabled := httptest.NewRecorder()
	handleAPIShoppingItems(rrDisabled, reqDisabled)
	if rrDisabled.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for disabled store, got %d", rrDisabled.Code)
	}

	// GET shopping list.
	reqList := httptest.NewRequest("GET", "/api/v1/shopping", nil)
	rrList := httptest.NewRecorder()
	handleAPIShopping(rrList, reqList)
	if rrList.Code != http.StatusOK {
		t.Fatalf("list got %d", rrList.Code)
	}
	var listResp struct {
		Stores []apiShoppingStore `json:"stores"`
	}
	if err := json.NewDecoder(rrList.Body).Decode(&listResp); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	if len(listResp.Stores) != 1 || len(listResp.Stores[0].Items) != 1 {
		t.Fatalf("unexpected shopping list: %+v", listResp)
	}

	// PATCH item.
	itemPath := "/api/v1/shopping/items/" +
		strings.TrimSpace(jsonNumberString(created.ID))
	reqPatch := httptest.NewRequest(
		"PATCH", itemPath,
		strings.NewReader(`{"checked":true,"quantity":3}`),
	)
	rrPatch := httptest.NewRecorder()
	handleAPIShoppingItemByID(rrPatch, reqPatch)
	if rrPatch.Code != http.StatusOK {
		t.Fatalf("patch got %d: %s", rrPatch.Code, rrPatch.Body.String())
	}
	var patched apiShoppingItem
	_ = json.NewDecoder(rrPatch.Body).Decode(&patched)
	if !patched.Checked || patched.Quantity != 3 || patched.Name != "Coffee" {
		t.Errorf("unexpected patched item: %+v", patched)
	}

	// DELETE item.
	reqDel := httptest.NewRequest("DELETE", itemPath, nil)
	rrDel := httptest.NewRecorder()
	handleAPIShoppingItemByID(rrDel, reqDel)
	if rrDel.Code != http.StatusOK {
		t.Fatalf("delete got %d", rrDel.Code)
	}

	// Deleting again returns 404.
	reqMissing := httptest.NewRequest("DELETE", itemPath, nil)
	rrMissing := httptest.NewRecorder()
	handleAPIShoppingItemByID(rrMissing, reqMissing)
	if rrMissing.Code != http.StatusNotFound {
		t.Errorf("expected 404 for deleted item, got %d", rrMissing.Code)
	}
}

func TestAPIRemindersAndAcknowledge(t *testing.T) {
	reminders.ClearListeners()
	defer reminders.ClearListeners()

	_, cleanupDB, err := database.NewTestDB()
	if err != nil {
		t.Fatalf("NewTestDB failed: %v", err)
	}
	defer cleanupDB()

	id, err := database.AddReminderDB(database.Reminder{
		Title:         "Take pills",
		Time:          "09:00",
		Days:          "Everyday",
		Enabled:       true,
		Acknowledged:  false,
		LastTriggered: time.Now(),
	})
	if err != nil {
		t.Fatalf("AddReminderDB failed: %v", err)
	}

	reqList := httptest.NewRequest("GET", "/api/v1/reminders", nil)
	rrList := httptest.NewRecorder()
	handleAPIReminders(rrList, reqList)
	if rrList.Code != http.StatusOK {
		t.Fatalf("reminders list got %d", rrList.Code)
	}
	var listResp struct {
		Reminders    []database.Reminder `json:"reminders"`
		PendingCount int                 `json:"pending_count"`
	}
	if err := json.NewDecoder(rrList.Body).Decode(&listResp); err != nil {
		t.Fatalf("decode reminders: %v", err)
	}
	if listResp.PendingCount != 1 || len(listResp.Reminders) != 1 {
		t.Fatalf("unexpected reminders payload: %+v", listResp)
	}

	ackPath := "/api/v1/reminders/" + jsonNumberString(id) + "/acknowledge"
	reqAck := httptest.NewRequest("POST", ackPath, nil)
	rrAck := httptest.NewRecorder()
	handleAPIReminderAction(rrAck, reqAck)
	if rrAck.Code != http.StatusOK {
		t.Fatalf("ack got %d: %s", rrAck.Code, rrAck.Body.String())
	}

	pending, _ := reminders.GetPendingReminders()
	if len(pending) != 0 {
		t.Errorf("expected 0 pending reminders after ack, got %d", len(pending))
	}
}

func jsonNumberString(n int) string {
	b, _ := json.Marshal(n)
	return string(b)
}

func TestAPIRestart(t *testing.T) {
	restartChan := make(chan bool, 1)
	appLifecycleMu.Lock()
	origRestart := appRestarter
	appRestarter = func() {
		select {
		case restartChan <- true:
		default:
		}
	}
	appLifecycleMu.Unlock()
	defer func() {
		appLifecycleMu.Lock()
		appRestarter = origRestart
		appLifecycleMu.Unlock()
	}()

	cleanupCfg := config.SetMockConfig(config.Config{
		App: config.AppConfig{APIToken: "tok"},
	})
	defer cleanupCfg()

	// GET returns 405.
	rrGet := httptest.NewRecorder()
	handleAPIRestart(rrGet, httptest.NewRequest("GET", "/api/v1/restart", nil))
	if rrGet.Code != http.StatusMethodNotAllowed {
		t.Errorf("GET restart got %d, want 405", rrGet.Code)
	}

	// Missing auth returns 401.
	rrUnauth := httptest.NewRecorder()
	handleAPIRestart(
		rrUnauth, httptest.NewRequest("POST", "/api/v1/restart", nil),
	)
	if rrUnauth.Code != http.StatusUnauthorized {
		t.Errorf("unauth restart got %d, want 401", rrUnauth.Code)
	}

	// Valid POST invokes restarter.
	req := httptest.NewRequest("POST", "/api/v1/restart", nil)
	req.Header.Set("Authorization", "Bearer tok")
	rr := httptest.NewRecorder()
	handleAPIRestart(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("POST restart got %d, want 200", rr.Code)
	}
	select {
	case <-restartChan:
	case <-time.After(2 * time.Second):
		t.Fatal("expected appRestarter to be invoked")
	}
}

func TestAPIValidationAndErrorPaths(t *testing.T) {
	dpms.ResetState()
	defer dpms.ResetState()

	_, cleanupDB, err := database.NewTestDB()
	if err != nil {
		t.Fatalf("NewTestDB failed: %v", err)
	}
	defer cleanupDB()

	cleanupCfg := config.SetMockConfig(config.Config{
		Shopping: config.ShoppingConfig{
			Store: []config.StoreConfig{
				{Name: "Costco"},
				{Name: "Closed", Disabled: true},
			},
		},
	})
	defer cleanupCfg()

	// Method checks.
	methodCases := []struct {
		handler http.HandlerFunc
		method  string
		path    string
	}{
		{handleAPIStatus, "POST", "/api/v1/status"},
		{handleAPIAway, "DELETE", "/api/v1/away"},
		{handleAPINextPhoto, "GET", "/api/v1/next-photo"},
		{handleAPIAlert, "GET", "/api/v1/alert"},
		{handleAPIShopping, "POST", "/api/v1/shopping"},
		{handleAPIShoppingItems, "GET", "/api/v1/shopping/items"},
		{handleAPIReminders, "POST", "/api/v1/reminders"},
		{handleAPIReminderAction, "GET", "/api/v1/reminders/1/acknowledge"},
	}
	for _, tc := range methodCases {
		rr := httptest.NewRecorder()
		tc.handler(rr, httptest.NewRequest(tc.method, tc.path, nil))
		if rr.Code != http.StatusMethodNotAllowed {
			t.Errorf("%s %s got %d, want 405", tc.method, tc.path, rr.Code)
		}
	}

	// Away invalid JSON and persistence error.
	rrAwayBad := httptest.NewRecorder()
	handleAPIAway(
		rrAwayBad,
		httptest.NewRequest("POST", "/api/v1/away", strings.NewReader(`{}`)),
	)
	if rrAwayBad.Code != http.StatusBadRequest {
		t.Errorf("away empty JSON got %d, want 400", rrAwayBad.Code)
	}

	origSetAway := database.SetAwayMode
	database.SetAwayMode = func(bool) error { return errors.New("db fail") }
	rrAwayErr := httptest.NewRecorder()
	handleAPIAway(
		rrAwayErr,
		httptest.NewRequest(
			"POST", "/api/v1/away", strings.NewReader(`{"away":true}`),
		),
	)
	database.SetAwayMode = origSetAway
	if rrAwayErr.Code != http.StatusInternalServerError || dpms.IsAway() {
		t.Errorf("expected 500 and away=false on DB error, got %d",
			rrAwayErr.Code)
	}

	// Alert invalid payloads.
	for _, body := range []string{`{bad`, `{}`} {
		rr := httptest.NewRecorder()
		handleAPIAlert(
			rr,
			httptest.NewRequest("POST", "/api/v1/alert", strings.NewReader(body)),
		)
		if rr.Code != http.StatusBadRequest {
			t.Errorf("alert(%s) got %d, want 400", body, rr.Code)
		}
	}

	// Shopping item validation & item update errors.
	id, err := database.AddShoppingItem(database.ShoppingItem{
		Name: "Bread", Quantity: 1, StoreID: 1,
	})
	if err != nil {
		t.Fatalf("AddShoppingItem failed: %v", err)
	}
	itemURL := "/api/v1/shopping/items/" + jsonNumberString(id)

	rrBadCreate := httptest.NewRecorder()
	handleAPIShoppingItems(
		rrBadCreate,
		httptest.NewRequest(
			"POST", "/api/v1/shopping/items",
			strings.NewReader(`{"store_id":1,"name":"   "}`),
		),
	)
	if rrBadCreate.Code != http.StatusBadRequest {
		t.Errorf("empty name create got %d, want 400", rrBadCreate.Code)
	}

	rrBadID := httptest.NewRecorder()
	handleAPIShoppingItemByID(
		rrBadID,
		httptest.NewRequest("DELETE", "/api/v1/shopping/items/abc", nil),
	)
	if rrBadID.Code != http.StatusBadRequest {
		t.Errorf("invalid item ID got %d, want 400", rrBadID.Code)
	}

	badPatches := []string{
		`{"name":"  "}`,
		`{"quantity":0}`,
		`{"store_id":2}`,
		`{bad-json`,
	}
	for _, body := range badPatches {
		rr := httptest.NewRecorder()
		handleAPIShoppingItemByID(
			rr,
			httptest.NewRequest("PATCH", itemURL, strings.NewReader(body)),
		)
		if rr.Code != http.StatusBadRequest {
			t.Errorf("patch(%s) got %d, want 400", body, rr.Code)
		}
	}

	rrMethodItem := httptest.NewRecorder()
	handleAPIShoppingItemByID(
		rrMethodItem, httptest.NewRequest("POST", itemURL, nil),
	)
	if rrMethodItem.Code != http.StatusMethodNotAllowed {
		t.Errorf("POST item got %d, want 405", rrMethodItem.Code)
	}

	// Reminders action errors.
	remErrCases := []struct {
		path string
		want int
	}{
		{"/api/v1/reminders/1/other", http.StatusNotFound},
		{"/api/v1/reminders/abc/acknowledge", http.StatusBadRequest},
		{"/api/v1/reminders/999/acknowledge", http.StatusNotFound},
	}
	for _, tc := range remErrCases {
		rr := httptest.NewRecorder()
		handleAPIReminderAction(
			rr, httptest.NewRequest("POST", tc.path, nil),
		)
		if rr.Code != tc.want {
			t.Errorf("POST %s got %d, want %d", tc.path, rr.Code, tc.want)
		}
	}
}
