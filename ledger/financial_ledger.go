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
	"sort"
	"strings"

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
	if err := RecalculateBalances(account.ID); err != nil {
		log.Printf(
			"Failed to recalculate balances after account update: %v", err,
		)
	}
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

// AddLedgerRecord adds a new ledger record to the database and recalculates
// balances.
var AddLedgerRecord = func(record database.LedgerRecord) (int, error) {
	id, err := database.AddLedgerRecordDB(record)
	if err != nil {
		return 0, fmt.Errorf("failed to add ledger record to database: %w", err)
	}
	log.Printf("Added ledger record to database with ID: %d", id)
	if err := RecalculateBalances(record.AccountID); err != nil {
		log.Printf("Failed to recalculate balances after add: %v", err)
	}
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

// UpdateLedgerRecord updates an existing ledger record in the database and
// recalculates balances.
var UpdateLedgerRecord = func(record database.LedgerRecord) error {
	err := database.UpdateLedgerRecordDB(record)
	if err != nil {
		return fmt.Errorf("failed to update ledger record in database: %w", err)
	}
	log.Printf("Updated ledger record in database with ID: %d", record.ID)
	if err := RecalculateBalances(record.AccountID); err != nil {
		log.Printf("Failed to recalculate balances after update: %v", err)
	}
	return nil
}

// DeleteLedgerRecord deletes a ledger record from the database and
// recalculates balances.
var DeleteLedgerRecord = func(id int) error {
	record, err := database.GetLedgerRecordByIDDB(id)
	if err != nil {
		_ = database.DeleteLedgerRecordDB(id)
		return fmt.Errorf("failed to get ledger record %d: %w", id, err)
	}
	err = database.DeleteLedgerRecordDB(id)
	if err != nil {
		return fmt.Errorf("failed to delete ledger record from database: %w", err)
	}
	log.Printf("Deleted ledger record from database with ID: %d", id)
	if err := RecalculateBalances(record.AccountID); err != nil {
		log.Printf("Failed to recalculate balances after delete: %v", err)
	}
	return nil
}

// RecalculateBalances recalculates the running balances for all records of an
// account starting from the account's initial balance in chronological order
// (by timestamp, then id). It then updates the account's current balance.
var RecalculateBalances = func(accountID int) error {
	account, err := database.GetAccountByIDDB(accountID)
	if err != nil {
		return fmt.Errorf("failed to get account %d: %w", accountID, err)
	}

	records, err := database.GetLedgerRecordsDB(accountID)
	if err != nil {
		return fmt.Errorf(
			"failed to get ledger records for account %d: %w", accountID, err,
		)
	}

	// Sort oldest first for chronological balance calculation
	sort.SliceStable(records, func(i, j int) bool {
		if records[i].Timestamp.Equal(records[j].Timestamp) {
			return records[i].ID < records[j].ID
		}
		return records[i].Timestamp.Before(records[j].Timestamp)
	})

	currentBalance := account.InitialBalance
	for i := range records {
		if strings.EqualFold(string(records[i].Type), string(database.Credit)) {
			currentBalance += records[i].Amount
		} else {
			currentBalance -= records[i].Amount
		}
		records[i].Balance = currentBalance
		if err := database.UpdateLedgerRecordDB(records[i]); err != nil {
			log.Printf("Failed to update ledger record balance: %v", err)
		}
	}

	account.CurrentBalance = currentBalance
	if err := database.UpdateAccountDB(account); err != nil {
		return fmt.Errorf(
			"failed to update account %d balance: %w", accountID, err,
		)
	}

	return nil
}

// FormatBalance formats a float64 balance into a two-decimal string with
// comma separators for thousands (e.g., 1234.56 -> "1,234.56",
// -1000.00 -> "-1,000.00").
func FormatBalance(val float64) string {
	str := fmt.Sprintf("%.2f", val)
	if str == "-0.00" {
		str = "0.00"
	}

	sign := ""
	if strings.HasPrefix(str, "-") {
		sign = "-"
		str = str[1:]
	}

	parts := strings.Split(str, ".")
	intPart := parts[0]
	decPart := ""
	if len(parts) > 1 {
		decPart = "." + parts[1]
	}

	if len(intPart) <= 3 {
		return sign + intPart + decPart
	}

	var result []byte
	prefixLen := len(intPart) % 3
	if prefixLen > 0 {
		result = append(result, intPart[:prefixLen]...)
		if prefixLen < len(intPart) {
			result = append(result, ',')
		}
	}
	for i := prefixLen; i < len(intPart); i += 3 {
		result = append(result, intPart[i:i+3]...)
		if i+3 < len(intPart) {
			result = append(result, ',')
		}
	}

	return sign + string(result) + decPart
}
