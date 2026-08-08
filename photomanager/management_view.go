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
package photomanager

import (
	"log"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/liquidgecka/homehub/config"
	"github.com/liquidgecka/homehub/ui"
)

// photoListItem is a custom widget for displaying a photo's filename and management buttons.
type photoListItem struct {
	widget.BaseWidget
	path       string
	thumbnail  *canvas.Image
	filename   *widget.Label
	favButton  *widget.Button
	hideButton *widget.Button

	container *fyne.Container
}

func (i *photoListItem) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(i.container)
}

func newPhotoListItem() *photoListItem {
	thumbImg := canvas.NewImageFromResource(nil)
	thumbImg.SetMinSize(fyne.NewSize(60, 60))
	thumbImg.FillMode = canvas.ImageFillContain

	item := &photoListItem{
		thumbnail:  thumbImg,
		filename:   widget.NewLabel(""),
		favButton:  widget.NewButtonWithIcon("", fyne.NewStaticResource("heart_outline.svg", ui.MustLoadFile(ui.GetIconPath("heart_outline.svg"))), nil),
		hideButton: widget.NewButtonWithIcon("", fyne.NewStaticResource("thumb_down_outline.svg", ui.MustLoadFile(ui.GetIconPath("thumb_down_outline.svg"))), nil),
	}
	item.container = container.NewBorder(
		nil, nil, item.thumbnail, // top, bottom, left
		container.NewHBox(item.favButton, item.hideButton), // right
		item.filename, // center
	)
	item.ExtendBaseWidget(item)
	return item
}

func showDetailView(imageList []string, startIndex int, onUpdate func()) {
	if len(imageList) == 0 || startIndex < 0 || startIndex >= len(imageList) {
		return
	}

	win := fyne.CurrentApp().NewWindow("Photo Viewer")
	win.SetPadded(true)
	currentIndex := startIndex

	img := canvas.NewImageFromFile(imageList[currentIndex])
	img.FillMode = canvas.ImageFillContain

	heartOutlineIcon := fyne.NewStaticResource("heart_outline.svg", ui.MustLoadFile(ui.GetIconPath("heart_outline.svg")))
	heartIcon := fyne.NewStaticResource("heart.svg", ui.MustLoadFile(ui.GetIconPath("heart.svg")))
	thumbDownOutlineIcon := fyne.NewStaticResource("thumb_down_outline.svg", ui.MustLoadFile(ui.GetIconPath("thumb_down_outline.svg")))

	favButton := widget.NewButtonWithIcon("", heartOutlineIcon, nil)
	hideButton := widget.NewButtonWithIcon("", thumbDownOutlineIcon, nil)
	backButton := widget.NewButtonWithIcon("", theme.NavigateBackIcon(), nil)
	forwardButton := widget.NewButtonWithIcon("", theme.NavigateNextIcon(), nil)
	closeButton := widget.NewButton("Close", func() {
		onUpdate() // Refresh the main list when the detail view is closed
		win.Close()
	})
	win.SetOnClosed(onUpdate)

	var updateAll func()
	updateAll = func() {
		path := imageList[currentIndex]
		baseFilename := filepath.Base(path)
		win.SetTitle(baseFilename)
		img.File = path
		img.Refresh()

		isFav := IsPhotoFavorite(baseFilename)
		isHidden := IsPhotoHidden(baseFilename)

		if isHidden {
			favButton.Disable()
			hideButton.SetIcon(theme.ContentUndoIcon())
		} else {
			favButton.Enable()
			hideButton.SetIcon(thumbDownOutlineIcon)
			if isFav {
				favButton.SetIcon(heartIcon)
				favButton.Importance = widget.HighImportance
			} else {
				favButton.SetIcon(heartOutlineIcon)
				favButton.Importance = widget.LowImportance
			}
		}

		if currentIndex == 0 {
			backButton.Disable()
		} else {
			backButton.Enable()
		}
		if currentIndex == len(imageList)-1 {
			forwardButton.Disable()
		} else {
			forwardButton.Enable()
		}

		favButton.Refresh()
		hideButton.Refresh()
		backButton.Refresh()
		forwardButton.Refresh()
	}

	favButton.OnTapped = func() {
		path := imageList[currentIndex]
		SetPhotoFavorite(filepath.Base(path), !IsPhotoFavorite(filepath.Base(path)))
		updateAll()
	}
	hideButton.OnTapped = func() {
		path := imageList[currentIndex]
		SetPhotoHidden(filepath.Base(path), !IsPhotoHidden(filepath.Base(path)))
		updateAll()
	}
	backButton.OnTapped = func() {
		if currentIndex > 0 {
			currentIndex--
			updateAll()
		}
	}
	forwardButton.OnTapped = func() {
		if currentIndex < len(imageList)-1 {
			currentIndex++
			updateAll()
		}
	}

	controlBar := container.NewHBox(backButton, forwardButton, layout.NewSpacer(), favButton, hideButton, layout.NewSpacer(), closeButton)
	content := container.NewBorder(nil, controlBar, nil, nil, img)
	win.SetContent(content)

	updateAll()
	win.Resize(fyne.NewSize(800, 600))
	win.Show()
}

func CreateManagementView(win fyne.Window) fyne.CanvasObject {
	allImagePaths, err := ListLocalPhotos(config.GetConfig().LocalPhotos.Directory)
	if err != nil {
		return container.NewCenter(widget.NewLabel("Error loading photos: " + err.Error()))
	}
	sort.Strings(allImagePaths)

	// --- Metadata Cache ---
	photoMetadataCache := make(map[string]time.Time)
	for _, path := range allImagePaths {
		date, err := GetCreationDate(path)
		if err != nil {
			log.Printf("Could not get metadata for %s: %v", path, err)
			// Use a zero time if metadata is unavailable
			photoMetadataCache[path] = time.Time{}
		} else {
			photoMetadataCache[path] = date
		}
	}
	// --- End Metadata Cache ---

	filteredImagePaths := allImagePaths
	var list *widget.List

	searchEntry := ui.NewKeyboardEntry(win)
	searchEntry.SetPlaceHolder("Search: name, is:fav, is:hidden, date:YYYY-MM-DD, date-after:..., date-before:...")
	searchEntry.OnChanged = func(text string) {
		query := strings.ToLower(text)
		filteredImagePaths = []string{}

		var startDate, endDate time.Time
		var plainQueryParts []string

		// --- Parse Query ---
		parts := strings.Split(query, " ")
		for _, part := range parts {
			if strings.HasPrefix(part, "date:") {
				dateStr := strings.TrimPrefix(part, "date:")
				if strings.Contains(dateStr, "..") { // Date range
					dateParts := strings.Split(dateStr, "..")
					startDate, _ = time.Parse("2006-01-02", dateParts[0])
					endDate, _ = time.Parse("2006-01-02", dateParts[1])
					// To make the end date inclusive, set it to the end of the day
					if !endDate.IsZero() {
						endDate = endDate.Add(23*time.Hour + 59*time.Minute)
					}
				} else { // Single date
					singleDate, err := time.Parse("2006-01-02", dateStr)
					if err == nil {
						startDate = singleDate
						endDate = singleDate.Add(23*time.Hour + 59*time.Minute)
					}
				}
			} else if strings.HasPrefix(part, "date-after:") {
				dateStr := strings.TrimPrefix(part, "date-after:")
				startDate, _ = time.Parse("2006-01-02", dateStr)
			} else if strings.HasPrefix(part, "date-before:") {
				dateStr := strings.TrimPrefix(part, "date-before:")
				endDate, _ = time.Parse("2006-01-02", dateStr)
				if !endDate.IsZero() {
					endDate = endDate.Add(23*time.Hour + 59*time.Minute)
				}
			} else if part != "is:favorite" && part != "is:hidden" {
				plainQueryParts = append(plainQueryParts, part)
			}
		}
		plainQuery := strings.Join(plainQueryParts, " ")
		// --- End Parse Query ---

		for _, path := range allImagePaths {
			baseFilename := filepath.Base(path)
			matches := true

			if strings.Contains(query, "is:favorite") && !IsPhotoFavorite(baseFilename) {
				matches = false
			}
			if strings.Contains(query, "is:hidden") && !IsPhotoHidden(baseFilename) {
				matches = false
			}

			// Date range check
			photoDate := photoMetadataCache[path]
			if !startDate.IsZero() && photoDate.Before(startDate) {
				matches = false
			}
			if !endDate.IsZero() && photoDate.After(endDate) {
				matches = false
			}

			// Plain text search
			if plainQuery != "" && !strings.Contains(strings.ToLower(baseFilename), plainQuery) {
				matches = false
			}

			if matches {
				filteredImagePaths = append(filteredImagePaths, path)
			}
		}
		list.Refresh()
	}

	showThumbnails := true
	showThumbnailsCheck := widget.NewCheck("Thumbnails", func(checked bool) {
		showThumbnails = checked
		if list != nil {
			list.Refresh()
		}
	})
	showThumbnailsCheck.SetChecked(true)

	list = widget.NewList(
		func() int {
			return len(filteredImagePaths)
		},
		func() fyne.CanvasObject {
			return newPhotoListItem()
		},
		func(i widget.ListItemID, o fyne.CanvasObject) {
			item := o.(*photoListItem)
			path := filteredImagePaths[i]
			baseFilename := filepath.Base(path)

			item.path = path
			item.filename.SetText(baseFilename)

			if showThumbnails {
				thumbBytes, err := GenerateThumbnail(path, 80)
				if err == nil && len(thumbBytes) > 0 {
					item.thumbnail.Resource = fyne.NewStaticResource(baseFilename, thumbBytes)
					item.thumbnail.Show()
				} else {
					item.thumbnail.Hide()
				}
			} else {
				item.thumbnail.Hide()
			}

			isFav := IsPhotoFavorite(baseFilename)
			isHidden := IsPhotoHidden(baseFilename)

			if isHidden {
				item.favButton.Hide()
				item.hideButton.SetIcon(theme.ContentUndoIcon())
			} else {
				item.favButton.Show()
				item.hideButton.SetIcon(fyne.NewStaticResource("thumb_down_outline.svg", ui.MustLoadFile(ui.GetIconPath("thumb_down_outline.svg"))))
				if isFav {
					item.favButton.SetIcon(fyne.NewStaticResource("heart.svg", ui.MustLoadFile(ui.GetIconPath("heart.svg"))))
					item.favButton.Importance = widget.HighImportance
				} else {
					item.favButton.SetIcon(fyne.NewStaticResource("heart_outline.svg", ui.MustLoadFile(ui.GetIconPath("heart_outline.svg"))))
					item.favButton.Importance = widget.LowImportance
				}
			}

			item.favButton.OnTapped = func() {
				if IsPhotoHidden(baseFilename) {
					return
				}
				SetPhotoFavorite(baseFilename, !isFav)
				list.Refresh()
			}

			item.hideButton.OnTapped = func() {
				SetPhotoHidden(baseFilename, !isHidden)
				list.Refresh()
			}

			item.Refresh()
		},
	)

	list.OnSelected = func(id widget.ListItemID) {
		showDetailView(filteredImagePaths, id, func() {
			searchEntry.OnChanged(searchEntry.Text)
		})
		list.Unselect(id)
	}

	topBar := container.NewBorder(nil, nil, nil, showThumbnailsCheck, searchEntry)
	return container.NewBorder(topBar, nil, nil, nil, list)
}
