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
	"encoding/json"
	"fmt"
	"log"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/liquidgecka/homehub/config"
	"github.com/liquidgecka/homehub/database"
	"github.com/liquidgecka/homehub/ledger"
	"github.com/liquidgecka/homehub/photomanager"
	"github.com/liquidgecka/homehub/shopping"
)

func TestStart(t *testing.T) {
	t.Run("Server Disabled", func(t *testing.T) {
		var buf bytes.Buffer
		log.SetOutput(&buf)

		cfg := &config.AppConfig{WebServerPort: 0}
		Start(cfg)

		if !strings.Contains(buf.String(), "Web server is disabled") {
			t.Errorf(
				"Expected log message about disabled server, but got: %s",
				buf.String(),
			)
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
	// Set mock config pointing to real web templates directory
	config.SetMockConfig(config.Config{
		App: config.AppConfig{
			WebTemplatesDirectory: "web_templates",
		},
		Shopping: config.ShoppingConfig{
			Store: []config.StoreConfig{
				{Name: "Costco"},
				{Name: "Trader Joe's"},
			},
		},
	})

	// Mock database
	database.GetShoppingItems = func() ([]database.ShoppingItem, error) {
		return []database.ShoppingItem{
			{ID: 1, StoreID: 1, Name: "Eggs", Quantity: 2, Checked: false},
			{ID: 2, StoreID: 2, Name: "Bananas", Quantity: 1, Checked: true},
		}, nil
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

	body := rr.Body.String()
	hasTabs := strings.Contains(body, `data-tab="store-1"`) &&
		strings.Contains(body, `data-tab="store-2"`)
	if !hasTabs {
		t.Errorf("Expected store tabs in HTML body, got: %s", body)
	}
	if !strings.Contains(body, `data-tab="tab-new-store"`) {
		t.Errorf("Expected new store tab button in body, got: %s", body)
	}
	hasPanels := strings.Contains(body, `id="store-1"`) &&
		strings.Contains(body, `id="store-2"`)
	if !hasPanels {
		t.Errorf("Expected store panels in HTML body, got: %s", body)
	}

	// Test active store query param
	req2, err := http.NewRequest("GET", "/shopping?store_id=2", nil)
	if err != nil {
		t.Fatal(err)
	}
	rr2 := httptest.NewRecorder()
	handler.ServeHTTP(rr2, req2)
	if rr2.Code != http.StatusOK {
		t.Errorf(
			"handler returned wrong status code: got %v want %v",
			rr2.Code, http.StatusOK,
		)
	}
	body2 := rr2.Body.String()
	expectedActive := `class="shopping-tab-btn active" role="tab" ` +
		`data-tab="store-2"`
	if !strings.Contains(body2, expectedActive) &&
		!strings.Contains(body2, `class="shopping-tab-btn active"`) {
		t.Errorf("Expected store-2 tab to be active, body: %s", body2)
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

	req, err := http.NewRequest(
		"POST", "/shopping/add-item",
		strings.NewReader(form.Encode()),
	)
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

	req, err := http.NewRequest(
		"POST", "/shopping/add-store",
		strings.NewReader(form.Encode()),
	)
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
	// Set mock config pointing to real web templates directory
	config.SetMockConfig(config.Config{
		App: config.AppConfig{
			WebTemplatesDirectory: "web_templates",
		},
	})

	// Mock ledger with multiple accounts and records
	ledger.GetAccounts = func() ([]ledger.Account, error) {
		return []ledger.Account{
			{ID: 1, Name: "Checking", CurrentBalance: 500.50},
			{ID: 2, Name: "Savings", CurrentBalance: 1250.00},
		}, nil
	}
	ledger.GetLedgerRecords = func(
		accountID int,
	) ([]database.LedgerRecord, error) {
		if accountID == 1 {
			return []database.LedgerRecord{
				{
					ID:          10,
					AccountID:   1,
					Description: "Groceries",
					Amount:      50.0,
					Type:        database.Debit,
					Balance:     500.50,
					Timestamp:   time.Now(),
				},
			}, nil
		}
		return []database.LedgerRecord{}, nil
	}

	// Test default active account (first account)
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

	body := rr.Body.String()
	hasTabs := strings.Contains(body, `data-tab="account-1"`) &&
		strings.Contains(body, `data-tab="account-2"`)
	if !hasTabs {
		t.Errorf("Expected ledger tabs in HTML body, got: %s", body)
	}
	hasPanels := strings.Contains(body, `id="account-1"`) &&
		strings.Contains(body, `id="account-2"`)
	if !hasPanels {
		t.Errorf("Expected ledger panels in HTML body, got: %s", body)
	}

	// Test query param for account 2
	req2, err := http.NewRequest("GET", "/ledger?account_id=2", nil)
	if err != nil {
		t.Fatal(err)
	}
	rr2 := httptest.NewRecorder()
	handler.ServeHTTP(rr2, req2)
	if rr2.Code != http.StatusOK {
		t.Errorf(
			"handler returned wrong status code: got %v want %v",
			rr2.Code, http.StatusOK,
		)
	}
	body2 := rr2.Body.String()
	expectedActive := `class="ledger-tab-btn active" role="tab" ` +
		`data-tab="account-2"`
	if !strings.Contains(body2, expectedActive) &&
		!strings.Contains(body2, `class="ledger-tab-btn active"`) {
		t.Errorf("Expected account-2 tab to be active, body: %s", body2)
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

	req, err := http.NewRequest(
		"POST", "/ledger/add-ledger",
		strings.NewReader(form.Encode()),
	)
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
		return []ledger.Account{
			{ID: 1, Name: "Test Account", CurrentBalance: 100},
		}, nil
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

	req, err := http.NewRequest(
		"POST", "/ledger/add-record",
		strings.NewReader(form.Encode()),
	)
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
	remindersPath := filepath.Join(tempDir, "reminders.html")
	tmplContentReminders := []byte(
		"{{range .Reminders}}{{.Title}}{{end}}",
	)
	os.WriteFile(remindersPath, tmplContentReminders, 0644)

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
		t.Errorf(
			"handler returned wrong status code: got %v want %v",
			status, http.StatusOK,
		)
	}
	if !strings.Contains(rr.Body.String(), "Feed the dogs") {
		t.Errorf(
			"handler returned body missing reminder title: %s",
			rr.Body.String(),
		)
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

	req, err := http.NewRequest(
		"POST", "/reminders/add",
		strings.NewReader(form.Encode()),
	)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(handleAddReminderWeb)
	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusFound {
		t.Errorf(
			"handler returned wrong status code: got %v want %v",
			status, http.StatusFound,
		)
	}
	if !addCalled {
		t.Error("Expected AddReminderDB to be called")
	}
}

func TestHandleEditReminderWeb(t *testing.T) {
	var updateCalled bool
	database.GetReminderByIDDB = func(id int) (database.Reminder, error) {
		return database.Reminder{
			ID: 1, Title: "Feed the dogs", Time: "08:00", Days: "Everyday",
		}, nil
	}
	database.UpdateReminderDB = func(r database.Reminder) error {
		updateCalled = true
		return nil
	}

	form := url.Values{}
	form.Add("title", "Feed dogs & cats")
	form.Add("time", "08:30")
	form.Add("days", "Everyday")

	req, err := http.NewRequest(
		"POST", "/reminders/edit/1",
		strings.NewReader(form.Encode()),
	)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(handleEditReminderWeb)
	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusFound {
		t.Errorf(
			"handler returned wrong status code: got %v want %v",
			status, http.StatusFound,
		)
	}
	if !updateCalled {
		t.Error("Expected UpdateReminderDB to be called")
	}
}

func TestHandlePhotosPagination(t *testing.T) {
	tempDir := t.TempDir()
	photosDir := t.TempDir()
	config.SetMockConfig(config.Config{
		App: config.AppConfig{
			WebTemplatesDirectory: tempDir,
		},
		LocalPhotos: config.LocalPhotosConfig{
			Directory: photosDir,
		},
	})
	// Create dummy template file
	tmplContent := `Photos: {{len .Photos}}, CurrentPage: {{.CurrentPage}}, ` +
		`TotalPages: {{.TotalPages}}, PerPage: {{.PerPage}}`
	os.WriteFile(tempDir+"/photos.html", []byte(tmplContent), 0644)

	// Create 5 dummy image files
	for i := 1; i <= 5; i++ {
		imgName := fmt.Sprintf("/img%d.jpg", i)
		os.WriteFile(photosDir+imgName, []byte("dummy"), 0644)
	}

	req, err := http.NewRequest("GET", "/photos?page=2&per_page=2", nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(handlePhotos)
	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf(
			"handler returned wrong status code: got %v want %v",
			status, http.StatusOK,
		)
	}

	expected := "Photos: 2, CurrentPage: 2, TotalPages: 3, PerPage: 2"
	if !strings.Contains(rr.Body.String(), expected) {
		t.Errorf(
			"handler returned unexpected body: got %q want %q",
			rr.Body.String(), expected,
		)
	}
}

func TestHandlePhotoUpload(t *testing.T) {
	tempDir := t.TempDir()
	photosDir := t.TempDir()
	config.SetMockConfig(config.Config{
		App: config.AppConfig{
			WebTemplatesDirectory: tempDir,
		},
		LocalPhotos: config.LocalPhotosConfig{
			Directory: photosDir,
		},
	})
	uploadTmplPath := filepath.Join(tempDir, "photos_upload.html")
	os.WriteFile(uploadTmplPath, []byte("<h1>Upload Photo</h1>"), 0644)

	t.Run("GET Request", func(t *testing.T) {
		req, err := http.NewRequest("GET", "/photos/upload", nil)
		if err != nil {
			t.Fatal(err)
		}
		rr := httptest.NewRecorder()
		handler := http.HandlerFunc(handlePhotoUpload)
		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("Expected status OK, got %v", rr.Code)
		}
		if !strings.Contains(rr.Body.String(), "<h1>Upload Photo</h1>") {
			t.Errorf(
				"Expected body to contain template content, got: %s",
				rr.Body.String(),
			)
		}
	})

	t.Run("POST Single Photo", func(t *testing.T) {
		origAddPhoto := photomanager.AddPhoto
		defer func() { photomanager.AddPhoto = origAddPhoto }()

		var savedFiles []string
		photomanager.AddPhoto = func(
			filename string, data []byte, localPhotosDir string,
		) error {
			savedFiles = append(savedFiles, filename)
			return nil
		}

		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)
		part, err := writer.CreateFormFile("photo_file", "test1.jpg")
		if err != nil {
			t.Fatal(err)
		}
		part.Write([]byte("fake image data 1"))
		writer.Close()

		req, err := http.NewRequest("POST", "/photos/upload", body)
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Content-Type", writer.FormDataContentType())

		rr := httptest.NewRecorder()
		handler := http.HandlerFunc(handlePhotoUpload)
		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusFound {
			t.Errorf("Expected status 302 redirect, got %v", rr.Code)
		}
		if len(savedFiles) != 1 || savedFiles[0] != "test1.jpg" {
			t.Errorf("Expected test1.jpg to be saved, got %v", savedFiles)
		}
	})

	t.Run("POST Multiple Photos", func(t *testing.T) {
		origAddPhoto := photomanager.AddPhoto
		defer func() { photomanager.AddPhoto = origAddPhoto }()

		var savedFiles []string
		photomanager.AddPhoto = func(
			filename string, data []byte, localPhotosDir string,
		) error {
			savedFiles = append(savedFiles, filename)
			return nil
		}

		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)
		part1, err := writer.CreateFormFile("photo_files", "vacation1.jpg")
		if err != nil {
			t.Fatal(err)
		}
		part1.Write([]byte("fake image 1"))

		part2, err := writer.CreateFormFile("photo_files", "vacation2.png")
		if err != nil {
			t.Fatal(err)
		}
		part2.Write([]byte("fake image 2"))

		part3, err := writer.CreateFormFile("photo_files", "vacation3.jpeg")
		if err != nil {
			t.Fatal(err)
		}
		part3.Write([]byte("fake image 3"))

		writer.Close()

		req, err := http.NewRequest("POST", "/photos/upload", body)
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Content-Type", writer.FormDataContentType())

		rr := httptest.NewRecorder()
		handler := http.HandlerFunc(handlePhotoUpload)
		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusFound {
			t.Errorf("Expected status 302 redirect, got %v", rr.Code)
		}
		if len(savedFiles) != 3 {
			t.Fatalf("Expected 3 files saved, got %d: %v", len(savedFiles), savedFiles)
		}
		expectedSaved := []string{
			"vacation1.jpg", "vacation2.png", "vacation3.jpeg",
		}
		if !reflect.DeepEqual(savedFiles, expectedSaved) {
			t.Errorf("Unexpected saved files: %v", savedFiles)
		}
	})

	t.Run("POST Empty Request", func(t *testing.T) {
		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)
		writer.Close()

		req, err := http.NewRequest("POST", "/photos/upload", body)
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Content-Type", writer.FormDataContentType())

		rr := httptest.NewRecorder()
		handler := http.HandlerFunc(handlePhotoUpload)
		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Errorf("Expected status 400 Bad Request, got %v", rr.Code)
		}
	})
}

func TestHandlePhotosSorting(t *testing.T) {
	tempDir := t.TempDir()
	photosDir := t.TempDir()

	config.SetMockConfig(config.Config{
		App: config.AppConfig{
			WebTemplatesDirectory: tempDir,
		},
		LocalPhotos: config.LocalPhotosConfig{
			Directory: photosDir,
		},
	})

	tmplContent := `{{.CurrentSort}}:{{range .Photos}}{{.Filename}},{{end}}`
	os.WriteFile(filepath.Join(tempDir, "photos.html"), []byte(tmplContent), 0644)

	// Create test image files with staggered mod times
	f1 := filepath.Join(photosDir, "c_photo.jpg")
	f2 := filepath.Join(photosDir, "a_photo.jpg")
	f3 := filepath.Join(photosDir, "b_photo.jpg")

	os.WriteFile(f1, []byte("data1"), 0644)
	time.Sleep(10 * time.Millisecond)
	os.WriteFile(f2, []byte("data2"), 0644)
	time.Sleep(10 * time.Millisecond)
	os.WriteFile(f3, []byte("data3"), 0644)

	// 1. Sort by name ascending (default for name)
	t.Run("Sort by Name Ascending", func(t *testing.T) {
		req, _ := http.NewRequest(
			"GET", "/photos?sort=name&order=asc", nil,
		)
		rr := httptest.NewRecorder()
		handlePhotos(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("Expected 200 OK, got %d", rr.Code)
		}
		expected := "name:a_photo.jpg,b_photo.jpg,c_photo.jpg,"
		if rr.Body.String() != expected {
			t.Errorf("Expected %q, got %q", expected, rr.Body.String())
		}
	})

	// 2. Sort by name descending
	t.Run("Sort by Name Descending", func(t *testing.T) {
		req, _ := http.NewRequest(
			"GET", "/photos?sort=name&order=desc", nil,
		)
		rr := httptest.NewRecorder()
		handlePhotos(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("Expected 200 OK, got %d", rr.Code)
		}
		expected := "name:c_photo.jpg,b_photo.jpg,a_photo.jpg,"
		if rr.Body.String() != expected {
			t.Errorf("Expected %q, got %q", expected, rr.Body.String())
		}
	})

	// 3. Sort by upload date descending (default)
	t.Run("Sort by Upload Date Descending", func(t *testing.T) {
		req, _ := http.NewRequest(
			"GET", "/photos?sort=date_upload&order=desc", nil,
		)
		rr := httptest.NewRecorder()
		handlePhotos(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("Expected 200 OK, got %d", rr.Code)
		}
		expected := "date_upload:b_photo.jpg,a_photo.jpg,c_photo.jpg,"
		if rr.Body.String() != expected {
			t.Errorf("Expected %q, got %q", expected, rr.Body.String())
		}
	})

	// 4. Sort by upload date ascending (oldest first)
	t.Run("Sort by Upload Date Ascending", func(t *testing.T) {
		req, _ := http.NewRequest(
			"GET", "/photos?sort=date_upload&order=asc", nil,
		)
		rr := httptest.NewRecorder()
		handlePhotos(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("Expected 200 OK, got %d", rr.Code)
		}
		expected := "date_upload:c_photo.jpg,a_photo.jpg,b_photo.jpg,"
		if rr.Body.String() != expected {
			t.Errorf("Expected %q, got %q", expected, rr.Body.String())
		}
	})

	// 5. Sort by metadata date (default)
	t.Run("Sort by Metadata Date Default", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/photos", nil)
		rr := httptest.NewRecorder()
		handlePhotos(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("Expected 200 OK, got %d", rr.Code)
		}
		if !strings.HasPrefix(rr.Body.String(), "date_meta:") {
			t.Errorf("Expected date_meta default, got %q", rr.Body.String())
		}
	})
}

func TestHandlePhotoUploadDeduplication(t *testing.T) {
	tempDir := t.TempDir()
	photosDir := t.TempDir()

	config.SetMockConfig(config.Config{
		App: config.AppConfig{
			WebTemplatesDirectory: tempDir,
		},
		LocalPhotos: config.LocalPhotosConfig{
			Directory: photosDir,
		},
	})
	os.WriteFile(
		filepath.Join(tempDir, "photos_upload.html"),
		[]byte("<h1>Upload</h1>"), 0644,
	)

	origAddPhoto := photomanager.AddPhoto
	defer func() { photomanager.AddPhoto = origAddPhoto }()

	callCount := 0
	photomanager.AddPhoto = func(
		filename string, data []byte, localPhotosDir string,
	) error {
		callCount++
		if filename == "duplicate.jpg" {
			return photomanager.ErrDuplicatePhoto
		}
		return nil
	}

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	p1, _ := writer.CreateFormFile("photo_files", "duplicate.jpg")
	p1.Write([]byte("dup content"))
	p2, _ := writer.CreateFormFile("photo_files", "fresh.jpg")
	p2.Write([]byte("fresh content"))
	writer.Close()

	req, _ := http.NewRequest("POST", "/photos/upload", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	rr := httptest.NewRecorder()
	handlePhotoUpload(rr, req)

	if rr.Code != http.StatusFound {
		t.Errorf(
			"Expected 302 redirect for batch with duplicate, got %d",
			rr.Code,
		)
	}
	if callCount != 2 {
		t.Errorf("Expected AddPhoto to be called twice, got %d", callCount)
	}
}

func TestHandleAppRestartAndQuit(t *testing.T) {
	t.Run("POST /app/restart", func(t *testing.T) {
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

		req, _ := http.NewRequest("POST", "/app/restart", nil)
		rr := httptest.NewRecorder()
		handleAppRestart(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("Expected 200 OK, got %d", rr.Code)
		}
		var resp map[string]string
		if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
			t.Fatalf("Failed to decode json response: %v", err)
		}
		if resp["status"] != "restarting" {
			t.Errorf("Expected status 'restarting', got %q", resp["status"])
		}

		select {
		case <-restartChan:
			// Success
		case <-time.After(2 * time.Second):
			t.Errorf("Expected appRestarter to be invoked")
		}
	})

	t.Run("POST /app/quit", func(t *testing.T) {
		quitChan := make(chan bool, 1)

		appLifecycleMu.Lock()
		origQuit := appQuitter
		appQuitter = func() {
			select {
			case quitChan <- true:
			default:
			}
		}
		appLifecycleMu.Unlock()

		defer func() {
			appLifecycleMu.Lock()
			appQuitter = origQuit
			appLifecycleMu.Unlock()
		}()

		req, _ := http.NewRequest("POST", "/app/quit", nil)
		rr := httptest.NewRecorder()
		handleAppQuit(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("Expected 200 OK, got %d", rr.Code)
		}
		var resp map[string]string
		if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
			t.Fatalf("Failed to decode json response: %v", err)
		}
		if resp["status"] != "quitting" {
			t.Errorf("Expected status 'quitting', got %q", resp["status"])
		}

		select {
		case <-quitChan:
			// Success
		case <-time.After(2 * time.Second):
			t.Errorf("Expected appQuitter to be invoked")
		}
	})

	t.Run("GET /app/restart Method Not Allowed", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/app/restart", nil)
		rr := httptest.NewRecorder()
		handleAppRestart(rr, req)

		if rr.Code != http.StatusMethodNotAllowed {
			t.Errorf("Expected 405 Method Not Allowed, got %d", rr.Code)
		}
	})

	t.Run("GET /app/quit Method Not Allowed", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/app/quit", nil)
		rr := httptest.NewRecorder()
		handleAppQuit(rr, req)

		if rr.Code != http.StatusMethodNotAllowed {
			t.Errorf("Expected 405 Method Not Allowed, got %d", rr.Code)
		}
	})
}

func TestGetStorageInfoAndFormatting(t *testing.T) {
	tempDir := t.TempDir()

	// Create test photo file
	photoFile := filepath.Join(tempDir, "sample.jpg")
	os.WriteFile(photoFile, []byte("some-image-data"), 0644)

	info := GetStorageInfo(tempDir)
	if info.PhotoCount != 1 {
		t.Errorf("Expected PhotoCount 1, got %d", info.PhotoCount)
	}

	// Test formatStorageBytes
	if formatted := formatStorageBytes(500); formatted != "500 B" {
		t.Errorf("Expected '500 B', got %q", formatted)
	}
	if formatted := formatStorageBytes(1024); formatted != "1.0 KB" {
		t.Errorf("Expected '1.0 KB', got %q", formatted)
	}
	if formatted := formatStorageBytes(1024 * 1024 * 5); formatted != "5.0 MB" {
		t.Errorf("Expected '5.0 MB', got %q", formatted)
	}
}

func TestHandleTogglePhotoActionsWithOrder(t *testing.T) {
	_, cleanupDB, err := database.NewTestDB()
	if err != nil {
		t.Fatalf("NewTestDB failed: %v", err)
	}
	defer cleanupDB()

	tempDir := t.TempDir()
	config.SetMockConfig(config.Config{
		LocalPhotos: config.LocalPhotosConfig{
			Directory: tempDir,
		},
	})

	photoPath := filepath.Join(tempDir, "photo.jpg")
	os.WriteFile(photoPath, []byte("data"), 0644)

	t.Run("Toggle Favorite with Sort and Order", func(t *testing.T) {
		form := url.Values{}
		form.Set("page", "2")
		form.Set("per_page", "12")
		form.Set("sort", "name")
		form.Set("order", "asc")

		req, _ := http.NewRequest(
			"POST", "/photos/toggle-favorite/photo.jpg",
			strings.NewReader(form.Encode()),
		)
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rr := httptest.NewRecorder()

		handleTogglePhotoFavorite(rr, req)
		if rr.Code != http.StatusFound {
			t.Fatalf("Expected 302 redirect, got %d", rr.Code)
		}
		expectedLoc := "/photos?page=2&per_page=12&sort=name&order=asc"
		if loc := rr.Header().Get("Location"); loc != expectedLoc {
			t.Errorf("Expected Location %q, got %q", expectedLoc, loc)
		}
	})

	t.Run("Toggle Hidden with Sort and Order", func(t *testing.T) {
		form := url.Values{}
		form.Set("page", "3")
		form.Set("per_page", "24")
		form.Set("sort", "date_upload")
		form.Set("order", "desc")

		req, _ := http.NewRequest(
			"POST", "/photos/toggle-hidden/photo.jpg",
			strings.NewReader(form.Encode()),
		)
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rr := httptest.NewRecorder()

		handleTogglePhotoHidden(rr, req)
		if rr.Code != http.StatusFound {
			t.Fatalf("Expected 302 redirect, got %d", rr.Code)
		}
		expectedLoc := "/photos?page=3&per_page=24&sort=date_upload&order=desc"
		if loc := rr.Header().Get("Location"); loc != expectedLoc {
			t.Errorf("Expected Location %q, got %q", expectedLoc, loc)
		}
	})

	t.Run("Delete Photo with Sort and Order", func(t *testing.T) {
		form := url.Values{}
		form.Set("page", "1")
		form.Set("per_page", "24")
		form.Set("sort", "name")
		form.Set("order", "desc")

		req, _ := http.NewRequest(
			"POST", "/photos/delete/photo.jpg",
			strings.NewReader(form.Encode()),
		)
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rr := httptest.NewRecorder()

		handleDeletePhoto(rr, req)
		if rr.Code != http.StatusFound {
			t.Fatalf("Expected 302 redirect, got %d", rr.Code)
		}
		expectedLoc := "/photos?page=1&per_page=24&sort=name&order=desc"
		if loc := rr.Header().Get("Location"); loc != expectedLoc {
			t.Errorf("Expected Location %q, got %q", expectedLoc, loc)
		}
	})
}

func TestHandleBackups(t *testing.T) {
	tempHome := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tempHome)
	defer os.Setenv("HOME", origHome)

	backupDir := filepath.Join(tempHome, "backups")
	os.MkdirAll(backupDir, 0700)

	config.SetMockConfig(config.Config{
		App: config.AppConfig{
			WebTemplatesDirectory: "web_templates",
		},
		Database: config.DatabaseConfig{
			BackupDirectory: backupDir,
		},
	})

	req, err := http.NewRequest("GET", "/backups", nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	handleBackups(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK, got %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "Database Backups &amp; Restore") {
		t.Errorf(
			"Expected backups page header, got body: %s",
			rr.Body.String(),
		)
	}
}

func TestHandleCreateAndDownloadAndRestoreBackup(t *testing.T) {
	tempHome := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tempHome)
	defer os.Setenv("HOME", origHome)

	// Create dummy homehub.db
	dbDir := filepath.Join(tempHome, ".local", "homehub")
	os.MkdirAll(dbDir, 0700)
	sqliteHeader := []byte("SQLite format 3\x00dummy-data")
	os.WriteFile(filepath.Join(dbDir, "homehub.db"), sqliteHeader, 0600)

	backupDir := filepath.Join(tempHome, "backups")
	config.SetMockConfig(config.Config{
		App: config.AppConfig{
			WebTemplatesDirectory: "web_templates",
		},
		Database: config.DatabaseConfig{
			BackupDirectory: backupDir,
		},
	})

	// Test handleCreateBackup
	reqCreate, _ := http.NewRequest("POST", "/backups/create", nil)
	rrCreate := httptest.NewRecorder()
	handleCreateBackup(rrCreate, reqCreate)

	if rrCreate.Code != http.StatusSeeOther {
		t.Fatalf("Expected 303 redirect, got %d", rrCreate.Code)
	}

	backups, err := database.ListBackups(backupDir)
	if err != nil || len(backups) == 0 {
		t.Fatalf("Expected backup created, found: %v", backups)
	}
	createdFilename := backups[0].Filename

	// Test handleDownloadBackup
	reqDown, _ := http.NewRequest(
		"GET", "/backups/download?filename="+createdFilename, nil,
	)
	rrDown := httptest.NewRecorder()
	handleDownloadBackup(rrDown, reqDown)

	if rrDown.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK on download, got %d", rrDown.Code)
	}
	if rrDown.Header().Get("Content-Type") != "application/zip" {
		t.Errorf(
			"Expected application/zip, got %s",
			rrDown.Header().Get("Content-Type"),
		)
	}

	// Test handleRestoreBackup
	form := url.Values{}
	form.Set("filename", createdFilename)
	reqRestore, _ := http.NewRequest(
		"POST", "/backups/restore", strings.NewReader(form.Encode()),
	)
	reqRestore.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rrRestore := httptest.NewRecorder()
	handleRestoreBackup(rrRestore, reqRestore)

	if rrRestore.Code != http.StatusSeeOther {
		t.Fatalf(
			"Expected 303 redirect on restore, got %d",
			rrRestore.Code,
		)
	}

	// Test handleDeleteBackup
	reqDel, _ := http.NewRequest(
		"POST", "/backups/delete", strings.NewReader(form.Encode()),
	)
	reqDel.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rrDel := httptest.NewRecorder()
	handleDeleteBackup(rrDel, reqDel)

	if rrDel.Code != http.StatusSeeOther {
		t.Fatalf("Expected 303 redirect on delete, got %d", rrDel.Code)
	}

	backupsAfterDel, _ := database.ListBackups(backupDir)
	if len(backupsAfterDel) != 0 {
		t.Fatalf(
			"Expected 0 backups after delete, got %d",
			len(backupsAfterDel),
		)
	}
}

func TestHandleUploadBackup(t *testing.T) {
	tempHome := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tempHome)
	defer os.Setenv("HOME", origHome)

	backupDir := filepath.Join(tempHome, "backups")
	config.SetMockConfig(config.Config{
		Database: config.DatabaseConfig{
			BackupDirectory: backupDir,
		},
	})

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("backup_file", "manual_backup.zip")
	if err != nil {
		t.Fatal(err)
	}
	part.Write([]byte("PK\x03\x04dummyzipcontent"))
	writer.Close()

	req, _ := http.NewRequest("POST", "/backups/upload", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rr := httptest.NewRecorder()

	handleUploadBackup(rr, req)
	if rr.Code != http.StatusSeeOther {
		t.Fatalf("Expected 303 redirect on upload, got %d", rr.Code)
	}

	dest := filepath.Join(backupDir, "manual_backup.zip")
	if _, err := os.Stat(dest); err != nil {
		t.Fatalf("Expected uploaded file in backup directory: %v", err)
	}
}
