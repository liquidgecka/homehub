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
	"bytes"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/liquidgecka/homehub/config"
	"github.com/liquidgecka/homehub/database"
	"github.com/liquidgecka/homehub/ledger"
	"github.com/liquidgecka/homehub/shopping"
)

func TestStart(t *testing.T) {
	t.Run("Server Disabled", func(t *testing.T) {
		var buf bytes.Buffer
		log.SetOutput(&buf)

		cfg := &config.AppConfig{WebServerPort: 0}
		Start(cfg)

		if !strings.Contains(buf.String(), "Web server is disabled") {
			t.Errorf("Expected log message about disabled server, but got: %s", buf.String())
		}
	})
}

func TestHandleIndex(t *testing.T) {
	// Create a dummy template file
	tempDir := t.TempDir()
	config.SetMockConfig(config.Config{
		App: config.AppConfig{
			WebTemplatesDirectory: tempDir,
		},
	})
	os.WriteFile(tempDir+"/index.html", []byte("<h1>HomeHub Web</h1>"), 0644)

	req, err := http.NewRequest("GET", "/", nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(handleIndex)
	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}

	expected := "<h1>HomeHub Web</h1>"
	if !strings.Contains(rr.Body.String(), expected) {
		t.Errorf("handler returned unexpected body: got %v want %s",
			rr.Body.String(), expected)
	}
}

func TestHandleShopping(t *testing.T) {
	// Create a dummy template file
	tempDir := t.TempDir()
	config.SetMockConfig(config.Config{
		App: config.AppConfig{
			WebTemplatesDirectory: tempDir,
		},
	})
	os.WriteFile(tempDir+"/shopping.html", []byte("{{range .Stores}}{{.Name}}{{end}}"), 0644)

	// Mock database
	database.GetShoppingItems = func() ([]database.ShoppingItem, error) {
		return []database.ShoppingItem{{Name: "Test Item"}}, nil
	}

	req, err := http.NewRequest("GET", "/shopping", nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(handleShopping)
	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}
}

func TestHandleAddShoppingItem(t *testing.T) {
	// Mock shopping.AddItem
	var addItemCalled bool
	shopping.AddItem = func(item database.ShoppingItem) error {
		addItemCalled = true
		return nil
	}

	form := url.Values{}
	form.Add("store_id", "1")
	form.Add("item_name", "Test Item")
	form.Add("quantity", "1")

	req, err := http.NewRequest("POST", "/shopping/add-item", strings.NewReader(form.Encode()))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(handleAddShoppingItem)
	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusFound {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusFound)
	}

	if !addItemCalled {
		t.Error("Expected shopping.AddItem to be called")
	}
}

func TestHandleAddStore(t *testing.T) {
	// Mock config.SaveConfig
	var saveConfigCalled bool
	config.SaveConfig = func(cfg *config.Config) error {
		saveConfigCalled = true
		return nil
	}

	form := url.Values{}
	form.Add("store_name", "New Store")

	req, err := http.NewRequest("POST", "/shopping/add-store", strings.NewReader(form.Encode()))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(handleAddStore)
	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusFound {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusFound)
	}

	if !saveConfigCalled {
		t.Error("Expected config.SaveConfig to be called")
	}
}

func TestHandleLedger(t *testing.T) {
	// Create a dummy template file
	tempDir := t.TempDir()
	config.SetMockConfig(config.Config{
		App: config.AppConfig{
			WebTemplatesDirectory: tempDir,
		},
	})
	os.WriteFile(tempDir+"/ledger.html", []byte("{{range .Accounts}}{{.Name}}{{end}}"), 0644)

	// Mock ledger
	ledger.GetAccounts = func() ([]ledger.Account, error) {
		return []ledger.Account{{Name: "Test Account"}}, nil
	}
	ledger.GetLedgerRecords = func(accountID int) ([]database.LedgerRecord, error) {
		return []database.LedgerRecord{}, nil
	}

	req, err := http.NewRequest("GET", "/ledger", nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(handleLedger)
	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}
}

func TestHandleAddLedger(t *testing.T) {
	// Mock ledger.AddAccount
	var addAccountCalled bool
	ledger.AddAccount = func(name string, initialBalance float64) error {
		addAccountCalled = true
		return nil
	}

	form := url.Values{}
	form.Add("ledger_name", "New Ledger")
	form.Add("initial_balance", "100")

	req, err := http.NewRequest("POST", "/ledger/add-ledger", strings.NewReader(form.Encode()))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(handleAddLedger)
	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusFound {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusFound)
	}

	if !addAccountCalled {
		t.Error("Expected ledger.AddAccount to be called")
	}
}

func TestHandleAddLedgerRecord(t *testing.T) {
	// Mock ledger
	ledger.GetAccounts = func() ([]ledger.Account, error) {
		return []ledger.Account{{ID: 1, Name: "Test Account", CurrentBalance: 100}}, nil
	}
	var addLedgerRecordCalled bool
	ledger.AddLedgerRecord = func(record database.LedgerRecord) (int, error) {
		addLedgerRecordCalled = true
		return 1, nil
	}
	var updateAccountCalled bool
	ledger.UpdateAccount = func(account ledger.Account) error {
		updateAccountCalled = true
		return nil
	}

	form := url.Values{}
	form.Add("account_id", "1")
	form.Add("description", "Test Record")
	form.Add("amount", "10")
	form.Add("type", "debit")

	req, err := http.NewRequest("POST", "/ledger/add-record", strings.NewReader(form.Encode()))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(handleAddLedgerRecord)
	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusFound {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusFound)
	}

	if !addLedgerRecordCalled {
		t.Error("Expected ledger.AddLedgerRecord to be called")
	}
	if !updateAccountCalled {
		t.Error("Expected ledger.UpdateAccount to be called")
	}
}

func TestHandleReminders(t *testing.T) {
	tempDir := t.TempDir()
	config.SetMockConfig(config.Config{
		App: config.AppConfig{
			WebTemplatesDirectory: tempDir,
		},
	})
	os.WriteFile(tempDir+"/reminders.html", []byte("{{range .Reminders}}{{.Title}}{{end}}"), 0644)

	database.GetRemindersDB = func() ([]database.Reminder, error) {
		return []database.Reminder{{Title: "Feed the dogs"}}, nil
	}

	req, err := http.NewRequest("GET", "/reminders", nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(handleReminders)
	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}
	if !strings.Contains(rr.Body.String(), "Feed the dogs") {
		t.Errorf("handler returned body missing reminder title: %s", rr.Body.String())
	}
}

func TestHandleAddReminderWeb(t *testing.T) {
	var addCalled bool
	database.AddReminderDB = func(r database.Reminder) (int, error) {
		addCalled = true
		return 1, nil
	}

	form := url.Values{}
	form.Add("title", "Feed the dogs")
	form.Add("time", "08:00")
	form.Add("days", "Everyday")

	req, err := http.NewRequest("POST", "/reminders/add", strings.NewReader(form.Encode()))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(handleAddReminderWeb)
	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusFound {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusFound)
	}
	if !addCalled {
		t.Error("Expected AddReminderDB to be called")
	}
}

func TestHandleEditReminderWeb(t *testing.T) {
	var updateCalled bool
	database.GetReminderByIDDB = func(id int) (database.Reminder, error) {
		return database.Reminder{ID: 1, Title: "Feed the dogs", Time: "08:00", Days: "Everyday"}, nil
	}
	database.UpdateReminderDB = func(r database.Reminder) error {
		updateCalled = true
		return nil
	}

	form := url.Values{}
	form.Add("title", "Feed dogs & cats")
	form.Add("time", "08:30")
	form.Add("days", "Everyday")

	req, err := http.NewRequest("POST", "/reminders/edit/1", strings.NewReader(form.Encode()))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(handleEditReminderWeb)
	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusFound {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusFound)
	}
	if !updateCalled {
		t.Error("Expected UpdateReminderDB to be called")
	}
}
