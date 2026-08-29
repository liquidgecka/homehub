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
	"fmt"
	"log"

	"github.com/liquidgecka/homehub/config"
	"github.com/liquidgecka/homehub/database"
)

// Account represents a single financial account/person.
// It uses the same structure as AccountConfig from config.go.
type Account = config.AccountConfig

// AddAccount adds a new financial account to the database.
var AddAccount = func(name string, initialBalance float64) error {
	id, err := database.AddAccountDB(name, initialBalance)
	if err != nil {
		return fmt.Errorf("failed to add account to database: %w", err)
	}
	log.Printf(
		"Added account to DB: ID=%d, Name=%s, Initial Balance=%.2f",
		id, name, initialBalance,
	)
	return nil
}

// GetAccounts retrieves all financial accounts from the database.
var GetAccounts = func() ([]Account, error) {
	accounts, err := database.GetAccountsDB()
	if err != nil {
		return nil, fmt.Errorf("failed to get accounts from database: %w", err)
	}
	return accounts, nil
}

// GetAccountByID retrieves a single financial account by its ID from the
// database.
var GetAccountByID = func(id int) (Account, error) {
	account, err := database.GetAccountByIDDB(id)
	if err != nil {
		return Account{}, fmt.Errorf(
			"failed to get account by ID %d from database: %w", id, err,
		)
	}
	return account, nil
}

// UpdateAccount updates an existing financial account in the database.
var UpdateAccount = func(account Account) error {
	err := database.UpdateAccountDB(account)
	if err != nil {
		return fmt.Errorf("failed to update account in database: %w", err)
	}
	log.Printf("Updated account in database: %+v", account)
	return nil
}

// DeleteAccount deletes a financial account from the database.

func DeleteAccount(id int) error {

	err := database.DeleteAccountDB(id)

	if err != nil {

		return fmt.Errorf("failed to delete account from database: %w", err)

	}

	log.Printf("Deleted account from database with ID: %d", id)

	return nil

}

// AddLedgerRecord adds a new ledger record to the database.
var AddLedgerRecord = func(record database.LedgerRecord) (int, error) {
	id, err := database.AddLedgerRecordDB(record)
	if err != nil {
		return 0, fmt.Errorf("failed to add ledger record to database: %w", err)
	}
	log.Printf("Added ledger record to database with ID: %d", id)
	return id, nil
}

// GetLedgerRecords retrieves all ledger records for a specific account from
// the database.
var GetLedgerRecords = func(accountID int) ([]database.LedgerRecord, error) {
	records, err := database.GetLedgerRecordsDB(accountID)
	if err != nil {
		return nil, fmt.Errorf(
			"failed to get ledger records from database: %w", err,
		)
	}
	return records, nil
}

// GetLedgerRecordByID retrieves a single ledger record by its ID from the
// database.
var GetLedgerRecordByID = func(id int) (database.LedgerRecord, error) {
	record, err := database.GetLedgerRecordByIDDB(id)
	if err != nil {
		return database.LedgerRecord{}, fmt.Errorf(
			"failed to get ledger record by ID %d from database: %w", id, err,
		)
	}
	return record, nil
}

// UpdateLedgerRecord updates an existing ledger record in the database.
func UpdateLedgerRecord(record database.LedgerRecord) error {
	err := database.UpdateLedgerRecordDB(record)
	if err != nil {
		return fmt.Errorf("failed to update ledger record in database: %w", err)
	}
	log.Printf("Updated ledger record in database with ID: %d", record.ID)
	return nil
}

// DeleteLedgerRecord deletes a ledger record from the database.
func DeleteLedgerRecord(id int) error {
	err := database.DeleteLedgerRecordDB(id)
	if err != nil {
		return fmt.Errorf("failed to delete ledger record from database: %w", err)
	}
	log.Printf("Deleted ledger record from database with ID: %d", id)
	return nil
}
