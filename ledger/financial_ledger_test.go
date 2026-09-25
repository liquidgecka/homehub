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

package ledger

import (
	"errors"
	"testing"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"

	"github.com/liquidgecka/homehub/config"
	"github.com/liquidgecka/homehub/database"
	"github.com/liquidgecka/homehub/dialogs"
	"github.com/liquidgecka/homehub/ui"
)

func TestAddAccount(t *testing.T) {
	originalAdd := database.AddAccountDB
	defer func() { database.AddAccountDB = originalAdd }()

	database.AddAccountDB = func(
		name string, initialBalance float64,
	) (int, error) {
		return 1, nil
	}
	err := AddAccount("test", 100.0)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	database.AddAccountDB = func(
		name string, initialBalance float64,
	) (int, error) {
		return 0, errors.New("db error")
	}
	err = AddAccount("test", 100.0)
	if err == nil {
		t.Error("Expected an error, but got nil")
	}
}

func TestGetAccounts(t *testing.T) {
	originalGet := database.GetAccountsDB
	defer func() { database.GetAccountsDB = originalGet }()

	database.GetAccountsDB = func() ([]config.AccountConfig, error) {
		return []config.AccountConfig{{ID: 1, Name: "test"}}, nil
	}
	accounts, err := GetAccounts()
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if len(accounts) != 1 || accounts[0].Name != "test" {
		t.Errorf("Expected one account with name 'test', got %v", accounts)
	}

	database.GetAccountsDB = func() ([]config.AccountConfig, error) {
		return nil, errors.New("db error")
	}
	_, err = GetAccounts()
	if err == nil {
		t.Error("Expected an error, but got nil")
	}
}

func TestGetAccountByID(t *testing.T) {
	originalGet := database.GetAccountByIDDB
	defer func() { database.GetAccountByIDDB = originalGet }()

	database.GetAccountByIDDB = func(id int) (config.AccountConfig, error) {
		return config.AccountConfig{ID: 1, Name: "test"}, nil
	}
	account, err := GetAccountByID(1)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if account.ID != 1 || account.Name != "test" {
		t.Errorf("Expected account with ID 1 and name 'test', got %v", account)
	}

	database.GetAccountByIDDB = func(id int) (config.AccountConfig, error) {
		return config.AccountConfig{}, errors.New("db error")
	}
	_, err = GetAccountByID(1)
	if err == nil {
		t.Error("Expected an error, but got nil")
	}
}

func TestUpdateAccount(t *testing.T) {
	originalUpdate := database.UpdateAccountDB
	defer func() { database.UpdateAccountDB = originalUpdate }()

	database.UpdateAccountDB = func(account config.AccountConfig) error {
		return nil
	}
	err := UpdateAccount(Account{})
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	database.UpdateAccountDB = func(account config.AccountConfig) error {
		return errors.New("db error")
	}
	err = UpdateAccount(Account{})
	if err == nil {
		t.Error("Expected an error, but got nil")
	}
}

func TestDeleteAccount(t *testing.T) {
	originalDelete := database.DeleteAccountDB
	defer func() { database.DeleteAccountDB = originalDelete }()

	database.DeleteAccountDB = func(id int) error {
		return nil
	}
	err := DeleteAccount(1)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	database.DeleteAccountDB = func(id int) error {
		return errors.New("db error")
	}
	err = DeleteAccount(1)
	if err == nil {
		t.Error("Expected an error, but got nil")
	}
}

func TestAddLedgerRecord(t *testing.T) {
	originalAdd := database.AddLedgerRecordDB
	defer func() { database.AddLedgerRecordDB = originalAdd }()

	database.AddLedgerRecordDB = func(record database.LedgerRecord) (int, error) {
		return 1, nil
	}
	id, err := AddLedgerRecord(database.LedgerRecord{})
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if id != 1 {
		t.Errorf("Expected ID 1, got %d", id)
	}

	database.AddLedgerRecordDB = func(record database.LedgerRecord) (int, error) {
		return 0, errors.New("db error")
	}
	_, err = AddLedgerRecord(database.LedgerRecord{})
	if err == nil {
		t.Error("Expected an error, but got nil")
	}
}

func TestGetLedgerRecords(t *testing.T) {
	originalGet := database.GetLedgerRecordsDB
	defer func() { database.GetLedgerRecordsDB = originalGet }()

	database.GetLedgerRecordsDB = func(
		accountID int,
	) ([]database.LedgerRecord, error) {
		return []database.LedgerRecord{{ID: 1, Description: "test"}}, nil
	}
	records, err := GetLedgerRecords(1)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if len(records) != 1 || records[0].Description != "test" {
		t.Errorf("Expected one record with description 'test', got %v", records)
	}

	database.GetLedgerRecordsDB = func(
		accountID int,
	) ([]database.LedgerRecord, error) {
		return nil, errors.New("db error")
	}
	_, err = GetLedgerRecords(1)
	if err == nil {
		t.Error("Expected an error, but got nil")
	}
}

func TestUpdateLedgerRecord(t *testing.T) {
	originalUpdate := database.UpdateLedgerRecordDB
	originalRecalc := RecalculateBalances
	defer func() {
		database.UpdateLedgerRecordDB = originalUpdate
		RecalculateBalances = originalRecalc
	}()

	RecalculateBalances = func(accountID int) error { return nil }

	database.UpdateLedgerRecordDB = func(record database.LedgerRecord) error {
		return nil
	}
	err := UpdateLedgerRecord(database.LedgerRecord{})
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	database.UpdateLedgerRecordDB = func(record database.LedgerRecord) error {
		return errors.New("db error")
	}
	err = UpdateLedgerRecord(database.LedgerRecord{})
	if err == nil {
		t.Error("Expected an error, but got nil")
	}
}

func TestDeleteLedgerRecord(t *testing.T) {
	originalDelete := database.DeleteLedgerRecordDB
	originalGet := database.GetLedgerRecordByIDDB
	originalRecalc := RecalculateBalances
	defer func() {
		database.DeleteLedgerRecordDB = originalDelete
		database.GetLedgerRecordByIDDB = originalGet
		RecalculateBalances = originalRecalc
	}()

	RecalculateBalances = func(accountID int) error { return nil }
	database.GetLedgerRecordByIDDB = func(id int) (database.LedgerRecord, error) {
		return database.LedgerRecord{ID: id, AccountID: 1}, nil
	}

	database.DeleteLedgerRecordDB = func(id int) error {
		return nil
	}
	err := DeleteLedgerRecord(1)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	database.DeleteLedgerRecordDB = func(id int) error {
		return errors.New("db error")
	}
	err = DeleteLedgerRecord(1)
	if err == nil {
		t.Error("Expected an error, but got nil")
	}
}

func TestRecalculateBalances(t *testing.T) {
	originalGetAccount := database.GetAccountByIDDB
	originalGetLedger := database.GetLedgerRecordsDB
	originalUpdateLedger := database.UpdateLedgerRecordDB
	originalUpdateAccount := database.UpdateAccountDB
	defer func() {
		database.GetAccountByIDDB = originalGetAccount
		database.GetLedgerRecordsDB = originalGetLedger
		database.UpdateLedgerRecordDB = originalUpdateLedger
		database.UpdateAccountDB = originalUpdateAccount
	}()

	database.GetAccountByIDDB = func(id int) (config.AccountConfig, error) {
		return config.AccountConfig{
			ID: 1, Name: "test", InitialBalance: 100.0,
		}, nil
	}
	database.GetLedgerRecordsDB = func(
		accountID int,
	) ([]database.LedgerRecord, error) {
		return []database.LedgerRecord{
			{
				ID:        1,
				Amount:    50,
				Type:      database.Credit,
				Timestamp: time.Now().Add(-time.Hour),
			},
			{
				ID:        2,
				Amount:    25,
				Type:      database.Debit,
				Timestamp: time.Now(),
			},
		}, nil
	}
	updatedRecords := make(map[int]database.LedgerRecord)
	database.UpdateLedgerRecordDB = func(record database.LedgerRecord) error {
		updatedRecords[record.ID] = record
		return nil
	}
	var updatedAccount config.AccountConfig
	database.UpdateAccountDB = func(account config.AccountConfig) error {
		updatedAccount = account
		return nil
	}

	err := RecalculateBalances(1)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if updatedRecords[1].Balance != 150.0 {
		t.Errorf(
			"Expected record 1 balance 150.0, got %f",
			updatedRecords[1].Balance,
		)
	}
	if updatedRecords[2].Balance != 125.0 {
		t.Errorf(
			"Expected record 2 balance 125.0, got %f",
			updatedRecords[2].Balance,
		)
	}
	if updatedAccount.CurrentBalance != 125.0 {
		t.Errorf(
			"Expected account balance 125.0, got %f",
			updatedAccount.CurrentBalance,
		)
	}
}

func TestRecalculateBalances_CopperheadScenario(t *testing.T) {
	originalGetAccount := database.GetAccountByIDDB
	originalGetLedger := database.GetLedgerRecordsDB
	originalUpdateLedger := database.UpdateLedgerRecordDB
	originalUpdateAccount := database.UpdateAccountDB
	defer func() {
		database.GetAccountByIDDB = originalGetAccount
		database.GetLedgerRecordsDB = originalGetLedger
		database.UpdateLedgerRecordDB = originalUpdateLedger
		database.UpdateAccountDB = originalUpdateAccount
	}()

	database.GetAccountByIDDB = func(id int) (config.AccountConfig, error) {
		return config.AccountConfig{
			ID: 1, Name: "Esther", InitialBalance: -148.25,
		}, nil
	}

	t0 := time.Date(2026, 8, 16, 10, 0, 0, 0, time.UTC)
	records := []database.LedgerRecord{
		{ID: 1, Amount: 20.0, Type: database.Credit, Timestamp: t0},
		{ID: 2, Amount: 50.0, Type: database.Credit, Timestamp: t0.Add(2 * 24 * time.Hour)},
		{ID: 3, Amount: 40.0, Type: database.Credit, Timestamp: t0.Add(3 * 24 * time.Hour)},
		{ID: 4, Amount: 15.0, Type: database.Credit, Timestamp: t0.Add(10 * 24 * time.Hour)},
		{ID: 5, Amount: 15.0, Type: database.Credit, Timestamp: t0.Add(25 * 24 * time.Hour)},
		{ID: 6, Amount: 20.0, Type: database.Credit, Timestamp: t0.Add(26 * 24 * time.Hour)},
		// Dog poop changed from debit to credit
		{ID: 7, Description: "Dog poop", Amount: 20.0, Type: database.Credit, Timestamp: t0.Add(28 * 24 * time.Hour)},
		// Trying the noodle
		{ID: 8, Description: "Trying the noodle", Amount: 1.0, Type: database.Credit, Timestamp: t0.Add(28*24*time.Hour + time.Hour)},
	}

	database.GetLedgerRecordsDB = func(
		accountID int,
	) ([]database.LedgerRecord, error) {
		return records, nil
	}

	updatedRecords := make(map[int]database.LedgerRecord)
	database.UpdateLedgerRecordDB = func(record database.LedgerRecord) error {
		updatedRecords[record.ID] = record
		return nil
	}
	var updatedAccount config.AccountConfig
	database.UpdateAccountDB = func(account config.AccountConfig) error {
		updatedAccount = account
		return nil
	}

	err := RecalculateBalances(1)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Initial: -148.25
	// + 20 = -128.25
	// + 50 = -78.25
	// + 40 = -38.25
	// + 15 = -23.25
	// + 15 = -8.25
	// + 20 = 11.75
	// + 20 (Dog poop credit) = 31.75
	// + 1 (Trying the noodle) = 32.75
	if updatedRecords[7].Balance != 31.75 {
		t.Errorf("Expected Dog poop balance 31.75, got %f", updatedRecords[7].Balance)
	}
	if updatedRecords[8].Balance != 32.75 {
		t.Errorf("Expected Trying the noodle balance 32.75, got %f", updatedRecords[8].Balance)
	}
	if updatedAccount.CurrentBalance != 32.75 {
		t.Errorf("Expected account balance 32.75, got %f", updatedAccount.CurrentBalance)
	}
}

func TestCreateFinanceView(t *testing.T) {
	_, cleanup, err := database.NewTestDB()
	if err != nil {
		t.Fatalf("NewTestDB failed: %v", err)
	}
	defer cleanup()

	app := test.NewApp()
	_ = app
	win := test.NewWindow(widget.NewLabel("Test Ledger"))
	defer win.Close()

	var refreshed bool
	viewObj := CreateFinanceView(win, func() {
		refreshed = true
	})
	if viewObj == nil {
		t.Fatal("CreateFinanceView returned nil")
	}

	lblPos := newBalanceLabel(100.50, fyne.TextAlignCenter, 14)
	if lblPos == nil || lblPos.Text.Text != "100.50" {
		t.Errorf("newBalanceLabel positive failed")
	}

	lblThousands := newBalanceLabel(1250.75, fyne.TextAlignCenter, 14)
	if lblThousands == nil || lblThousands.Text.Text != "1,250.75" {
		t.Errorf("newBalanceLabel thousands failed: got %v", lblThousands.Text.Text)
	}

	lblNeg := newBalanceLabel(-50.25, fyne.TextAlignLeading, 14)
	if lblNeg == nil || lblNeg.Text.Text != "-50.25" {
		t.Errorf("newBalanceLabel negative failed")
	}

	lblNegThousands := newBalanceLabel(-1250.75, fyne.TextAlignLeading, 14)
	if lblNegThousands == nil || lblNegThousands.Text.Text != "-1,250.75" {
		t.Errorf(
			"newBalanceLabel negative thousands failed: got %v",
			lblNegThousands.Text.Text,
		)
	}

	origGetRecords := database.GetLedgerRecordsDB
	defer func() { database.GetLedgerRecordsDB = origGetRecords }()
	database.GetLedgerRecordsDB = func(
		id int,
	) ([]database.LedgerRecord, error) {
		return []database.LedgerRecord{
			{
				ID:          1,
				AccountID:   1,
				Description: "Groceries",
				Amount:      50.0,
				Type:        database.Debit,
				Balance:     450.0,
				Timestamp:   time.Now(),
			},
			{
				ID:          2,
				AccountID:   1,
				Description: "Deposit",
				Amount:      200.0,
				Type:        database.Credit,
				Balance:     650.0,
				Timestamp:   time.Now(),
			},
		}, nil
	}

	acc := Account{ID: 1, Name: "Checking", CurrentBalance: 500.0}
	lv := createLedgerView(acc, win, func() {})
	if lv == nil {
		t.Error("createLedgerView returned nil")
	}

	// Trigger button and row taps inside createLedgerView
	if border, ok := lv.(*fyne.Container); ok && len(border.Objects) > 1 {
		// addRecordButton is at Objects[1]
		if addBtn, ok := border.Objects[1].(*widget.Button); ok && addBtn.OnTapped != nil {
			addBtn.OnTapped()
			dialogs.CloseAll()
		}
		// vscroll is at Objects[len-1]
		if vscroll, ok := border.Objects[len(border.Objects)-1].(*container.Scroll); ok {
			if vbox, ok := vscroll.Content.(*fyne.Container); ok {
				for _, rowObj := range vbox.Objects {
					if rowCont, ok := rowObj.(*fyne.Container); ok {
						for _, child := range rowCont.Objects {
							switch w := child.(type) {
							case *ui.TappableText:
								w.Tapped(&fyne.PointEvent{})
								dialogs.CloseAll()
							}
						}
					}
				}
			}
		}
	}

	// Test error case in createLedgerView
	database.GetLedgerRecordsDB = func(
		id int,
	) ([]database.LedgerRecord, error) {
		return nil, errors.New("mock db failure")
	}
	lvErr := createLedgerView(acc, win, func() {})
	if lvErr == nil {
		t.Error("createLedgerView should handle db errors")
	}

	// Test ledgerLayout
	ll := &ledgerLayout{}
	objs := []fyne.CanvasObject{
		widget.NewLabel("2026-09-20"),
		widget.NewLabel("Groceries"),
		widget.NewLabel("50.00"),
		widget.NewLabel("450.00"),
	}
	llMin := ll.MinSize(objs)
	if llMin.Width <= 0 || llMin.Height <= 0 {
		t.Errorf("invalid ledgerLayout min size: %v", llMin)
	}
	ll.Layout(objs, fyne.NewSize(500, 40))

	// Test dialogs
	tabs := container.NewAppTabs()
	showAddLedgerDialog(win, tabs, func() {})
	dialogs.CloseAll()
	showAddLedgerRecordDialog(win, acc, func() {})
	dialogs.CloseAll()
	showEditLedgerRecordDialog(win, database.LedgerRecord{
		ID:          1,
		AccountID:   1,
		Description: "Groceries",
		Amount:      50.0,
		Type:        database.Debit,
		Timestamp:   time.Now(),
	}, acc, func() {})
	dialogs.CloseAll()
	showLedgerSettingsDialog(win, acc, func() {})
	dialogs.CloseAll()

	_ = refreshed
}

func TestFormatBalance(t *testing.T) {
	tests := []struct {
		input    float64
		expected string
	}{
		{0.0, "0.00"},
		{0.5, "0.50"},
		{9.99, "9.99"},
		{999.99, "999.99"},
		{1000.0, "1,000.00"},
		{1250.5, "1,250.50"},
		{12345.67, "12,345.67"},
		{123456.78, "123,456.78"},
		{1234567.89, "1,234,567.89"},
		{10000000.0, "10,000,000.00"},
		{-0.0, "0.00"},
		{-0.5, "-0.50"},
		{-999.99, "-999.99"},
		{-1000.0, "-1,000.00"},
		{-1250.5, "-1,250.50"},
		{-1234567.89, "-1,234,567.89"},
	}

	for _, tc := range tests {
		got := FormatBalance(tc.input)
		if got != tc.expected {
			t.Errorf("FormatBalance(%f) = %q, want %q", tc.input, got, tc.expected)
		}
	}
}
