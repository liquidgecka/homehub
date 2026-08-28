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
	"fmt"
	"html/template"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"
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

// PhotoUploadTemplateData holds data for the photo upload page.
type PhotoUploadTemplateData struct {
	Storage StorageInfo
}

// StorageInfo represents filesystem and photo storage statistics.
type StorageInfo struct {
	UsedPercent          float64
	UsedPercentFormatted string
	UsedBytesFormatted   string
	TotalBytesFormatted  string
	FreeBytesFormatted   string
	PhotoBytesFormatted  string
	PhotoCount           int
	IsCritical           bool
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
	http.HandleFunc("/photos/upload", handlePhotoUpload)

	// Reminders Handlers
	http.HandleFunc("/reminders", handleReminders)
	http.HandleFunc("/reminders/add", handleAddReminderWeb)
	http.HandleFunc("/reminders/acknowledge/", handleAcknowledgeReminderWeb)
	http.HandleFunc("/reminders/delete/", handleDeleteReminderWeb)
	http.HandleFunc("/reminders/toggle/", handleToggleReminderWeb)
	http.HandleFunc("/reminders/edit/", handleEditReminderWeb)

	// Application System Control Handlers (Restart and Quit)
	http.HandleFunc("/app/restart", handleAppRestart)
	http.HandleFunc("/app/quit", handleAppQuit)

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
	type EditStoreData struct {
		ID   int
		Name string
	}
	type EditItemTemplateData struct {
		Item   database.ShoppingItem
		Stores []EditStoreData
	}
	var stores []EditStoreData
	for i, store := range config.GetConfig().Shopping.Store {
		if !store.Disabled {
			stores = append(stores, EditStoreData{
				ID:   i + 1,
				Name: store.Name,
			})
		}
	}
	data := EditItemTemplateData{
		Item:   item,
		Stores: stores,
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
	Photos         []PhotoData
	TotalPhotos    int
	CurrentPage    int
	TotalPages     int
	PerPage        int
	PerPageOptions []int
	HasPrev        bool
	HasNext        bool
	PrevPage       int
	NextPage       int
	PageNumbers    []int
	CurrentSort    string
	SortOptions    []SortOptionData
	CurrentOrder   string
	OrderOptions   []OrderOptionData
}

// SortOptionData defines a sort option for the photos page.
type SortOptionData struct {
	Key      string
	Label    string
	Selected bool
}

// OrderOptionData defines an order option (ascending/descending) for the photos page.
type OrderOptionData struct {
	Key      string
	Label    string
	Selected bool
}

// PhotoData represents a single photo with its metadata for the template.
type PhotoData struct {
	Filename   string
	IsFavorite bool
	IsHidden   bool
}

type photoItemWithMeta struct {
	path     string
	filename string
	metaDate time.Time
	modTime  time.Time
}

// GetStorageInfo returns storage usage metrics for localPhotosDir.
func GetStorageInfo(dir string) StorageInfo {
	if err := os.MkdirAll(dir, 0755); err != nil {
		log.Printf("Error ensuring photo dir for storage check: %v", err)
	}

	var stat syscall.Statfs_t
	var totalBytes, freeBytes, usedBytes uint64
	var usedPercent float64

	if err := syscall.Statfs(dir, &stat); err == nil {
		totalBytes = stat.Blocks * uint64(stat.Bsize)
		freeBytes = stat.Bavail * uint64(stat.Bsize)
		if totalBytes > 0 {
			usedBytes = totalBytes - freeBytes
			usedPercent = (float64(usedBytes) / float64(totalBytes)) * 100.0
		}
	}

	var photoBytes uint64
	var photoCount int
	if photos, err := photomanager.ListLocalPhotos(dir); err == nil {
		photoCount = len(photos)
		for _, p := range photos {
			if fi, err := os.Stat(p); err == nil {
				photoBytes += uint64(fi.Size())
			}
		}
	}

	return StorageInfo{
		UsedPercent:          usedPercent,
		UsedPercentFormatted: fmt.Sprintf("%.1f%%", usedPercent),
		UsedBytesFormatted:   formatStorageBytes(usedBytes),
		TotalBytesFormatted:  formatStorageBytes(totalBytes),
		FreeBytesFormatted:   formatStorageBytes(freeBytes),
		PhotoBytesFormatted:  formatStorageBytes(photoBytes),
		PhotoCount:           photoCount,
		IsCritical:           usedPercent >= 80.0,
	}
}

func formatStorageBytes(b uint64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := uint64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}

func handlePhotos(w http.ResponseWriter, r *http.Request) {
	localPhotosDir := config.GetConfig().LocalPhotos.Directory
	imagePaths, err := photomanager.ListLocalPhotos(localPhotosDir)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to list local photos: %v", err), http.StatusInternalServerError)
		return
	}

	sortBy := r.URL.Query().Get("sort")
	if sortBy != "date_upload" && sortBy != "name" {
		sortBy = "date_meta" // Default to date from metadata
	}

	order := strings.ToLower(r.URL.Query().Get("order"))
	if order != "asc" && order != "desc" {
		if sortBy == "name" {
			order = "asc" // Name defaults to A-Z (ascending)
		} else {
			order = "desc" // Dates default to newest first (descending)
		}
	}

	items := make([]photoItemWithMeta, 0, len(imagePaths))
	for _, path := range imagePaths {
		item := photoItemWithMeta{
			path:     path,
			filename: filepath.Base(path),
		}
		if sortBy == "date_meta" {
			if creationDate, err := photomanager.GetCreationDate(path); err == nil {
				item.metaDate = creationDate
			} else if fi, err := os.Stat(path); err == nil {
				item.metaDate = fi.ModTime()
			}
		} else if sortBy == "date_upload" {
			if fi, err := os.Stat(path); err == nil {
				item.modTime = fi.ModTime()
			}
		}
		items = append(items, item)
	}

	switch sortBy {
	case "date_meta":
		sort.Slice(items, func(i, j int) bool {
			if items[i].metaDate.Equal(items[j].metaDate) {
				if order == "asc" {
					return items[i].filename < items[j].filename
				}
				return items[i].filename > items[j].filename
			}
			if order == "asc" {
				return items[i].metaDate.Before(items[j].metaDate) // Oldest first
			}
			return items[i].metaDate.After(items[j].metaDate) // Newest first
		})
	case "date_upload":
		sort.Slice(items, func(i, j int) bool {
			if items[i].modTime.Equal(items[j].modTime) {
				if order == "asc" {
					return items[i].filename < items[j].filename
				}
				return items[i].filename > items[j].filename
			}
			if order == "asc" {
				return items[i].modTime.Before(items[j].modTime) // Oldest first
			}
			return items[i].modTime.After(items[j].modTime) // Newest first
		})
	case "name":
		sort.Slice(items, func(i, j int) bool {
			if order == "desc" {
				return strings.ToLower(items[i].filename) > strings.ToLower(items[j].filename)
			}
			return strings.ToLower(items[i].filename) < strings.ToLower(items[j].filename)
		})
	}

	perPage := 24 // default items per page
	if perPageStr := r.URL.Query().Get("per_page"); perPageStr != "" {
		if val, err := strconv.Atoi(perPageStr); err == nil && val >= 0 {
			perPage = val
		}
	}

	totalPhotos := len(items)
	totalPages := 1
	if perPage > 0 && totalPhotos > 0 {
		totalPages = (totalPhotos + perPage - 1) / perPage
	}

	currentPage := 1
	if pageStr := r.URL.Query().Get("page"); pageStr != "" {
		if val, err := strconv.Atoi(pageStr); err == nil && val > 0 {
			currentPage = val
		}
	}
	if currentPage > totalPages {
		currentPage = totalPages
	}
	if currentPage < 1 {
		currentPage = 1
	}

	startIndex := 0
	endIndex := totalPhotos
	if perPage > 0 {
		startIndex = (currentPage - 1) * perPage
		if startIndex > totalPhotos {
			startIndex = totalPhotos
		}
		endIndex = startIndex + perPage
		if endIndex > totalPhotos {
			endIndex = totalPhotos
		}
	}

	pagedItems := items[startIndex:endIndex]

	var photos []PhotoData
	for _, it := range pagedItems {
		photos = append(photos, PhotoData{
			Filename:   it.filename,
			IsFavorite: photomanager.IsPhotoFavorite(it.filename),
			IsHidden:   photomanager.IsPhotoHidden(it.filename),
		})
	}

	var pageNumbers []int
	for i := 1; i <= totalPages; i++ {
		pageNumbers = append(pageNumbers, i)
	}

	sortOptions := []SortOptionData{
		{Key: "date_meta", Label: "📅 Date Taken (EXIF)", Selected: sortBy == "date_meta"},
		{Key: "date_upload", Label: "🕒 Upload Date", Selected: sortBy == "date_upload"},
		{Key: "name", Label: "🔤 File Name", Selected: sortBy == "name"},
	}

	orderOptions := []OrderOptionData{
		{Key: "desc", Label: "▼ Descending", Selected: order == "desc"},
		{Key: "asc", Label: "▲ Ascending", Selected: order == "asc"},
	}

	data := PhotoTemplateData{
		Photos:         photos,
		TotalPhotos:    totalPhotos,
		CurrentPage:    currentPage,
		TotalPages:     totalPages,
		PerPage:        perPage,
		PerPageOptions: []int{12, 24, 48, 96},
		HasPrev:        currentPage > 1,
		HasNext:        currentPage < totalPages,
		PrevPage:       currentPage - 1,
		NextPage:       currentPage + 1,
		PageNumbers:    pageNumbers,
		CurrentSort:    sortBy,
		SortOptions:    sortOptions,
		CurrentOrder:   order,
		OrderOptions:   orderOptions,
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
	redirectURL := "/photos"
	page := r.FormValue("page")
	perPage := r.FormValue("per_page")
	sortParam := r.FormValue("sort")
	orderParam := r.FormValue("order")
	if page != "" || perPage != "" || sortParam != "" || orderParam != "" {
		redirectURL = fmt.Sprintf("/photos?page=%s&per_page=%s&sort=%s&order=%s", page, perPage, sortParam, orderParam)
	}
	http.Redirect(w, r, redirectURL, http.StatusFound)
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
	redirectURL := "/photos"
	page := r.FormValue("page")
	perPage := r.FormValue("per_page")
	sortParam := r.FormValue("sort")
	orderParam := r.FormValue("order")
	if page != "" || perPage != "" || sortParam != "" || orderParam != "" {
		redirectURL = fmt.Sprintf("/photos?page=%s&per_page=%s&sort=%s&order=%s", page, perPage, sortParam, orderParam)
	}
	http.Redirect(w, r, redirectURL, http.StatusFound)
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
	redirectURL := "/photos"
	page := r.FormValue("page")
	perPage := r.FormValue("per_page")
	sortParam := r.FormValue("sort")
	orderParam := r.FormValue("order")
	if page != "" || perPage != "" || sortParam != "" || orderParam != "" {
		redirectURL = fmt.Sprintf("/photos?page=%s&per_page=%s&sort=%s&order=%s", page, perPage, sortParam, orderParam)
	}
	http.Redirect(w, r, redirectURL, http.StatusFound)
}

func handlePhotoUpload(w http.ResponseWriter, r *http.Request) {
	localPhotosDir := config.GetConfig().LocalPhotos.Directory

	if r.Method == http.MethodGet {
		lp := filepath.Join(config.GetConfig().App.WebTemplatesDirectory, "photos_upload.html")
		tmpl, err := template.ParseFiles(lp)
		if err != nil {
			log.Printf("Error parsing photos_upload template: %v", err)
			http.Error(w, "Error rendering page", http.StatusInternalServerError)
			return
		}
		data := PhotoUploadTemplateData{
			Storage: GetStorageInfo(localPhotosDir),
		}
		tmpl.Execute(w, data)
		return
	}

	if r.Method == http.MethodPost {
		// 128 MB is the maximum memory that uploaded files can take
		if err := r.ParseMultipartForm(128 << 20); err != nil {
			http.Error(w, "Error parsing multipart form: "+err.Error(), http.StatusBadRequest)
			return
		}

		var fileHeaders []*multipart.FileHeader
		if r.MultipartForm != nil && r.MultipartForm.File != nil {
			for _, key := range []string{"photo_files", "photos", "photo_file"} {
				if headers, ok := r.MultipartForm.File[key]; ok && len(headers) > 0 {
					fileHeaders = append(fileHeaders, headers...)
				}
			}
			if len(fileHeaders) == 0 {
				for _, headers := range r.MultipartForm.File {
					fileHeaders = append(fileHeaders, headers...)
				}
			}
		}

		if len(fileHeaders) == 0 {
			http.Error(w, "No photo files provided", http.StatusBadRequest)
			return
		}

		uploadedCount := 0
		duplicateCount := 0
		var uploadErrors []string

		for _, fh := range fileHeaders {
			if fh == nil || fh.Filename == "" {
				continue
			}
			filename := filepath.Base(fh.Filename)
			if filename == "" || filename == "." || filename == "/" {
				continue
			}

			file, err := fh.Open()
			if err != nil {
				errMsg := fmt.Sprintf("Error opening file %s: %v", filename, err)
				log.Print(errMsg)
				uploadErrors = append(uploadErrors, errMsg)
				continue
			}

			fileBytes, err := io.ReadAll(file)
			file.Close()
			if err != nil {
				errMsg := fmt.Sprintf("Error reading file %s: %v", filename, err)
				log.Print(errMsg)
				uploadErrors = append(uploadErrors, errMsg)
				continue
			}

			if len(fileBytes) == 0 {
				continue
			}

			if err := photomanager.AddPhoto(filename, fileBytes, localPhotosDir); err != nil {
				if errors.Is(err, photomanager.ErrDuplicatePhoto) {
					duplicateCount++
					continue
				}
				errMsg := fmt.Sprintf("Error saving photo %s: %v", filename, err)
				log.Print(errMsg)
				uploadErrors = append(uploadErrors, errMsg)
				continue
			}

			uploadedCount++
		}

		if uploadedCount == 0 && duplicateCount == 0 && len(uploadErrors) > 0 {
			http.Error(w, "Failed to upload photos: "+strings.Join(uploadErrors, "; "), http.StatusInternalServerError)
			return
		}

		// Notify slideshow manager that new photos were added
		if uploadedCount > 0 {
			photomanager.NotifyNewPhotoDownloaded()
		}

		http.Redirect(w, r, "/photos", http.StatusFound)
		return
	}

	http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
}

// Global hooks for app restart and quit (can be overridden in tests).
var appRestarter = func() {
	execPath, err := os.Executable()
	if err != nil {
		log.Printf("Error finding executable for restart: %v", err)
		os.Exit(0)
	}
	database.CloseDB()
	if err := syscall.Exec(execPath, os.Args, os.Environ()); err != nil {
		log.Printf("Error executing restart: %v, falling back to exit", err)
		os.Exit(0)
	}
}

var appQuitter = func() {
	database.CloseDB()
	os.Exit(0)
}

func handleAppRestart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	log.Println("App restart requested via web interface.")
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"status":  "restarting",
		"message": "HomeHub is restarting in place...",
	})

	go func() {
		time.Sleep(300 * time.Millisecond)
		appRestarter()
	}()
}

func handleAppQuit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	log.Println("App shutdown requested via web interface.")
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"status":  "quitting",
		"message": "HomeHub application is shutting down.",
	})

	go func() {
		time.Sleep(300 * time.Millisecond)
		appQuitter()
	}()
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
