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

package database

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"

	"github.com/liquidgecka/homehub/config"
)

var (
	db *sql.DB

	// osUserHomeDir is a package-level variable so it can be reassigned for
	// testing.
	osUserHomeDir = os.UserHomeDir
)

// GetDB returns the global database connection.
func GetDB() *sql.DB {
	return db
}

// SetDB sets the global database connection.
func SetDB(newDB *sql.DB) {
	db = newDB
}

// ShoppingItem represents a single item in the shopping list.
type ShoppingItem struct {
	ID       int
	Name     string
	Quantity int
	Checked  bool
	StoreID  int
}

// LedgerRecordType defines whether a record is a credit or a debit.
type LedgerRecordType string

const (
	Credit LedgerRecordType = "credit"
	Debit  LedgerRecordType = "debit"
)

// LedgerRecord represents a single transaction in a financial ledger.
type LedgerRecord struct {
	ID          int
	AccountID   int
	Timestamp   time.Time
	Description string
	Amount      float64
	Type        LedgerRecordType `json:"type"`
	Balance     float64
}

// Reminder represents a scheduled reminder item.
type Reminder struct {
	ID             int       `json:"id"`
	Title          string    `json:"title"`
	Time           string    `json:"time"`
	Days           string    `json:"days"`
	Enabled        bool      `json:"enabled"`
	LastTriggered  time.Time `json:"last_triggered"`
	Acknowledged   bool      `json:"acknowledged"`
	AcknowledgedAt time.Time `json:"acknowledged_at"`
}

// Celebration represents an annual or date-specific event celebration overlay.
type Celebration struct {
	ID      int    `json:"id"`
	Title   string `json:"title"`
	Type    string `json:"type"`
	Month   int    `json:"month"`
	Day     int    `json:"day"`
	Year    int    `json:"year"`
	Message string `json:"message"`
	Enabled bool   `json:"enabled"`
}

// InitDB creates necessary tables on the DB connection.
// It assumes that the DB variable has already been initialized.
func InitDB() error {
	if db == nil {
		return fmt.Errorf("database not initialized")
	}

	// This table is now replaced by app_storage
	// CREATE TABLE IF NOT EXISTS oauth_tokens ...

	createShoppingItemsTableSQL := `
	CREATE TABLE IF NOT EXISTS shopping_items (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL,
		quantity INTEGER NOT NULL,
		checked BOOLEAN NOT NULL DEFAULT FALSE,
		store_id INTEGER DEFAULT 0
	);`

	if _, err := db.Exec(createShoppingItemsTableSQL); err != nil {
		return fmt.Errorf("unable to create shopping_items table: %w", err)
	}

	createShoppingStoresMetadataTableSQL := `
	CREATE TABLE IF NOT EXISTS shopping_stores_metadata (
		store_id INTEGER PRIMARY KEY,
		last_seen DATETIME NOT NULL
	);`

	if _, err := db.Exec(createShoppingStoresMetadataTableSQL); err != nil {
		return fmt.Errorf("unable to create shopping_stores_metadata table: %w", err)
	}

	createAccountsTableSQL := `
	CREATE TABLE IF NOT EXISTS accounts (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL,
		initial_balance REAL NOT NULL,
		current_balance REAL NOT NULL
	);`

	if _, err := db.Exec(createAccountsTableSQL); err != nil {
		return fmt.Errorf("unable to create accounts table: %w", err)
	}

	createLedgerRecordsTableSQL := `
	CREATE TABLE IF NOT EXISTS ledger_records (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		account_id INTEGER NOT NULL,
		timestamp DATETIME NOT NULL,
		description TEXT NOT NULL,
		amount REAL NOT NULL,
		type TEXT NOT NULL,
		balance REAL NOT NULL,
		FOREIGN KEY(account_id) REFERENCES accounts(id)
	);`

	if _, err := db.Exec(createLedgerRecordsTableSQL); err != nil {
		return fmt.Errorf("unable to create ledger_records table: %w", err)
	}

	createAppStorageTableSQL := `
	CREATE TABLE IF NOT EXISTS app_storage (
		key TEXT PRIMARY KEY,
		value TEXT NOT NULL
	);`

	if _, err := db.Exec(createAppStorageTableSQL); err != nil {
		return fmt.Errorf("unable to create app_storage table: %w", err)
	}

	createRemindersTableSQL := `
	CREATE TABLE IF NOT EXISTS reminders (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		title TEXT NOT NULL,
		time TEXT NOT NULL,
		days TEXT NOT NULL DEFAULT 'Everyday',
		enabled BOOLEAN NOT NULL DEFAULT TRUE,
		last_triggered DATETIME,
		acknowledged BOOLEAN NOT NULL DEFAULT TRUE,
		acknowledged_at DATETIME
	);`

	if _, err := db.Exec(createRemindersTableSQL); err != nil {
		return fmt.Errorf("unable to create reminders table: %w", err)
	}

	createCelebrationsTableSQL := `
	CREATE TABLE IF NOT EXISTS celebrations (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		title TEXT NOT NULL,
		type TEXT NOT NULL DEFAULT 'birthday',
		month INTEGER NOT NULL,
		day INTEGER NOT NULL,
		year INTEGER NOT NULL DEFAULT 0,
		message TEXT NOT NULL,
		enabled BOOLEAN NOT NULL DEFAULT TRUE
	);`

	if _, err := db.Exec(createCelebrationsTableSQL); err != nil {
		return fmt.Errorf("unable to create celebrations table: %w", err)
	}

	log.Println("Database initialized and tables created.")
	return nil
}

// GetDBPath returns the full filesystem path to the production SQLite database.
func GetDBPath() (string, error) {
	usrHomeDir, err := osUserHomeDir()
	if err != nil {
		return "", fmt.Errorf("unable to get user home directory: %w", err)
	}
	return filepath.Join(usrHomeDir, ".local", "homehub", "homehub.db"), nil
}

// OpenFileDB opens the production database file.
func OpenFileDB() error {
	if db != nil {
		return nil // Already open
	}

	dbPath, err := GetDBPath()
	if err != nil {
		return err
	}

	homehubDir := filepath.Dir(dbPath)
	if err := os.MkdirAll(homehubDir, 0700); err != nil {
		return fmt.Errorf("unable to create .homehub directory: %w", err)
	}

	dsn := dbPath + "?_busy_timeout=5000&_journal_mode=WAL"
	newDB, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return fmt.Errorf("unable to open database: %w", err)
	}
	db = newDB
	return nil
}

// CloseDB closes the database connection and resets the global db variable.
func CloseDB() {
	if db != nil {
		db.Close()
		db = nil
		log.Println("Database connection closed.")
	}
}

// NewTestDB creates a new in-memory SQLite database for testing purposes.
// It returns a database connection and a cleanup function.
func NewTestDB() (*sql.DB, func(), error) {
	newDB, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		return nil, nil, fmt.Errorf(
			"unable to open in-memory database: %w", err,
		)
	}
	SetDB(newDB)
	if err := InitDB(); err != nil {
		newDB.Close()
		return nil, nil, fmt.Errorf(
			"unable to initialize test database: %w", err,
		)
	}

	cleanup := func() {
		newDB.Close()
		SetDB(nil) // Reset global db
	}

	return newDB, cleanup, nil
}

// SetStorageValue stores a key-value pair in the database.
var SetStorageValue = func(key, value string) error {
	if db == nil {
		return fmt.Errorf("database not initialized")
	}
	insertSQL := `INSERT OR REPLACE INTO app_storage (key, value) ` +
		`VALUES (?, ?);`
	_, err := db.Exec(insertSQL, key, value)
	if err != nil {
		return fmt.Errorf("unable to store value for key %s: %w", key, err)
	}
	return nil
}

// GetStorageValue retrieves a value by its key from the database.
var GetStorageValue = func(key string) (string, error) {
	if db == nil {
		return "", fmt.Errorf("database not initialized")
	}
	selectSQL := `SELECT value FROM app_storage WHERE key = ?;`
	row := db.QueryRow(selectSQL, key)
	var value string
	err := row.Scan(&value)
	if err != nil {
		return "", err // Return underlying error (could be sql.ErrNoRows)
	}
	return value, nil
}

// DeleteStorageValue deletes a key-value pair from the database.
var DeleteStorageValue = func(key string) error {
	if db == nil {
		return fmt.Errorf("database not initialized")
	}
	deleteSQL := `DELETE FROM app_storage WHERE key = ?;`
	_, err := db.Exec(deleteSQL, key)
	if err != nil {
		return fmt.Errorf("unable to delete value for key %s: %w", key, err)
	}
	return nil
}

// ListStorageKeysWithPrefix retrieves all keys from the app_storage table
// that start with the given prefix.
var ListStorageKeysWithPrefix = func(prefix string) ([]string, error) {
	if db == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	selectSQL := `SELECT key FROM app_storage WHERE key LIKE ?;`
	rows, err := db.Query(selectSQL, prefix+"%")
	if err != nil {
		return nil, fmt.Errorf(
			"unable to retrieve keys with prefix %s: %w", prefix, err,
		)
	}
	defer rows.Close()

	var keys []string
	for rows.Next() {
		var key string
		if err := rows.Scan(&key); err != nil {
			return nil, fmt.Errorf("unable to scan key: %w", err)
		}
		keys = append(keys, key)
	}
	return keys, nil
}

// StoreRefreshToken stores an OAuth refresh token in the app_storage table.
var StoreRefreshToken = func(serviceName, refreshToken string) error {
	return SetStorageValue(fmt.Sprintf("oauth_%s", serviceName), refreshToken)
}

// GetRefreshToken retrieves an OAuth refresh token from the app_storage table.
var GetRefreshToken = func(serviceName string) (string, error) {
	return GetStorageValue(fmt.Sprintf("oauth_%s", serviceName))
}

// DeleteRefreshToken deletes an OAuth refresh token from the app_storage
// table.
var DeleteRefreshToken = func(serviceName string) error {
	if db == nil {
		return fmt.Errorf("database not initialized")
	}
	deleteSQL := `DELETE FROM app_storage WHERE key = ?;`
	_, err := db.Exec(deleteSQL, fmt.Sprintf("oauth_%s", serviceName))
	if err != nil {
		return fmt.Errorf(
			"unable to delete refresh token for service %s: %w",
			serviceName, err,
		)
	}
	return nil
}

// AddShoppingItem adds a new shopping item to the database.
var AddShoppingItem = func(item ShoppingItem) (int, error) {
	if db == nil {
		return 0, fmt.Errorf("database not initialized")
	}
	insertSQL := `INSERT INTO shopping_items ` +
		`(name, quantity, checked, store_id) VALUES (?, ?, ?, ?);`
	result, err := db.Exec(
		insertSQL, item.Name, item.Quantity, item.Checked, item.StoreID,
	)
	if err != nil {
		return 0, fmt.Errorf("unable to add shopping item: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf(
			"unable to get last insert ID for shopping item: %w", err,
		)
	}
	return int(id), nil
}

// GetShoppingItems retrieves all shopping items from the database.
var GetShoppingItems = func() ([]ShoppingItem, error) {
	return GetShoppingItemsByStore(-1) // -1 indicates all stores
}

// GetShoppingItemsByStore retrieves shopping items for a specific store from
// the database.
var GetShoppingItemsByStore = func(storeID int) ([]ShoppingItem, error) {
	if db == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	var selectSQL string
	var rows *sql.Rows
	var err error
	if storeID == -1 {
		cfg := config.GetConfig()
		var activeStoreIDs []string
		for i, store := range cfg.Shopping.Store {
			if !store.Disabled {
				activeStoreIDs = append(
					activeStoreIDs, fmt.Sprintf("%d", i+1),
				)
			}
		}
		if len(activeStoreIDs) == 0 {
			return []ShoppingItem{}, nil
		}
		selectSQL = fmt.Sprintf(
			`SELECT id, name, quantity, checked, store_id `+
				`FROM shopping_items WHERE store_id IN (%s) ORDER BY id DESC;`,
			strings.Join(activeStoreIDs, ","),
		)
		rows, err = db.Query(selectSQL)
	} else {
		selectSQL = `SELECT id, name, quantity, checked, store_id ` +
			`FROM shopping_items WHERE store_id = ? ORDER BY id DESC;`
		rows, err = db.Query(selectSQL, storeID)
	}
	if err != nil {
		return nil, fmt.Errorf(
			"unable to retrieve shopping items for store %d: %w",
			storeID, err,
		)
	}
	defer rows.Close()
	var items []ShoppingItem
	for rows.Next() {
		var item ShoppingItem
		if err := rows.Scan(
			&item.ID, &item.Name, &item.Quantity, &item.Checked, &item.StoreID,
		); err != nil {
			return nil, fmt.Errorf("unable to scan shopping item: %w", err)
		}
		items = append(items, item)
	}
	return items, nil
}

// GetShoppingItemByIDDB retrieves a single shopping item by its ID from the
// database.
var GetShoppingItemByIDDB = func(id int) (ShoppingItem, error) {
	if db == nil {
		return ShoppingItem{}, fmt.Errorf("database not initialized")
	}
	selectSQL := `SELECT id, name, quantity, checked, store_id ` +
		`FROM shopping_items WHERE id = ?;`
	row := db.QueryRow(selectSQL, id)
	var item ShoppingItem
	err := row.Scan(
		&item.ID, &item.Name, &item.Quantity, &item.Checked, &item.StoreID,
	)
	if err != nil {
		return ShoppingItem{}, fmt.Errorf(
			"unable to retrieve shopping item %d: %w", id, err,
		)
	}
	return item, nil
}

// UpdateShoppingItem updates an existing shopping item in the database.
var UpdateShoppingItem = func(item ShoppingItem) error {
	if db == nil {
		return fmt.Errorf("database not initialized")
	}
	updateSQL := `UPDATE shopping_items ` +
		`SET name = ?, quantity = ?, checked = ?, store_id = ? WHERE id = ?;`
	_, err := db.Exec(
		updateSQL, item.Name, item.Quantity, item.Checked, item.StoreID, item.ID,
	)
	if err != nil {
		return fmt.Errorf("unable to update shopping item %d: %w", item.ID, err)
	}
	return nil
}

// DeleteShoppingItem deletes a shopping item from the database.
var DeleteShoppingItem = func(id int) error {
	if db == nil {
		return fmt.Errorf("database not initialized")
	}
	deleteSQL := `DELETE FROM shopping_items WHERE id = ?;`
	_, err := db.Exec(deleteSQL, id)
	if err != nil {
		return fmt.Errorf("unable to delete shopping item %d: %w", id, err)
	}
	return nil
}

// DeleteShoppingItemsByStore deletes all shopping items for a specific store
// from the database.
var DeleteShoppingItemsByStore = func(storeID int) error {
	if db == nil {
		return fmt.Errorf("database not initialized")
	}
	deleteSQL := `DELETE FROM shopping_items WHERE store_id = ?;`
	_, err := db.Exec(deleteSQL, storeID)
	if err != nil {
		return fmt.Errorf(
			"unable to delete shopping items for store %d: %w",
			storeID, err,
		)
	}
	return nil
}

// AddAccountDB adds a new financial account to the database.
var AddAccountDB = func(name string, initialBalance float64) (int, error) {
	if db == nil {
		return 0, fmt.Errorf("database not initialized")
	}
	insertSQL := `INSERT INTO accounts ` +
		`(name, initial_balance, current_balance) VALUES (?, ?, ?);`
	result, err := db.Exec(insertSQL, name, initialBalance, initialBalance)
	if err != nil {
		return 0, fmt.Errorf("unable to add account: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("unable to get last insert ID for account: %w", err)
	}
	return int(id), nil
}

// GetAccountsDB retrieves all financial accounts from the database.
var GetAccountsDB = func() ([]config.AccountConfig, error) {
	if db == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	selectSQL := `SELECT id, name, initial_balance, current_balance ` +
		`FROM accounts ORDER BY id DESC;`
	rows, err := db.Query(selectSQL)
	if err != nil {
		return nil, fmt.Errorf("unable to retrieve accounts: %w", err)
	}
	defer rows.Close()
	var accounts []config.AccountConfig
	for rows.Next() {
		var account config.AccountConfig
		if err := rows.Scan(
			&account.ID, &account.Name,
			&account.InitialBalance, &account.CurrentBalance,
		); err != nil {
			return nil, fmt.Errorf("unable to scan account: %w", err)
		}
		accounts = append(accounts, account)
	}
	return accounts, nil
}

// GetAccountByIDDB retrieves a single financial account by its ID from the
// database.
var GetAccountByIDDB = func(id int) (config.AccountConfig, error) {
	if db == nil {
		return config.AccountConfig{}, fmt.Errorf("database not initialized")
	}
	selectSQL := `SELECT id, name, initial_balance, current_balance ` +
		`FROM accounts WHERE id = ?;`
	row := db.QueryRow(selectSQL, id)
	var account config.AccountConfig
	err := row.Scan(
		&account.ID, &account.Name,
		&account.InitialBalance, &account.CurrentBalance,
	)
	if err != nil {
		return config.AccountConfig{}, fmt.Errorf(
			"unable to retrieve account %d: %w", id, err,
		)
	}
	return account, nil
}

// UpdateAccountDB updates an existing financial account in the database.
var UpdateAccountDB = func(account config.AccountConfig) error {
	if db == nil {
		return fmt.Errorf("database not initialized")
	}
	updateSQL := `UPDATE accounts ` +
		`SET name = ?, initial_balance = ?, current_balance = ? WHERE id = ?;`
	_, err := db.Exec(
		updateSQL,
		account.Name, account.InitialBalance, account.CurrentBalance,
		account.ID,
	)
	if err != nil {
		return fmt.Errorf("unable to update account %d: %w", account.ID, err)
	}
	return nil
}

// DeleteAccountDB deletes a financial account from the database.
var DeleteAccountDB = func(id int) error {
	if db == nil {
		return fmt.Errorf("database not initialized")
	}
	deleteSQL := `DELETE FROM accounts WHERE id = ?;`
	_, err := db.Exec(deleteSQL, id)
	if err != nil {
		return fmt.Errorf("unable to delete account %d: %w", id, err)
	}
	return nil
}

// AddLedgerRecordDB adds a new ledger record to the database.
var AddLedgerRecordDB = func(record LedgerRecord) (int, error) {
	if db == nil {
		return 0, fmt.Errorf("database not initialized")
	}
	insertSQL := `INSERT INTO ledger_records ` +
		`(account_id, timestamp, description, amount, type, balance) ` +
		`VALUES (?, ?, ?, ?, ?, ?);`
	result, err := db.Exec(
		insertSQL, record.AccountID, record.Timestamp, record.Description,
		record.Amount, record.Type, record.Balance,
	)
	if err != nil {
		return 0, fmt.Errorf("unable to add ledger record: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf(
			"unable to get last insert ID for ledger record: %w", err,
		)
	}
	return int(id), nil
}

// GetLedgerRecordsDB retrieves all ledger records for a specific account
// from the database.
var GetLedgerRecordsDB = func(accountID int) ([]LedgerRecord, error) {
	if db == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	selectSQL := `SELECT id, account_id, timestamp, description, amount, ` +
		`type, balance FROM ledger_records ` +
		`WHERE account_id = ? ORDER BY timestamp DESC;`
	rows, err := db.Query(selectSQL, accountID)
	if err != nil {
		return nil, fmt.Errorf("unable to retrieve ledger records: %w", err)
	}
	defer rows.Close()
	var records []LedgerRecord
	for rows.Next() {
		var record LedgerRecord
		if err := rows.Scan(
			&record.ID, &record.AccountID, &record.Timestamp,
			&record.Description, &record.Amount, &record.Type, &record.Balance,
		); err != nil {
			return nil, fmt.Errorf("unable to scan ledger record: %w", err)
		}
		records = append(records, record)
	}
	return records, nil
}

// GetLedgerRecordByIDDB retrieves a single ledger record by its ID from the
// database.
var GetLedgerRecordByIDDB = func(id int) (LedgerRecord, error) {
	if db == nil {
		return LedgerRecord{}, fmt.Errorf("database not initialized")
	}
	selectSQL := `SELECT id, account_id, timestamp, description, amount, ` +
		`type, balance FROM ledger_records WHERE id = ?;`
	row := db.QueryRow(selectSQL, id)
	var record LedgerRecord
	err := row.Scan(
		&record.ID, &record.AccountID, &record.Timestamp,
		&record.Description, &record.Amount, &record.Type, &record.Balance,
	)
	if err != nil {
		return LedgerRecord{}, fmt.Errorf(
			"unable to retrieve ledger record %d: %w", id, err,
		)
	}
	return record, nil
}

// UpdateLedgerRecordDB updates an existing ledger record in the database.
var UpdateLedgerRecordDB = func(record LedgerRecord) error {
	if db == nil {
		return fmt.Errorf("database not initialized")
	}
	updateSQL := `UPDATE ledger_records ` +
		`SET timestamp = ?, description = ?, amount = ?, type = ?, ` +
		`balance = ? WHERE id = ?;`
	_, err := db.Exec(
		updateSQL, record.Timestamp, record.Description, record.Amount,
		record.Type, record.Balance, record.ID,
	)
	if err != nil {
		return fmt.Errorf(
			"unable to update ledger record %d: %w", record.ID, err,
		)
	}
	return nil
}

// DeleteLedgerRecordDB deletes a ledger record from the database.
var DeleteLedgerRecordDB = func(id int) error {
	if db == nil {
		return fmt.Errorf("database not initialized")
	}
	deleteSQL := `DELETE FROM ledger_records WHERE id = ?;`
	_, err := db.Exec(deleteSQL, id)
	if err != nil {
		return fmt.Errorf("unable to delete ledger record %d: %w", id, err)
	}
	return nil
}

// AddOrUpdateShoppingStoreMetadata adds or updates a store's last seen time.
var AddOrUpdateShoppingStoreMetadata = func(
	storeID int, lastSeen time.Time,
) error {
	if db == nil {
		return fmt.Errorf("database not initialized")
	}
	insertSQL := `INSERT OR REPLACE INTO shopping_stores_metadata ` +
		`(store_id, last_seen) VALUES (?, ?);`
	_, err := db.Exec(insertSQL, storeID, lastSeen)
	if err != nil {
		return fmt.Errorf(
			"unable to add or update store metadata for %d: %w",
			storeID, err,
		)
	}
	return nil
}

// GetExpiredShoppingStoreIDs retrieves the IDs of stores that have not been
// seen since the threshold time.
var GetExpiredShoppingStoreIDs = func(threshold time.Time) ([]int, error) {
	if db == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	selectSQL := `SELECT store_id FROM shopping_stores_metadata ` +
		`WHERE last_seen < ?;`
	rows, err := db.Query(selectSQL, threshold)
	if err != nil {
		return nil, fmt.Errorf(
			"unable to retrieve expired shopping store IDs: %w", err,
		)
	}
	defer rows.Close()
	var storeIDs []int
	for rows.Next() {
		var storeID int
		if err := rows.Scan(&storeID); err != nil {
			return nil, fmt.Errorf(
				"unable to scan expired shopping store ID: %w", err,
			)
		}
		storeIDs = append(storeIDs, storeID)
	}
	return storeIDs, nil
}

// DeleteShoppingStoreMetadata deletes a store's metadata from the database.
var DeleteShoppingStoreMetadata = func(storeID int) error {
	if db == nil {
		return fmt.Errorf("database not initialized")
	}
	deleteSQL := `DELETE FROM shopping_stores_metadata WHERE store_id = ?;`
	_, err := db.Exec(deleteSQL, storeID)
	if err != nil {
		return fmt.Errorf(
			"unable to delete store metadata for %d: %w", storeID, err,
		)
	}
	return nil
}

// AddReminderDB adds a new reminder to the database.
var AddReminderDB = func(item Reminder) (int, error) {
	if db == nil {
		return 0, fmt.Errorf("database not initialized")
	}
	if item.Days == "" {
		item.Days = "Everyday"
	}
	insertSQL := `INSERT INTO reminders ` +
		`(title, time, days, enabled, last_triggered, acknowledged, ` +
		`acknowledged_at) VALUES (?, ?, ?, ?, ?, ?, ?);`
	var lastTrig, ackAt interface{}
	if !item.LastTriggered.IsZero() {
		lastTrig = item.LastTriggered
	}
	if !item.AcknowledgedAt.IsZero() {
		ackAt = item.AcknowledgedAt
	}
	result, err := db.Exec(
		insertSQL, item.Title, item.Time, item.Days, item.Enabled,
		lastTrig, item.Acknowledged, ackAt,
	)
	if err != nil {
		return 0, fmt.Errorf("unable to add reminder: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf(
			"unable to get last insert ID for reminder: %w", err,
		)
	}
	return int(id), nil
}

// GetRemindersDB retrieves all reminders from the database.
var GetRemindersDB = func() ([]Reminder, error) {
	if db == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	selectSQL := `SELECT id, title, time, days, enabled, last_triggered, ` +
		`acknowledged, acknowledged_at FROM reminders ORDER BY id ASC;`
	rows, err := db.Query(selectSQL)
	if err != nil {
		return nil, fmt.Errorf("unable to retrieve reminders: %w", err)
	}
	defer rows.Close()

	var reminders []Reminder
	for rows.Next() {
		var r Reminder
		var lastTriggered, ackAt sql.NullTime
		if err := rows.Scan(
			&r.ID, &r.Title, &r.Time, &r.Days, &r.Enabled,
			&lastTriggered, &r.Acknowledged, &ackAt,
		); err != nil {
			return nil, fmt.Errorf("unable to scan reminder: %w", err)
		}
		if lastTriggered.Valid {
			r.LastTriggered = lastTriggered.Time
		}
		if ackAt.Valid {
			r.AcknowledgedAt = ackAt.Time
		}
		reminders = append(reminders, r)
	}
	return reminders, nil
}

// GetReminderByIDDB retrieves a single reminder by its ID from the database.
var GetReminderByIDDB = func(id int) (Reminder, error) {
	if db == nil {
		return Reminder{}, fmt.Errorf("database not initialized")
	}
	selectSQL := `SELECT id, title, time, days, enabled, last_triggered, ` +
		`acknowledged, acknowledged_at FROM reminders WHERE id = ?;`
	row := db.QueryRow(selectSQL, id)
	var r Reminder
	var lastTriggered, ackAt sql.NullTime
	err := row.Scan(
		&r.ID, &r.Title, &r.Time, &r.Days, &r.Enabled,
		&lastTriggered, &r.Acknowledged, &ackAt,
	)
	if err != nil {
		return Reminder{}, fmt.Errorf(
			"unable to retrieve reminder %d: %w", id, err,
		)
	}
	if lastTriggered.Valid {
		r.LastTriggered = lastTriggered.Time
	}
	if ackAt.Valid {
		r.AcknowledgedAt = ackAt.Time
	}
	return r, nil
}

// UpdateReminderDB updates an existing reminder in the database.
var UpdateReminderDB = func(item Reminder) error {
	if db == nil {
		return fmt.Errorf("database not initialized")
	}
	if item.Days == "" {
		item.Days = "Everyday"
	}
	updateSQL := `UPDATE reminders SET title = ?, time = ?, days = ?, ` +
		`enabled = ?, last_triggered = ?, acknowledged = ?, ` +
		`acknowledged_at = ? WHERE id = ?;`
	var lastTrig, ackAt interface{}
	if !item.LastTriggered.IsZero() {
		lastTrig = item.LastTriggered
	}
	if !item.AcknowledgedAt.IsZero() {
		ackAt = item.AcknowledgedAt
	}
	_, err := db.Exec(
		updateSQL, item.Title, item.Time, item.Days, item.Enabled,
		lastTrig, item.Acknowledged, ackAt, item.ID,
	)
	if err != nil {
		return fmt.Errorf("unable to update reminder %d: %w", item.ID, err)
	}
	return nil
}

// DeleteReminderDB deletes a reminder from the database.
var DeleteReminderDB = func(id int) error {
	if db == nil {
		return fmt.Errorf("database not initialized")
	}
	deleteSQL := `DELETE FROM reminders WHERE id = ?;`
	_, err := db.Exec(deleteSQL, id)
	if err != nil {
		return fmt.Errorf("unable to delete reminder %d: %w", id, err)
	}
	return nil
}

// SetReminderAcknowledgedDB marks a reminder as acknowledged.
var SetReminderAcknowledgedDB = func(
	id int, ack bool, ackTime time.Time,
) error {
	if db == nil {
		return fmt.Errorf("database not initialized")
	}
	updateSQL := `UPDATE reminders ` +
		`SET acknowledged = ?, acknowledged_at = ? WHERE id = ?;`
	var ackVal interface{}
	if ack && !ackTime.IsZero() {
		ackVal = ackTime
	}
	_, err := db.Exec(updateSQL, ack, ackVal, id)
	if err != nil {
		return fmt.Errorf(
			"unable to update reminder acknowledged status %d: %w", id, err,
		)
	}
	return nil
}

// SetReminderTriggeredDB updates a reminder when triggered.
var SetReminderTriggeredDB = func(id int, triggerTime time.Time) error {
	if db == nil {
		return fmt.Errorf("database not initialized")
	}
	updateSQL := `UPDATE reminders ` +
		`SET last_triggered = ?, acknowledged = FALSE WHERE id = ?;`
	_, err := db.Exec(updateSQL, triggerTime, id)
	if err != nil {
		return fmt.Errorf("unable to set reminder triggered for %d: %w", id, err)
	}
	return nil
}

// AddCelebrationDB adds a new celebration to the database.
var AddCelebrationDB = func(item Celebration) (int, error) {
	if db == nil {
		return 0, fmt.Errorf("database not initialized")
	}
	if item.Type == "" {
		item.Type = "birthday"
	}
	insertSQL := `INSERT INTO celebrations ` +
		`(title, type, month, day, year, message, enabled) ` +
		`VALUES (?, ?, ?, ?, ?, ?, ?);`
	result, err := db.Exec(
		insertSQL, item.Title, item.Type, item.Month, item.Day, item.Year,
		item.Message, item.Enabled,
	)
	if err != nil {
		return 0, fmt.Errorf("unable to add celebration: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf(
			"unable to get last insert ID for celebration: %w", err,
		)
	}
	return int(id), nil
}

// GetCelebrationsDB retrieves all celebrations from the database.
var GetCelebrationsDB = func() ([]Celebration, error) {
	if db == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	selectSQL := `SELECT id, title, type, month, day, year, message, enabled ` +
		`FROM celebrations ORDER BY month ASC, day ASC, id ASC;`
	rows, err := db.Query(selectSQL)
	if err != nil {
		return nil, fmt.Errorf("unable to retrieve celebrations: %w", err)
	}
	defer rows.Close()

	var celebrations []Celebration
	for rows.Next() {
		var c Celebration
		if err := rows.Scan(
			&c.ID, &c.Title, &c.Type, &c.Month, &c.Day, &c.Year,
			&c.Message, &c.Enabled,
		); err != nil {
			return nil, fmt.Errorf("unable to scan celebration: %w", err)
		}
		celebrations = append(celebrations, c)
	}
	return celebrations, nil
}

// GetCelebrationByIDDB retrieves a single celebration by ID from the database.
var GetCelebrationByIDDB = func(id int) (Celebration, error) {
	if db == nil {
		return Celebration{}, fmt.Errorf("database not initialized")
	}
	selectSQL := `SELECT id, title, type, month, day, year, message, enabled ` +
		`FROM celebrations WHERE id = ?;`
	row := db.QueryRow(selectSQL, id)
	var c Celebration
	err := row.Scan(
		&c.ID, &c.Title, &c.Type, &c.Month, &c.Day, &c.Year,
		&c.Message, &c.Enabled,
	)
	if err != nil {
		return Celebration{}, fmt.Errorf(
			"unable to retrieve celebration %d: %w", id, err,
		)
	}
	return c, nil
}

// UpdateCelebrationDB updates an existing celebration in the database.
var UpdateCelebrationDB = func(item Celebration) error {
	if db == nil {
		return fmt.Errorf("database not initialized")
	}
	if item.Type == "" {
		item.Type = "birthday"
	}
	updateSQL := `UPDATE celebrations SET title = ?, type = ?, month = ?, ` +
		`day = ?, year = ?, message = ?, enabled = ? WHERE id = ?;`
	_, err := db.Exec(
		updateSQL, item.Title, item.Type, item.Month, item.Day, item.Year,
		item.Message, item.Enabled, item.ID,
	)
	if err != nil {
		return fmt.Errorf("unable to update celebration %d: %w", item.ID, err)
	}
	return nil
}

// DeleteCelebrationDB deletes a celebration from the database.
var DeleteCelebrationDB = func(id int) error {
	if db == nil {
		return fmt.Errorf("database not initialized")
	}
	deleteSQL := `DELETE FROM celebrations WHERE id = ?;`
	_, err := db.Exec(deleteSQL, id)
	if err != nil {
		return fmt.Errorf("unable to delete celebration %d: %w", id, err)
	}
	return nil
}
