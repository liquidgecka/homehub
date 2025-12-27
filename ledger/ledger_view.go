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
	"image/color"
	"log"
	"sort"
	"strconv"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/liquidgecka/homehub/database"
	"github.com/liquidgecka/homehub/dialogs"
	"github.com/liquidgecka/homehub/ui"
)

func CreateFinanceView(win fyne.Window, refresh func()) fyne.CanvasObject {
	accounts, err := database.GetAccountsDB()
	if err != nil {
		log.Printf("Failed to get accounts: %v", err)
		return widget.NewLabel("Failed to load accounts.")
	}

	var previousSelectedIndex int

	tabItems := make([]*container.TabItem, 0, len(accounts)+2) // +2 for settings and add tabs
	for _, acc := range accounts {
		account := acc // Create a new variable for the closure
		tabContent := createLedgerView(account, win, refresh)
		tabItems = append(tabItems, container.NewTabItem(account.Name, tabContent))
	}

	tabs := container.NewAppTabs(tabItems...)

	addTab := container.NewTabItemWithIcon("", theme.ContentAddIcon(), container.NewMax())
	tabs.Append(addTab)

	settingsTab := container.NewTabItemWithIcon("", theme.SettingsIcon(), container.NewMax())
	tabs.Append(settingsTab)

	tabs.OnSelected = func(item *container.TabItem) {
		if item == addTab {
			tabs.SelectIndex(previousSelectedIndex) // Go back to previous tab
			showAddLedgerDialog(win, tabs, refresh)
		} else if item == settingsTab {
			tabs.SelectIndex(previousSelectedIndex)                                   // Go back to previous tab
			if previousSelectedIndex != -1 && previousSelectedIndex < len(accounts) { // Check if a valid ledger tab was active
				account := accounts[previousSelectedIndex]
				showLedgerSettingsDialog(win, account, refresh)
			}
		} else {
			previousSelectedIndex = tabs.SelectedIndex()
		}
	}

	// Initially select the first ledger tab if available, otherwise select the add tab
	if previousSelectedIndex < len(tabItems) {
		tabs.SelectIndex(previousSelectedIndex)
	} else {
		tabs.SelectIndex(0)
	}

	return tabs
}

func showAddLedgerDialog(parent fyne.Window, tabs *container.AppTabs, refresh func()) {
	nameEntry := ui.NewKeyboardEntry(parent)
	nameEntry.SetPlaceHolder("Ledger Name")

	balanceEntry := ui.NewKeyboardEntry(parent)
	balanceEntry.SetPlaceHolder("Initial Balance")

	form := &widget.Form{
		Items: []*widget.FormItem{
			{Text: "Name", Widget: nameEntry},
			{Text: "Initial Balance", Widget: balanceEntry},
		},
	}

	// Create an error message label that is initially hidden
	errorText := canvas.NewText("", color.RGBA{R: 255, A: 255})
	errorText.TextStyle.Bold = true
	errorText.Hide()

	content := container.NewMax(
		canvas.NewRectangle(color.Transparent),
		container.NewVBox(form, errorText),
	)
	if rect, ok := content.Objects[0].(*canvas.Rectangle); ok {
		rect.SetMinSize(fyne.NewSize(400, 0))
	}

	d := dialogs.NewCustomConfirm(
		"Add New Ledger",
		"Add",
		"Cancel",
		content,
		func(b bool) {
			if !b {
				return
			}
			name := nameEntry.Text
			if name == "" {
				errorText.Text = "Ledger name cannot be empty."
				errorText.Show()
				// This is a bit of a hack to force the dialog to re-evaluate its size.
				// We resize the content slightly, which triggers a refresh.
				content.Resize(content.Size().Add(fyne.NewSize(0, 1)))
				return
			}
			balance, err := strconv.ParseFloat(balanceEntry.Text, 64)
			if err != nil {
				errorText.Text = "Invalid initial balance."
				errorText.Show()
				content.Resize(content.Size().Add(fyne.NewSize(0, 1)))
				return
			}
			if err := AddAccount(name, balance); err != nil {
				log.Printf("Failed to add account: %v", err)
				return
			}
			// Refresh the view
			refresh()
		},
		parent,
	)
	d.Show()
}

func newBalanceLabel(balance float64, alignment fyne.TextAlign, textSize float32) *ui.TappableText {
	text := fmt.Sprintf("%.2f", balance)
	var textColor color.Color = color.White
	if balance < 0 {
		textColor = color.RGBA{R: 255, G: 0, B: 0, A: 255} // Red
	}
	return ui.NewTappableText(text, textColor, textSize, nil, alignment)
}

type ledgerLayout struct{}

func (l *ledgerLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	// Calculate dynamic column widths based on available size and content needs
	padding := theme.Padding()

	// Define desired fixed widths for date, amount, balance
	// These are "ideal" widths; they'll be adjusted to respect MinSize and available space.
	idealDateWidth := float32(100)
	idealAmountWidth := float32(80)
	idealBalanceWidth := float32(80) // Adjusted to be the same as amount

	// Calculate actual widths ensuring minimum size and respecting ideal sizes
	dateColWidth := fyne.Max(objects[0].MinSize().Width, idealDateWidth)
	amountColWidth := fyne.Max(objects[2].MinSize().Width, idealAmountWidth)
	balanceColWidth := fyne.Max(objects[3].MinSize().Width, idealBalanceWidth)

	// Ensure total fixed width doesn't exceed available width, adjust proportionally if it does
	remainingSpaceForDesc := size.Width - dateColWidth - amountColWidth - balanceColWidth - (3 * padding)
	descColWidth := fyne.Max(objects[1].MinSize().Width, remainingSpaceForDesc)
	// Ensure descColWidth is not negative if remainingSpaceForDesc is too small
	descColWidth = fyne.Max(0, descColWidth)

	// Adjust widths if overallocation happens, shrinking description first
	totalCurrentWidth := dateColWidth + descColWidth + amountColWidth + balanceColWidth + (3 * padding)
	if totalCurrentWidth > size.Width {
		// Prioritize shrinking description if it's larger than its minimum
		if descColWidth > objects[1].MinSize().Width {
			descColWidth -= (totalCurrentWidth - size.Width)
			descColWidth = fyne.Max(objects[1].MinSize().Width, descColWidth) // Don't go below min
		}
		// If still too wide, then fixed columns might need proportional shrinking (more complex)
		// For now, let's assume desc can absorb most overflow or clip if absolutely necessary
	}

	// Recalculate positions
	xOffset := float32(0)

	// Date
	objects[0].Resize(fyne.NewSize(dateColWidth, size.Height))
	objects[0].Move(fyne.NewPos(xOffset, 0))
	xOffset += dateColWidth + padding

	// Description
	objects[1].Resize(fyne.NewSize(descColWidth, size.Height))
	objects[1].Move(fyne.NewPos(xOffset, 0))
	xOffset += descColWidth + padding

	// Amount
	objects[2].Resize(fyne.NewSize(amountColWidth, size.Height))
	objects[2].Move(fyne.NewPos(xOffset, 0))
	xOffset += amountColWidth + padding

	// Balance
	objects[3].Resize(fyne.NewSize(balanceColWidth, size.Height))
	objects[3].Move(fyne.NewPos(xOffset, 0))
}

func (l *ledgerLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	minWidth := float32(0)
	minHeight := float32(0)
	padding := theme.Padding()

	// Sum min widths of all objects plus padding for separation
	if len(objects) == 4 {
		minWidth += objects[0].MinSize().Width + padding // Date
		minWidth += objects[1].MinSize().Width + padding // Description
		minWidth += objects[2].MinSize().Width + padding // Amount
		minWidth += objects[3].MinSize().Width           // Balance
	}

	for _, o := range objects {
		minHeight = fyne.Max(minHeight, o.MinSize().Height)
	}
	return fyne.NewSize(minWidth, minHeight)
}

func createLedgerView(account Account, win fyne.Window, refresh func()) fyne.CanvasObject {
	balanceLabel := newBalanceLabel(account.CurrentBalance, fyne.TextAlignCenter, 28) // 2x size
	balanceLabel.Text.TextStyle = fyne.TextStyle{Bold: true}

	records, err := GetLedgerRecords(account.ID)
	if err != nil {
		log.Printf("Failed to get ledger records: %v", err)
		return widget.NewLabel("Failed to load records.")
	}

	var recordWidgets []fyne.CanvasObject
	for _, record := range records {
		rec := record                                         // Capture range variable
		amountColor := color.RGBA{R: 255, G: 0, B: 0, A: 255} // Red for debit
		if record.Type == database.Credit {
			amountColor = color.RGBA{R: 0, G: 255, B: 0, A: 255} // Green for credit
		}

		runningBalanceLabel := newBalanceLabel(record.Balance, fyne.TextAlignTrailing, 14) // Standard size
		runningBalanceLabel.OnTap = func() {
			showEditLedgerRecordDialog(win, rec, account, refresh)
		}

		recordWidgets = append(recordWidgets,
			container.New(&ledgerLayout{},
				ui.NewTappableText(record.Timestamp.Format("2006-01-02"), color.White, 14, func() {
					showEditLedgerRecordDialog(win, rec, account, refresh)
				}, fyne.TextAlignLeading),
				ui.NewTappableText(record.Description, color.White, 14, func() {
					showEditLedgerRecordDialog(win, rec, account, refresh)
				}, fyne.TextAlignLeading),
				ui.NewTappableText(fmt.Sprintf("%.2f", record.Amount), amountColor, 14, func() {
					showEditLedgerRecordDialog(win, rec, account, refresh)
				}, fyne.TextAlignTrailing), // Right-align amount
				runningBalanceLabel,
			),
		)
	}

	list := container.NewVScroll(container.NewVBox(recordWidgets...))
	addRecordButton := widget.NewButtonWithIcon("", theme.ContentAddIcon(), func() {
		showAddLedgerRecordDialog(win, account, refresh)
	})
	addRecordButton.Importance = widget.HighImportance

	return container.NewBorder(
		balanceLabel,
		addRecordButton,
		nil,
		nil,
		list,
	)
}

func showAddLedgerRecordDialog(parent fyne.Window, account Account, refresh func()) {
	descriptionEntry := ui.NewKeyboardEntry(parent)
	descriptionEntry.SetPlaceHolder("Description")

	amountEntry := ui.NewKeyboardEntry(parent)
	amountEntry.SetPlaceHolder("Amount")
	typeRadio := widget.NewRadioGroup([]string{"Credit", "Debit"}, nil)
	typeRadio.SetSelected("Debit")

	form := &widget.Form{
		Items: []*widget.FormItem{
			{Text: "Description", Widget: descriptionEntry},
			{Text: "Amount", Widget: amountEntry},
			{Text: "Type", Widget: typeRadio},
		},
	}

	errorText := canvas.NewText("", color.RGBA{R: 255, A: 255})
	errorText.TextStyle.Bold = true
	errorText.Hide()

	content := container.NewMax(
		canvas.NewRectangle(color.Transparent),
		container.NewVBox(form, errorText),
	)
	if rect, ok := content.Objects[0].(*canvas.Rectangle); ok {
		rect.SetMinSize(fyne.NewSize(400, 0))
	}

	d := dialogs.NewCustomConfirm("Add Ledger Record", "Add", "Cancel", content, func(b bool) {
		if !b { // If cancel is pressed
			return
		}
		if descriptionEntry.Text == "" {
			errorText.Text = "Description cannot be empty."
			errorText.Show()
			content.Resize(content.Size().Add(fyne.NewSize(0, 1)))
			return
		}
		if amountEntry.Text == "" {
			errorText.Text = "Amount cannot be empty."
			errorText.Show()
			content.Resize(content.Size().Add(fyne.NewSize(0, 1)))
			return
		}
		description := descriptionEntry.Text
		amount, err := strconv.ParseFloat(amountEntry.Text, 64)
		if err != nil {
			errorText.Text = "Invalid amount."
			errorText.Show()
			content.Resize(content.Size().Add(fyne.NewSize(0, 1)))
			return
		}
		recordType := database.LedgerRecordType(strings.ToLower(typeRadio.Selected))

		newBalance := account.CurrentBalance
		if recordType == database.Credit {
			newBalance += amount
		} else {
			newBalance -= amount
		}

		record := database.LedgerRecord{
			AccountID:   account.ID,
			Timestamp:   time.Now(),
			Description: description,
			Amount:      amount,
			Type:        recordType,
			Balance:     newBalance,
		}

		if _, err := AddLedgerRecord(record); err != nil {
			log.Printf("Failed to add ledger record: %v", err)
			return
		}

		account.CurrentBalance = newBalance
		if err := UpdateAccount(account); err != nil {
			log.Printf("Failed to update account balance: %v", err)
			return
		}

		// Refresh the view
		refresh()
	}, parent)
	d.Show()
}

func showEditLedgerRecordDialog(parent fyne.Window, record database.LedgerRecord, account Account, refresh func()) {
	descriptionEntry := ui.NewKeyboardEntry(parent)
	descriptionEntry.SetText(record.Description)

	amountEntry := ui.NewKeyboardEntry(parent)
	amountEntry.SetText(fmt.Sprintf("%.2f", record.Amount))
	typeRadio := widget.NewRadioGroup([]string{"Credit", "Debit"}, nil)
	typeRadio.SetSelected(strings.Title(string(record.Type)))

	form := &widget.Form{
		Items: []*widget.FormItem{
			{Text: "Description", Widget: descriptionEntry},
			{Text: "Amount", Widget: amountEntry},
			{Text: "Type", Widget: typeRadio},
		},
	}

	var editDialog dialog.Dialog

	deleteButton := widget.NewButton("Delete", func() {
		d := dialogs.NewCustomConfirm("Delete Record", "Yes", "No", widget.NewLabel("Are you sure you want to delete this record?"), func(confirm bool) {
			if !confirm {
				return
			}
			if err := DeleteLedgerRecord(record.ID); err != nil {
				log.Printf("Failed to delete ledger record: %v", err)
				return
			}
			if err := recalculateBalances(account.ID); err != nil {
				log.Printf("Failed to recalculate balances: %v", err)
			}
			// Refresh the view
			refresh()
			editDialog.Hide()
		}, parent)
		d.Show()
	})
	editDialog = dialogs.NewCustomConfirm(
		"Edit Ledger Record",
		"Update",
		"Cancel",
		container.NewVBox(form, deleteButton),
		func(b bool) {
			if !b {
				return
			}
			description := descriptionEntry.Text
			amount, err := strconv.ParseFloat(amountEntry.Text, 64)
			if err != nil {
				log.Printf("Invalid amount: %v", err)
				return
			}
			recordType := database.LedgerRecordType(strings.ToLower(typeRadio.Selected))

			// Recalculate and update
			originalAmount := record.Amount
			if record.Type == database.Debit {
				originalAmount = -originalAmount
			}
			newAmount := amount
			if recordType == database.Debit {
				newAmount = -newAmount
			}
			balanceDifference := newAmount - originalAmount

			record.Description = description
			record.Amount = amount
			record.Type = recordType
			record.Balance += balanceDifference

			if err := UpdateLedgerRecord(record); err != nil {
				log.Printf("Failed to update ledger record: %v", err)
				return
			}

			account.CurrentBalance += balanceDifference
			if err := UpdateAccount(account); err != nil {
				log.Printf("Failed to update account balance: %v", err)
				return
			}

			if err := recalculateBalances(account.ID); err != nil {
				log.Printf("Failed to recalculate balances: %v", err)
			}

			// Refresh the view
			refresh()
		},
		parent,
	)
	editDialog.Show()
}

func showLedgerSettingsDialog(parent fyne.Window, account Account, refresh func()) {
	nameEntry := ui.NewKeyboardEntry(parent)
	nameEntry.SetText(account.Name)

	form := &widget.Form{
		Items: []*widget.FormItem{
			{Text: "Ledger Name", Widget: nameEntry},
		},
	}

	var settingsDialog dialog.Dialog

	deleteButton := widget.NewButton("Delete Ledger", func() {
		d := dialogs.NewCustomConfirm("Delete Ledger", "Yes", "No", widget.NewLabel("Are you sure you want to delete this ledger and all its records?"), func(confirm bool) {
			if !confirm {
				return
			}
			records, err := GetLedgerRecords(account.ID)
			if err != nil {
				log.Printf("Failed to get ledger records for deletion: %v", err)
				return
			}
			for _, record := range records {
				if err := DeleteLedgerRecord(record.ID); err != nil {
					log.Printf("Failed to delete ledger record: %v", err)
					return
				}
			}
			if err := DeleteAccount(account.ID); err != nil {
				log.Printf("Failed to delete account: %v", err)
				return
			}
			// Refresh the view
			refresh()
			settingsDialog.Hide()
		}, parent)
		d.Show()
	})

	settingsDialog = dialogs.NewCustomConfirm(
		"Ledger Settings",
		"Save",
		"Cancel",
		container.NewVBox(form, deleteButton),
		func(b bool) {
			if !b {
				return
			}
			newName := nameEntry.Text
			if newName != account.Name {
				account.Name = newName
				if err := UpdateAccount(account); err != nil {
					log.Printf("Failed to update account name: %v", err)
					return
				}
				// Refresh the view
				refresh()
			}
		},
		parent,
	)
	settingsDialog.Show()
}

func recalculateBalances(accountID int) error {
	records, err := GetLedgerRecords(accountID)
	if err != nil {
		return err
	}

	// It's easier to recalculate from the start.
	// For this, we need the initial balance.
	accounts, err := GetAccounts()
	if err != nil {
		return err
	}
	var initialBalance float64
	for _, acc := range accounts {
		if acc.ID == accountID {
			initialBalance = acc.InitialBalance
			break
		}
	}

	sort.Slice(records, func(i, j int) bool {
		return records[i].Timestamp.Before(records[j].Timestamp)
	})

	currentBalance := initialBalance
	for i := range records {
		if records[i].Type == database.Credit {
			currentBalance += records[i].Amount
		} else {
			currentBalance -= records[i].Amount
		}
		records[i].Balance = currentBalance
		if err := UpdateLedgerRecord(records[i]); err != nil {
			log.Printf("Failed to update record balance: %v", err)
		}
	}

	// Update the account's current balance
	for i := range accounts {
		if accounts[i].ID == accountID {
			accounts[i].CurrentBalance = currentBalance
			if err := UpdateAccount(accounts[i]); err != nil {
				return err
			}
			break
		}
	}

	return nil
}
