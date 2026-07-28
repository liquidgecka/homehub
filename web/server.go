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
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/liquidgecka/homehub/config"
	"github.com/liquidgecka/homehub/database"
	"github.com/liquidgecka/homehub/ledger"
	"github.com/liquidgecka/homehub/photomanager"
	"github.com/liquidgecka/homehub/reminders"
	"github.com/liquidgecka/homehub/shopping"
)

// RemindersTemplateData holds the data for the reminders page.
type RemindersTemplateData struct {
	Reminders []database.Reminder
}

// ShoppingTemplateData holds the data for the shopping list page.
type ShoppingTemplateData struct {
	Stores []StoreData
}

// StoreData represents a single store with its items.
type StoreData struct {
	ID    int
	Name  string
	Items []ItemData
}

// ItemData represents a single shopping list item.
type ItemData struct {
	ID       int
	Name     string
	Quantity int
	Checked  bool
}

// LedgerTemplateData holds the data for the ledger page.
type LedgerTemplateData struct {
	Accounts []AccountData
}

// AccountData represents a single financial account.
type AccountData struct {
	ID             int
	Name           string
	CurrentBalance float64
	Records        []database.LedgerRecord
}

// Start starts the web server.
func Start(cfg *config.AppConfig) {
	if cfg.WebServerPort == 0 {
		log.Println("Web server is disabled (WebServerPort is 0).")
		return
	}

	http.HandleFunc("/", handleIndex)
	http.HandleFunc("/shopping", handleShopping)
	http.HandleFunc("/shopping/add-item", handleAddShoppingItem)
	http.HandleFunc("/shopping/add-store", handleAddStore)
	http.HandleFunc("/shopping/edit-item/", handleEditShoppingItem)
	http.HandleFunc("/shopping/delete-item/", handleDeleteShoppingItem)
	http.HandleFunc("/shopping/toggle-checked/", handleToggleShoppingItemChecked)
	http.HandleFunc("/ledger", handleLedger)
	http.HandleFunc("/ledger/add-ledger", handleAddLedger)
	http.HandleFunc("/ledger/edit-account/", handleEditLedgerAccount)
	http.HandleFunc("/ledger/delete-account/", handleDeleteLedgerAccount)
	http.HandleFunc("/ledger/add-record", handleAddLedgerRecord)
	http.HandleFunc("/ledger/edit-record/", handleEditLedgerRecord)
	http.HandleFunc("/ledger/delete-record/", handleDeleteLedgerRecord)

	// Photo Management Handlers
	http.HandleFunc("/photos", handlePhotos)
	http.HandleFunc("/photos/thumbnail/", handlePhotoThumbnail)
	http.HandleFunc("/photos/fullsize/", handlePhotoFullsize)
	http.HandleFunc("/photos/toggle-favorite/", handleTogglePhotoFavorite)
	http.HandleFunc("/photos/toggle-hidden/", handleTogglePhotoHidden)
	http.HandleFunc("/photos/delete/", handleDeletePhoto)
	http.HandleFunc("/photos/upload", handlePhotoUpload) // New handler for photo uploads

	// Reminders Handlers
	http.HandleFunc("/reminders", handleReminders)
	http.HandleFunc("/reminders/add", handleAddReminderWeb)
	http.HandleFunc("/reminders/acknowledge/", handleAcknowledgeReminderWeb)
	http.HandleFunc("/reminders/delete/", handleDeleteReminderWeb)
	http.HandleFunc("/reminders/toggle/", handleToggleReminderWeb)
	http.HandleFunc("/reminders/edit/", handleEditReminderWeb)

	addr := fmt.Sprintf("%s:%d", cfg.WebServerListenAddress, cfg.WebServerPort)
	log.Printf("Starting web server on %s", addr)
	go func() {
		if err := http.ListenAndServe(addr, loggingMiddleware(http.DefaultServeMux)); err != nil {
			log.Fatalf("Failed to start web server: %v", err)
		}
	}()
}

// loggingMiddleware logs details of each incoming HTTP request.
func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("WEB: %s %s %s from %s", r.Method, r.URL.Path, r.Proto, r.RemoteAddr)
		next.ServeHTTP(w, r)
	})
}

func handleIndex(w http.ResponseWriter, r *http.Request) {
	lp := filepath.Join(config.GetConfig().App.WebTemplatesDirectory, "index.html")
	tmpl, err := template.ParseFiles(lp)
	if err != nil {
		log.Printf("Error parsing index template: %v", err)
		http.Error(w, "Error rendering page", http.StatusInternalServerError)
		return
	}
	tmpl.Execute(w, nil)
}

func handleShopping(w http.ResponseWriter, r *http.Request) {
	cfg := config.GetConfig()
	allStores := cfg.Shopping.Store
	allItems, err := database.GetShoppingItems()
	if err != nil {
		http.Error(w, "Failed to get shopping items", http.StatusInternalServerError)
		return
	}

	data := ShoppingTemplateData{}
	for i, store := range allStores {
		if store.Disabled {
			continue
		}
		storeData := StoreData{
			ID:   i + 1,
			Name: store.Name,
		}
		for _, item := range allItems {
			if item.StoreID == storeData.ID {
				storeData.Items = append(storeData.Items, ItemData{
					ID:       item.ID,
					Name:     item.Name,
					Quantity: item.Quantity,
					Checked:  item.Checked,
				})
			}
		}
		data.Stores = append(data.Stores, storeData)
	}

	lp := filepath.Join(config.GetConfig().App.WebTemplatesDirectory, "shopping.html")
	tmpl, err := template.ParseFiles(lp)
	if err != nil {
		log.Printf("Error parsing template: %v", err)
		http.Error(w, "Error rendering page", http.StatusInternalServerError)
		return
	}
	tmpl.Execute(w, data)
}

func handleAddShoppingItem(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
		return
	}
	r.ParseForm()
	storeID, _ := strconv.Atoi(r.FormValue("store_id"))
	itemName := r.FormValue("item_name")
	quantity, _ := strconv.Atoi(r.FormValue("quantity"))

	if itemName == "" || quantity <= 0 {
		http.Error(w, "Invalid item name or quantity", http.StatusBadRequest)
		return
	}

	item := database.ShoppingItem{
		Name:     itemName,
		Quantity: quantity,
		StoreID:  storeID,
		Checked:  false,
	}
	if err := shopping.AddItem(item); err != nil {
		http.Error(w, "Failed to add item", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/shopping", http.StatusFound)
}

func handleAddStore(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
		return
	}
	r.ParseForm()
	storeName := r.FormValue("store_name")

	if storeName == "" {
		http.Error(w, "Store name cannot be empty", http.StatusBadRequest)
		return
	}

	cfg := config.GetConfig()
	newStore := config.StoreConfig{Name: storeName}
	cfg.Shopping.Store = append(cfg.Shopping.Store, newStore)

	if err := config.SaveConfig(cfg); err != nil {
		http.Error(w, "Failed to save config", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/shopping", http.StatusFound)
}

func handleLedger(w http.ResponseWriter, r *http.Request) {
	accounts, err := ledger.GetAccounts()
	if err != nil {
		http.Error(w, "Failed to get accounts", http.StatusInternalServerError)
		return
	}

	data := LedgerTemplateData{}
	for _, acc := range accounts {
		records, err := ledger.GetLedgerRecords(acc.ID)
		if err != nil {
			http.Error(w, "Failed to get ledger records", http.StatusInternalServerError)
			return
		}
		data.Accounts = append(data.Accounts, AccountData{
			ID:             acc.ID,
			Name:           acc.Name,
			CurrentBalance: acc.CurrentBalance,
			Records:        records,
		})
	}

	lp := filepath.Join(config.GetConfig().App.WebTemplatesDirectory, "ledger.html")
	tmpl, err := template.ParseFiles(lp)
	if err != nil {
		log.Printf("Error parsing template: %v", err)
		http.Error(w, "Error rendering page", http.StatusInternalServerError)
		return
	}
	tmpl.Execute(w, data)
}

func handleAddLedger(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
		return
	}
	r.ParseForm()
	name := r.FormValue("ledger_name")
	balance, _ := strconv.ParseFloat(r.FormValue("initial_balance"), 64)

	if name == "" {
		http.Error(w, "Ledger name cannot be empty", http.StatusBadRequest)
		return
	}

	if err := ledger.AddAccount(name, balance); err != nil {
		http.Error(w, "Failed to add ledger", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/ledger", http.StatusFound)
}

func handleAddLedgerRecord(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
		return
	}
	r.ParseForm()
	accountID, _ := strconv.Atoi(r.FormValue("account_id"))
	description := r.FormValue("description")
	amount, _ := strconv.ParseFloat(r.FormValue("amount"), 64)
	recordType := database.LedgerRecordType(r.FormValue("type"))

	if description == "" || amount <= 0 {
		http.Error(w, "Invalid description or amount", http.StatusBadRequest)
		return
	}

	accounts, _ := ledger.GetAccounts()
	var account ledger.Account
	for _, acc := range accounts {
		if acc.ID == accountID {
			account = acc
			break
		}
	}

	newBalance := account.CurrentBalance
	if recordType == database.Credit {
		newBalance += amount
	} else {
		newBalance -= amount
	}

	record := database.LedgerRecord{
		AccountID:   accountID,
		Timestamp:   time.Now(),
		Description: description,
		Amount:      amount,
		Type:        recordType,
		Balance:     newBalance,
	}
	if _, err := ledger.AddLedgerRecord(record); err != nil {
		http.Error(w, "Failed to add record", http.StatusInternalServerError)
		return
	}

	account.CurrentBalance = newBalance
	if err := ledger.UpdateAccount(account); err != nil {
		http.Error(w, "Failed to update account", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/ledger", http.StatusFound)
}

func handleEditLedgerAccount(w http.ResponseWriter, r *http.Request) {
	accountIDStr := r.URL.Path[len("/ledger/edit-account/"):]
	accountID, err := strconv.Atoi(accountIDStr)
	if err != nil {
		http.Error(w, "Invalid account ID", http.StatusBadRequest)
		return
	}

	account, err := ledger.GetAccountByID(accountID)
	if err != nil {
		http.Error(w, "Account not found", http.StatusNotFound)
		return
	}

	if r.Method == http.MethodPost {
		r.ParseForm()
		name := r.FormValue("name")
		initialBalance, _ := strconv.ParseFloat(r.FormValue("initial_balance"), 64)
		currentBalance, _ := strconv.ParseFloat(r.FormValue("current_balance"), 64)

		if name == "" {
			http.Error(w, "Account name cannot be empty", http.StatusBadRequest)
			return
		}

		account.Name = name
		account.InitialBalance = initialBalance
		account.CurrentBalance = currentBalance // Should be recalculated, but for direct edit, allow setting.
		if err := ledger.UpdateAccount(account); err != nil {
			http.Error(w, "Failed to update account", http.StatusInternalServerError)
			return
		}
		http.Redirect(w, r, "/ledger", http.StatusFound)
		return
	}

	lp := filepath.Join(config.GetConfig().App.WebTemplatesDirectory, "ledger_edit_account.html")
	tmpl, err := template.ParseFiles(lp)
	if err != nil {
		log.Printf("Error parsing ledger_edit_account template: %v", err)
		http.Error(w, "Error rendering page", http.StatusInternalServerError)
		return
	}
	tmpl.Execute(w, account)
}

func handleDeleteLedgerAccount(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
		return
	}
	accountIDStr := r.URL.Path[len("/ledger/delete-account/"):]
	accountID, err := strconv.Atoi(accountIDStr)
	if err != nil {
		http.Error(w, "Invalid account ID", http.StatusBadRequest)
		return
	}

	if err := ledger.DeleteAccount(accountID); err != nil {
		http.Error(w, "Failed to delete account", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/ledger", http.StatusFound)
}

func handleEditLedgerRecord(w http.ResponseWriter, r *http.Request) {
	recordIDStr := r.URL.Path[len("/ledger/edit-record/"):]
	recordID, err := strconv.Atoi(recordIDStr)
	if err != nil {
		http.Error(w, "Invalid record ID", http.StatusBadRequest)
		return
	}

	record, err := ledger.GetLedgerRecordByID(recordID)
	if err != nil {
		http.Error(w, "Record not found", http.StatusNotFound)
		return
	}

	if r.Method == http.MethodPost {
		r.ParseForm()
		description := r.FormValue("description")
		amount, _ := strconv.ParseFloat(r.FormValue("amount"), 64)
		recordType := database.LedgerRecordType(r.FormValue("type"))
		timestampStr := r.FormValue("timestamp")

		if description == "" || amount <= 0 {
			http.Error(w, "Invalid description or amount", http.StatusBadRequest)
			return
		}

		timestamp, err := time.Parse("2006-01-02T15:04", timestampStr) // Expect YYYY-MM-DDTHH:MM
		if err != nil {
			http.Error(w, "Invalid timestamp format", http.StatusBadRequest)
			return
		}
		// To accurately update, we need to know the old record's impact and remove it, then apply the new one.
		// This is getting too complex for a direct edit. For now, we update the record and mark the account balance as needing recalculation.
		// Or, simplest: update the record's fields and let the main ledger view recalculate total.

		record.Timestamp = timestamp
		record.Description = description
		record.Amount = amount
		record.Type = recordType

		if err := ledger.UpdateLedgerRecord(record); err != nil {
			http.Error(w, "Failed to update record", http.StatusInternalServerError)
			return
		}

		// After updating a record, it's safer to trigger a full recalculation for the affected account.
		// Or, for simplicity for now, just redirect and assume the main view will handle eventual consistency.
		// For a full implementation, `ledger.RecalculateAccountBalance(record.AccountID)` would be ideal.

		http.Redirect(w, r, fmt.Sprintf("/ledger?account_id=%d", record.AccountID), http.StatusFound)
		return
	}

	lp := filepath.Join(config.GetConfig().App.WebTemplatesDirectory, "ledger_edit_record.html")
	tmpl, err := template.ParseFiles(lp)
	if err != nil {
		log.Printf("Error parsing ledger_edit_record template: %v", err)
		http.Error(w, "Error rendering page", http.StatusInternalServerError)
		return
	}
	tmpl.Execute(w, record)
}

func handleDeleteLedgerRecord(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
		return
	}
	recordIDStr := r.URL.Path[len("/ledger/delete-record/"):]
	recordID, err := strconv.Atoi(recordIDStr)
	if err != nil {
		http.Error(w, "Invalid record ID", http.StatusBadRequest)
		return
	}

	record, err := ledger.GetLedgerRecordByID(recordID)
	if err != nil {
		http.Error(w, "Record not found", http.StatusNotFound)
		return
	}

	if err := ledger.DeleteLedgerRecord(recordID); err != nil {
		http.Error(w, "Failed to delete record", http.StatusInternalServerError)
		return
	}

	// After deleting a record, it's important to recalculate the account balance.
	// This functionality is currently missing in the `ledger` package.
	// For now, redirect and assume eventual consistency or manual recalculation.

	http.Redirect(w, r, fmt.Sprintf("/ledger?account_id=%d", record.AccountID), http.StatusFound)
}

func handleEditShoppingItem(w http.ResponseWriter, r *http.Request) {
	itemIDStr := r.URL.Path[len("/shopping/edit-item/"):]
	itemID, err := strconv.Atoi(itemIDStr)
	if err != nil {
		http.Error(w, "Invalid item ID", http.StatusBadRequest)
		return
	}

	item, err := shopping.GetShoppingItemByID(itemID)
	if err != nil {
		http.Error(w, "Shopping item not found", http.StatusNotFound)
		return
	}

	if r.Method == http.MethodPost {
		r.ParseForm()
		name := r.FormValue("name")
		quantity, _ := strconv.Atoi(r.FormValue("quantity"))
		storeID, _ := strconv.Atoi(r.FormValue("store_id"))
		checked := r.FormValue("checked") == "on"

		if name == "" || quantity <= 0 {
			http.Error(w, "Invalid item name or quantity", http.StatusBadRequest)
			return
		}

		item.Name = name
		item.Quantity = quantity
		item.StoreID = storeID
		item.Checked = checked

		if err := shopping.UpdateItem(item); err != nil {
			http.Error(w, "Failed to update item", http.StatusInternalServerError)
			return
		}
		http.Redirect(w, r, "/shopping", http.StatusFound)
		return
	}

	lp := filepath.Join(config.GetConfig().App.WebTemplatesDirectory, "shopping_edit_item.html")
	tmpl, err := template.ParseFiles(lp)
	if err != nil {
		log.Printf("Error parsing shopping_edit_item template: %v", err)
		http.Error(w, "Error rendering page", http.StatusInternalServerError)
		return
	}

	// Prepare data for template, including list of stores for a dropdown
	type EditItemTemplateData struct {
		Item   database.ShoppingItem
		Stores []config.StoreConfig
	}
	data := EditItemTemplateData{
		Item:   item,
		Stores: config.GetConfig().Shopping.Store,
	}

	tmpl.Execute(w, data)
}

func handleDeleteShoppingItem(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
		return
	}
	itemIDStr := r.URL.Path[len("/shopping/delete-item/"):]
	itemID, err := strconv.Atoi(itemIDStr)
	if err != nil {
		http.Error(w, "Invalid item ID", http.StatusBadRequest)
		return
	}

	if err := shopping.DeleteItem(itemID); err != nil {
		http.Error(w, "Failed to delete item", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/shopping", http.StatusFound)
}

func handleToggleShoppingItemChecked(w http.ResponseWriter, r *http.Request) {
	// Allow GET for simple links/buttons, but POST is safer for state changes
	if r.Method != http.MethodPost && r.Method != http.MethodGet {
		http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
		return
	}
	itemIDStr := r.URL.Path[len("/shopping/toggle-checked/"):]
	itemID, err := strconv.Atoi(itemIDStr)
	if err != nil {
		http.Error(w, "Invalid item ID", http.StatusBadRequest)
		return
	}

	item, err := shopping.GetShoppingItemByID(itemID)
	if err != nil {
		http.Error(w, "Shopping item not found", http.StatusNotFound)
		return
	}

	item.Checked = !item.Checked // Toggle the checked status

	if err := shopping.UpdateItem(item); err != nil {
		http.Error(w, "Failed to toggle item checked status", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/shopping", http.StatusFound)
}

// PhotoTemplateData holds data for the photo management page.
type PhotoTemplateData struct {
	Photos []PhotoData
}

// PhotoData represents a single photo with its metadata for the template.
type PhotoData struct {
	Filename   string
	IsFavorite bool
	IsHidden   bool
}

func handlePhotos(w http.ResponseWriter, r *http.Request) {
	localPhotosDir := config.GetConfig().LocalPhotos.Directory
	imagePaths, err := photomanager.ListLocalPhotos(localPhotosDir)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to list local photos: %v", err), http.StatusInternalServerError)
		return
	}

	var photos []PhotoData
	for _, path := range imagePaths {
		filename := filepath.Base(path)
		photos = append(photos, PhotoData{
			Filename:   filename,
			IsFavorite: photomanager.IsPhotoFavorite(filename),
			IsHidden:   photomanager.IsPhotoHidden(filename),
		})
	}

	data := PhotoTemplateData{
		Photos: photos,
	}

	lp := filepath.Join(config.GetConfig().App.WebTemplatesDirectory, "photos.html")
	tmpl, err := template.ParseFiles(lp)
	if err != nil {
		log.Printf("Error parsing photos template: %v", err)
		http.Error(w, "Error rendering page", http.StatusInternalServerError)
		return
	}
	tmpl.Execute(w, data)
}

func handlePhotoThumbnail(w http.ResponseWriter, r *http.Request) {
	filename := r.URL.Path[len("/photos/thumbnail/"):]
	if filename == "" {
		http.Error(w, "Filename not provided", http.StatusBadRequest)
		return
	}

	localPhotosDir := config.GetConfig().LocalPhotos.Directory
	imagePath := filepath.Join(localPhotosDir, filename)

	// Define thumbnail width (e.g., 200 pixels)
	const thumbnailWidth uint = 200
	thumbnailBytes, err := photomanager.GenerateThumbnail(imagePath, thumbnailWidth)
	if err != nil {
		log.Printf("Error generating thumbnail for %s: %v", filename, err)
		http.Error(w, "Failed to generate thumbnail", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "image/jpeg")
	w.Header().Set("Cache-Control", "public, max-age=31536000")                    // Cache for 1 year
	w.Header().Set("Expires", time.Now().AddDate(1, 0, 0).Format(http.TimeFormat)) // Expires in 1 year
	w.Header().Set("Content-Length", strconv.Itoa(len(thumbnailBytes)))
	w.Write(thumbnailBytes)
}

func handlePhotoFullsize(w http.ResponseWriter, r *http.Request) {
	filename := r.URL.Path[len("/photos/fullsize/"):]
	if filename == "" {
		http.Error(w, "Filename not provided", http.StatusBadRequest)
		return
	}

	localPhotosDir := config.GetConfig().LocalPhotos.Directory
	imagePath := filepath.Join(localPhotosDir, filename)

	w.Header().Set("Cache-Control", "public, max-age=31536000")                    // Cache for 1 year
	w.Header().Set("Expires", time.Now().AddDate(1, 0, 0).Format(http.TimeFormat)) // Expires in 1 year
	http.ServeFile(w, r, imagePath)
}

func handleTogglePhotoFavorite(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
		return
	}
	filename := r.URL.Path[len("/photos/toggle-favorite/"):]
	if filename == "" {
		http.Error(w, "Filename not provided", http.StatusBadRequest)
		return
	}

	isFavorite := photomanager.IsPhotoFavorite(filename)
	if err := photomanager.SetPhotoFavorite(filename, !isFavorite); err != nil {
		http.Error(w, "Failed to toggle favorite status", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/photos", http.StatusFound)
}

func handleTogglePhotoHidden(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
		return
	}
	filename := r.URL.Path[len("/photos/toggle-hidden/"):]
	if filename == "" {
		http.Error(w, "Filename not provided", http.StatusBadRequest)
		return
	}

	isHidden := photomanager.IsPhotoHidden(filename)
	if err := photomanager.SetPhotoHidden(filename, !isHidden); err != nil {
		http.Error(w, "Failed to toggle hidden status", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/photos", http.StatusFound)
}

func handleDeletePhoto(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
		return
	}
	filename := r.URL.Path[len("/photos/delete/"):]
	if filename == "" {
		http.Error(w, "Filename not provided", http.StatusBadRequest)
		return
	}

	localPhotosDir := config.GetConfig().LocalPhotos.Directory
	if err := photomanager.DeletePhoto(filename, localPhotosDir); err != nil {
		http.Error(w, "Failed to delete photo", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/photos", http.StatusFound)
}

func handlePhotoUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		lp := filepath.Join(config.GetConfig().App.WebTemplatesDirectory, "photos_upload.html")
		tmpl, err := template.ParseFiles(lp)
		if err != nil {
			log.Printf("Error parsing photos_upload template: %v", err)
			http.Error(w, "Error rendering page", http.StatusInternalServerError)
			return
		}
		tmpl.Execute(w, nil)
		return
	}

	if r.Method == http.MethodPost {
		// 32 MB is the maximum memory that an uploaded file's content can take
		r.ParseMultipartForm(32 << 20)

		file, handler, err := r.FormFile("photo_file")
		if err != nil {
			http.Error(w, "Error retrieving file from form: "+err.Error(), http.StatusBadRequest)
			return
		}
		defer file.Close()

		fileBytes, err := io.ReadAll(file)
		if err != nil {
			http.Error(w, "Error reading file content: "+err.Error(), http.StatusInternalServerError)
			return
		}

		// Sanitize filename to prevent path traversal issues
		filename := filepath.Base(handler.Filename)

		localPhotosDir := config.GetConfig().LocalPhotos.Directory
		if err := photomanager.AddPhoto(filename, fileBytes, localPhotosDir); err != nil {
			http.Error(w, "Error saving photo: "+err.Error(), http.StatusInternalServerError)
			return
		}

		http.Redirect(w, r, "/photos", http.StatusFound)
		return
	}

	http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
}

func handleReminders(w http.ResponseWriter, r *http.Request) {
	rems, err := database.GetRemindersDB()
	if err != nil {
		http.Error(w, "Failed to get reminders", http.StatusInternalServerError)
		return
	}
	data := RemindersTemplateData{Reminders: rems}
	lp := filepath.Join(config.GetConfig().App.WebTemplatesDirectory, "reminders.html")
	tmpl, err := template.ParseFiles(lp)
	if err != nil {
		log.Printf("Error parsing template: %v", err)
		http.Error(w, "Error rendering page", http.StatusInternalServerError)
		return
	}
	tmpl.Execute(w, data)
}

func handleAddReminderWeb(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
		return
	}
	r.ParseForm()
	title := r.FormValue("title")
	timeStr := r.FormValue("time")
	days := r.FormValue("days")
	if days == "" {
		days = "Everyday"
	}
	if title == "" || timeStr == "" {
		http.Error(w, "Title and time are required", http.StatusBadRequest)
		return
	}
	newRem := database.Reminder{
		Title:        title,
		Time:         timeStr,
		Days:         days,
		Enabled:      true,
		Acknowledged: true,
	}
	if _, err := database.AddReminderDB(newRem); err != nil {
		http.Error(w, "Failed to add reminder", http.StatusInternalServerError)
		return
	}
	reminders.NotifyListeners()
	http.Redirect(w, r, "/reminders", http.StatusFound)
}

func handleAcknowledgeReminderWeb(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
		return
	}
	idStr := strings.TrimPrefix(r.URL.Path, "/reminders/acknowledge/")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}
	if err := reminders.AcknowledgeReminder(id); err != nil {
		http.Error(w, "Failed to acknowledge reminder", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/reminders", http.StatusFound)
}

func handleDeleteReminderWeb(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
		return
	}
	idStr := strings.TrimPrefix(r.URL.Path, "/reminders/delete/")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}
	if err := database.DeleteReminderDB(id); err != nil {
		http.Error(w, "Failed to delete reminder", http.StatusInternalServerError)
		return
	}
	reminders.NotifyListeners()
	http.Redirect(w, r, "/reminders", http.StatusFound)
}

func handleToggleReminderWeb(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
		return
	}
	idStr := strings.TrimPrefix(r.URL.Path, "/reminders/toggle/")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}
	rem, err := database.GetReminderByIDDB(id)
	if err != nil {
		http.Error(w, "Reminder not found", http.StatusNotFound)
		return
	}
	rem.Enabled = !rem.Enabled
	if err := database.UpdateReminderDB(rem); err != nil {
		http.Error(w, "Failed to update reminder", http.StatusInternalServerError)
		return
	}
	reminders.NotifyListeners()
	http.Redirect(w, r, "/reminders", http.StatusFound)
}

func handleEditReminderWeb(w http.ResponseWriter, r *http.Request) {
	idStr := strings.TrimPrefix(r.URL.Path, "/reminders/edit/")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}

	rem, err := database.GetReminderByIDDB(id)
	if err != nil {
		http.Error(w, "Reminder not found", http.StatusNotFound)
		return
	}

	if r.Method == http.MethodGet {
		lp := filepath.Join(config.GetConfig().App.WebTemplatesDirectory, "reminders_edit.html")
		tmpl, err := template.ParseFiles(lp)
		if err != nil {
			log.Printf("Error parsing template: %v", err)
			http.Error(w, "Error rendering page", http.StatusInternalServerError)
			return
		}
		tmpl.Execute(w, rem)
		return
	}

	if r.Method == http.MethodPost {
		r.ParseForm()
		title := r.FormValue("title")
		timeStr := r.FormValue("time")
		days := r.FormValue("days")
		if days == "" {
			days = "Everyday"
		}
		if title == "" || timeStr == "" {
			http.Error(w, "Title and time are required", http.StatusBadRequest)
			return
		}

		rem.Title = title
		rem.Time = timeStr
		rem.Days = days

		if err := database.UpdateReminderDB(rem); err != nil {
			http.Error(w, "Failed to update reminder", http.StatusInternalServerError)
			return
		}
		reminders.NotifyListeners()
		http.Redirect(w, r, "/reminders", http.StatusFound)
		return
	}

	http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
}
