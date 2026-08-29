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

package shopping

import (
	"fmt"
	"image/color"
	"log"
	"strconv"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/liquidgecka/homehub/config"
	"github.com/liquidgecka/homehub/database"
	"github.com/liquidgecka/homehub/dialogs"
	"github.com/liquidgecka/homehub/ui"
)

var selectedStoreID int = -1 // Default to "All Stores"

type ShoppingView struct {
	win               fyne.Window
	content           fyne.CanvasObject
	tabs              []*widget.Button
	storeIDs          []int
	mainContent       *fyne.Container
	currentTabContent *fyne.Container
}

func NewShoppingView(
	win fyne.Window, mainContent *fyne.Container,
) *ShoppingView {
	v := &ShoppingView{
		win:         win,
		mainContent: mainContent,
	}
	v.makeUI()
	return v
}

func (v *ShoppingView) Content() fyne.CanvasObject {
	return v.content
}

func (v *ShoppingView) Refresh() {
	v.makeUI()
	v.mainContent.Objects = []fyne.CanvasObject{v.content}
	v.mainContent.Refresh()
}

func (v *ShoppingView) makeUI() {
	cfg := config.GetConfig()

	// Pre-calculate unchecked item counts
	allItems, err := database.GetShoppingItems()
	if err != nil {
		log.Printf("Failed to get all shopping items for count: %v", err)
	}
	uncheckedCounts := make(map[int]int)
	totalUnchecked := 0
	for _, item := range allItems {
		if !item.Checked {
			uncheckedCounts[item.StoreID]++
			totalUnchecked++
		}
	}

	v.tabs = nil
	v.storeIDs = nil
	v.currentTabContent = container.New(layout.NewMaxLayout())

	// Initial selection logic - default to "All Stores"
	activeTabIndex := 0
	if selectedStoreID != -1 {
		for i, store := range cfg.Shopping.Store {
			if !store.Disabled && i+1 == selectedStoreID {
				activeTabIndex = i + 1 // +1 because of "All Stores" tab
				break
			}
		}
	}

	// Add "All Stores" tab button
	v.tabs = append(
		v.tabs,
		widget.NewButton(fmt.Sprintf("All Stores (%d)", totalUnchecked), nil),
	)
	v.storeIDs = append(v.storeIDs, -1) // -1 for "All Stores"

	for i, store := range cfg.Shopping.Store {
		if store.Disabled {
			continue // Skip disabled devices
		}
		storeID := i + 1
		count := uncheckedCounts[storeID]
		tabText := fmt.Sprintf("%s (%d)", store.Name, count)

		var tabButton *widget.Button
		if store.Icon != "" {
			iconResource, err := fyne.LoadResourceFromPath(store.Icon)
			if err != nil {
				log.Printf(
					"Failed to load store icon from %s: %v",
					store.Icon, err,
				)
				tabButton = widget.NewButton(tabText, nil)
			} else {
				tabButton = widget.NewButtonWithIcon(tabText, iconResource, nil)
			}
		} else {
			tabButton = widget.NewButton(tabText, nil)
		}
		v.tabs = append(v.tabs, tabButton)
		v.storeIDs = append(v.storeIDs, storeID)
	}

	// Re-validate activeTabIndex
	if activeTabIndex >= len(v.storeIDs) {
		activeTabIndex = 0 // Fallback to "All Stores"
	}

	// Convert []*widget.Button to []fyne.CanvasObject for NewHBox
	tabButtonsAsCanvasObjects := make([]fyne.CanvasObject, len(v.tabs))
	for i, btn := range v.tabs {
		tabButtonsAsCanvasObjects[i] = btn
	}

	// Create a horizontally scrollable container for the tab buttons
	tabBar := container.NewScroll(
		container.NewHBox(tabButtonsAsCanvasObjects...),
	)

	// Update the initially selected tab button's visual
	if len(v.tabs) > 0 {
		v.tabs[activeTabIndex].Importance = widget.HighImportance
	}

	// Implement OnTapped logic for each tab button
	for i, btn := range v.tabs {
		idx := i // Capture loop variable
		btn.OnTapped = func() {
			// Reset all buttons to normal importance
			for _, b := range v.tabs {
				b.Importance = widget.LowImportance
				b.Refresh()
			}
			// Set active button to high importance
			btn.Importance = widget.HighImportance
			btn.Refresh()

			selectedStoreID = v.storeIDs[idx]

			// Refresh the content for the newly selected tab
			v.displayItemsForStore(selectedStoreID, v.currentTabContent)
		}
	}

	// Initial display of items for the selected tab
	v.displayItemsForStore(v.storeIDs[activeTabIndex], v.currentTabContent)

	// The overall layout for the shopping view
	// The space for the store tabs should extend all the way across the page
	// without any add or settings buttons.
	finalTopBar := container.NewBorder(
		nil,    // Top
		nil,    // Bottom
		nil,    // Left (no left element)
		nil,    // Right (no action buttons)
		tabBar, // Center (scrollable tab buttons, fills remaining space)
	)

	// The overall layout for the shopping view
	addButton := widget.NewButtonWithIcon("", theme.ContentAddIcon(), func() {
		v.showAddItemDialog()
	})
	// Force the button to use HighImportance for blue background
	addButton.Importance = widget.HighImportance

	// Create a container with a black background that wraps the button
	blackBackgroundContainer := container.New(
		layout.NewMaxLayout(),
		canvas.NewRectangle(color.Black), // Black background
		addButton,
	)

	v.content = container.NewBorder(
		finalTopBar,              // Top contains the tab buttons and action buttons
		blackBackgroundContainer, // Bottom for the Add Item button
		nil,                      // Left
		nil,                      // Right
		v.currentTabContent,      // Center contains the actual shopping list content
	)
}

func CreateShoppingView(
	win fyne.Window, mainContent *fyne.Container,
) (fyne.CanvasObject, func()) {
	// Trigger a sync when the view is created.
	go syncAllStores()
	v := NewShoppingView(win, mainContent)
	return v.Content(), nil
}

// This function recalculates unchecked item counts and updates tab buttons
func (v *ShoppingView) recalculateAndRefreshTabs(
	uncheckedCounts map[int]int, totalUnchecked int, activeTabIndex int,
) {
	cfg := config.GetConfig()
	// Re-calculate unchecked item counts
	allItems, err := database.GetShoppingItems()
	if err != nil {
		log.Printf("Failed to get all shopping items for count: %v", err)
		return
	}
	uncheckedCounts = make(map[int]int) // Reset
	totalUnchecked = 0
	for _, item := range allItems {
		if !item.Checked {
			uncheckedCounts[item.StoreID]++
			totalUnchecked++
		}
	}

	// Update "All Stores" tab button text
	v.tabs[0].SetText(fmt.Sprintf("All Stores (%d)", totalUnchecked))
	v.tabs[0].Refresh()

	// Update individual store tab button texts
	storeButtonIdx := 1 // Start from index 1 for actual stores
	for i, store := range cfg.Shopping.Store {
		if store.Disabled {
			continue
		}
		count := uncheckedCounts[i+1] // Store IDs are 1-based
		v.tabs[storeButtonIdx].SetText(
			fmt.Sprintf("%s (%d)", store.Name, count),
		)
		v.tabs[storeButtonIdx].Refresh()
		storeButtonIdx++
	}

	// Re-highlight the active tab if necessary
	if len(v.tabs) > 0 {
		for i, btn := range v.tabs {
			if i == activeTabIndex {
				btn.Importance = widget.HighImportance
			} else {
				btn.Importance = widget.LowImportance
			}
			btn.Refresh()
		}
	}
}

func (v *ShoppingView) displayItemsForStore(
	storeID int, contentContainer *fyne.Container,
) {
	contentContainer.Objects = nil // Clear the container
	items, err := database.GetShoppingItemsByStore(storeID)
	if err != nil {
		contentContainer.Objects = []fyne.CanvasObject{
			widget.NewLabel(
				fmt.Sprintf("Error fetching shopping items: %v", err),
			),
		}
		contentContainer.Refresh()
		return
	}

	list := widget.NewList(
		func() int {
			return len(items)
		},
		func() fyne.CanvasObject {
			return container.NewHBox(
				widget.NewCheck("", func(bool) {}),
				ui.NewTappableText(
					"", color.White, 18, nil, fyne.TextAlignLeading,
				),
			)
		},
		func(i widget.ListItemID, o fyne.CanvasObject) {
			item := items[i]
			hbox := o.(*fyne.Container)
			check := hbox.Objects[0].(*widget.Check)
			label := hbox.Objects[1].(*ui.TappableText)

			check.SetChecked(item.Checked)
			check.OnChanged = func(checked bool) {
				item.Checked = checked
				if err := database.UpdateShoppingItem(item); err != nil {
					log.Printf("Failed to update shopping item: %v", err)
				}
			}

			cfg := config.GetConfig()
			text := fmt.Sprintf("%d x %s", item.Quantity, item.Name)
			if storeID == -1 {
				for i, store := range cfg.Shopping.Store {
					if i+1 == item.StoreID {
						text = fmt.Sprintf("%s (%s)", text, store.Name)
						break
					}
				}
			}
			label.Text.Text = text
			label.OnTap = func() {
				v.showEditShoppingItemDialog(item)
			}
			label.Refresh()
		},
	)

	if len(items) == 0 {
		contentContainer.Objects = []fyne.CanvasObject{
			container.New(
				layout.NewMaxLayout(),
				widget.NewLabel("No items in list. Add some!"),
			),
		}
	} else {
		contentContainer.Objects = []fyne.CanvasObject{
			container.New(layout.NewMaxLayout(), list),
		}
	}
	contentContainer.Refresh()
}

func (v *ShoppingView) showEditShoppingItemDialog(
	item database.ShoppingItem,
) {
	itemNameEntry := ui.NewKeyboardEntry(v.win)
	itemNameEntry.SetText(item.Name)

	quantityCounter := ui.NewKeyboardEntry(v.win)
	quantityCounter.SetText(strconv.Itoa(item.Quantity))

	form := &widget.Form{
		Items: []*widget.FormItem{
			widget.NewFormItem("Item", itemNameEntry),
			widget.NewFormItem("Quantity", quantityCounter),
		},
	}

	var editDialog dialog.Dialog // Declare editDialog here
	deleteButton := widget.NewButton("Delete", func() {
		msg := "Are you sure you want to delete this item?"
		dialogs.ShowCustomConfirm(
			"Delete Item", "Yes", "No",
			widget.NewLabel(msg),
			func(confirm bool) {
				if !confirm {
					return
				}
				if err := database.DeleteShoppingItem(item.ID); err != nil {
					log.Printf("Failed to delete shopping item: %v", err)
					return
				}
				v.Refresh()
			},
			v.win,
		)
	})

	editDialog = dialogs.NewCustomConfirm(
		"Edit Shopping Item",
		"Update",
		"Cancel",
		container.NewVBox(form, deleteButton),
		func(b bool) {
			if !b {
				return
			}
			name := itemNameEntry.Text
			qty, err := strconv.Atoi(quantityCounter.Text)
			if err != nil {
				log.Printf("Invalid quantity: %v", err)
				return
			}
			if name != "" && qty > 0 {
				item.Name = name
				item.Quantity = qty
				if err := database.UpdateShoppingItem(item); err != nil {
					log.Printf("Failed to update shopping item: %v", err)
					return
				}
				v.Refresh()
			}
		},
		v.win,
	)
	editDialog.Show()
}

// showAddItemDialog creates and displays a dialog for adding a new shopping
// item.
func (v *ShoppingView) showAddItemDialog() {
	itemNameEntry := ui.NewKeyboardEntry(v.win)
	itemNameEntry.SetPlaceHolder("Item Name")

	quantityCounter := ui.NewKeyboardEntry(v.win) // Use Entry for counter
	quantityCounter.SetPlaceHolder("Quantity")
	quantityCounter.SetText("1") // Default quantity

	var storeSelect *widget.Select
	formItems := []*widget.FormItem{
		widget.NewFormItem("Item", itemNameEntry),
		widget.NewFormItem("Quantity", quantityCounter),
	}
	if selectedStoreID == -1 {
		storeNames := []string{}
		for _, store := range config.GetConfig().Shopping.Store {
			if !store.Disabled {
				storeNames = append(storeNames, store.Name)
			}
		}
		storeSelect = widget.NewSelect(storeNames, nil)
		formItems = append(formItems, widget.NewFormItem("Store", storeSelect))
	}

	errorText := canvas.NewText("", color.RGBA{R: 255, A: 255})
	errorText.TextStyle.Bold = true
	errorText.Hide() // Initially hidden

	form := &widget.Form{
		Items: formItems,
	}

	var d dialog.Dialog
	d = dialogs.NewCustomConfirm(
		"Add New Shopping Item",
		"Add",
		"Cancel",
		container.NewVBox(form, errorText),
		func(b bool) {
			if !b {
				return
			}
			// Clear previous errors
			errorText.Hide()
			errorText.Text = ""

			name := itemNameEntry.Text
			qty, err := strconv.Atoi(quantityCounter.Text)
			if err != nil {
				errorText.Text = fmt.Sprintf("Invalid quantity: %v", err)
				errorText.Show()
				return // Don't hide the dialog
			}

			storeID := selectedStoreID
			if selectedStoreID == -1 {
				if storeSelect != nil && storeSelect.Selected == "" {
					errorText.Text = "Please select a store"
					errorText.Show()
					return // Don't hide the dialog
				}
				for i, store := range config.GetConfig().Shopping.Store {
					if store.Name == storeSelect.Selected {
						storeID = i + 1
						break
					}
				}
			}

			if name == "" { // Basic validation for item name
				errorText.Text = "Item name cannot be empty"
				errorText.Show()
				return // Don't hide the dialog
			}

			if name != "" && qty > 0 {
				item := database.ShoppingItem{
					Name: name, Quantity: qty, Checked: false, StoreID: storeID,
				}
				if err := AddItem(item); err != nil {
					errorText.Text = fmt.Sprintf(
						"Failed to add shopping item: %v", err,
					)
					errorText.Show()
					return // Don't hide the dialog
				}
				// Refresh the entire shopping view to update counts
				v.Refresh()
			}
		},
		v.win,
	)
	d.Show()
}
