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
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"

	"github.com/liquidgecka/homehub/config"
	"github.com/liquidgecka/homehub/database"
)

func TestAddAccount(t *testing.T) {
	originalAdd := database.AddAccountDB
	defer func() { database.AddAccountDB = originalAdd }()

	database.AddAccountDB = func(name string, initialBalance float64) (int, error) {
		return 1, nil
	}
	err := AddAccount("test", 100.0)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	database.AddAccountDB = func(name string, initialBalance float64) (int, error) {
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
		t.Errorf("Expected one account named 'test', got %v", accounts)
	}

	database.GetAccountsDB = func() ([]config.AccountConfig, error) {
		return nil, errors.New("db error")
	}
	_, err = GetAccounts()
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
	_, err := AddLedgerRecord(database.LedgerRecord{})
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
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

	database.GetLedgerRecordsDB = func(accountID int) ([]database.LedgerRecord, error) {
		return []database.LedgerRecord{{ID: 1, Description: "test"}}, nil
	}
	records, err := GetLedgerRecords(1)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if len(records) != 1 || records[0].Description != "test" {
		t.Errorf("Expected one record with description 'test', got %v", records)
	}

	database.GetLedgerRecordsDB = func(accountID int) ([]database.LedgerRecord, error) {
		return nil, errors.New("db error")
	}
	_, err = GetLedgerRecords(1)
	if err == nil {
		t.Error("Expected an error, but got nil")
	}
}

func TestUpdateLedgerRecord(t *testing.T) {
	originalUpdate := database.UpdateLedgerRecordDB
	defer func() { database.UpdateLedgerRecordDB = originalUpdate }()

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
	defer func() { database.DeleteLedgerRecordDB = originalDelete }()

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
	originalGetAccounts := database.GetAccountsDB
	originalGetLedger := database.GetLedgerRecordsDB
	originalUpdateLedger := database.UpdateLedgerRecordDB
	originalUpdateAccount := database.UpdateAccountDB
	defer func() {
		database.GetAccountsDB = originalGetAccounts
		database.GetLedgerRecordsDB = originalGetLedger
		database.UpdateLedgerRecordDB = originalUpdateLedger
		database.UpdateAccountDB = originalUpdateAccount
	}()

	database.GetAccountsDB = func() ([]config.AccountConfig, error) {
		return []config.AccountConfig{{ID: 1, Name: "test", InitialBalance: 100.0}}, nil
	}
	database.GetLedgerRecordsDB = func(accountID int) ([]database.LedgerRecord, error) {
		return []database.LedgerRecord{
			{ID: 1, Amount: 50, Type: database.Credit, Timestamp: time.Now().Add(-time.Hour)},
			{ID: 2, Amount: 25, Type: database.Debit, Timestamp: time.Now()},
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

	err := recalculateBalances(1)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if updatedRecords[1].Balance != 150.0 {
		t.Errorf("Expected record 1 balance to be 150.0, got %f", updatedRecords[1].Balance)
	}
	if updatedRecords[2].Balance != 125.0 {
		t.Errorf("Expected record 2 balance to be 125.0, got %f", updatedRecords[2].Balance)
	}
	if updatedAccount.CurrentBalance != 125.0 {
		t.Errorf("Expected account current balance to be 125.0, got %f", updatedAccount.CurrentBalance)
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

	lblNeg := newBalanceLabel(-50.25, fyne.TextAlignLeading, 14)
	if lblNeg == nil || lblNeg.Text.Text != "-50.25" {
		t.Errorf("newBalanceLabel negative failed")
	}

	_ = refreshed
}
