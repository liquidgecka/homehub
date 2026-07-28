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

package home

import (
	"context"
	"fmt"
	"log"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"

	"github.com/liquidgecka/homehub/photomanager"
	"github.com/liquidgecka/homehub/reminders"
	"github.com/liquidgecka/homehub/ui"
)

// CreateView initializes the Fyne UI elements for the home view.
// It returns the main view canvas object and references to the dynamic UI elements.
func CreateView() (fyne.CanvasObject, *canvas.Image, *widget.Label, *ui.TappableIcon, *ui.TappableIcon) {
	img := canvas.NewImageFromFile("")
	img.FillMode = canvas.ImageFillContain
	img.Hide()

	label := widget.NewLabel("Loading photos...")
	label.Alignment = fyne.TextAlignCenter

	heartOutlineIcon := fyne.NewStaticResource("heart_outline.svg", ui.MustLoadFile(ui.GetIconPath("heart_outline.svg")))
	heartButton := ui.NewTappableIcon(heartOutlineIcon, nil)
	heartButton.Hide()

	thumbDownOutlineIcon := fyne.NewStaticResource("thumb_down_outline.svg", ui.MustLoadFile(ui.GetIconPath("thumb_down_outline.svg")))
	hideButton := ui.NewTappableIcon(thumbDownOutlineIcon, nil) // Initially use the outline icon
	hideButton.Hide()

	bottomButtonContainer := container.NewHBox(
		container.NewPadded(hideButton),
		layout.NewSpacer(),
		container.NewPadded(heartButton),
	)

	remindersOverlay := reminders.CreatePhotoOverlayView()

	view := container.NewMax(
		img,
		container.NewCenter(label),
		container.NewBorder(nil, bottomButtonContainer, nil, nil, nil),
		remindersOverlay,
	)

	return view, img, label, heartButton, hideButton
}

// StartSlideshowAndPhotoListener initializes and manages the slideshow logic and UI updates.
func StartSlideshowAndPhotoListener(
	img *canvas.Image,
	label *widget.Label,
	heartButton *ui.TappableIcon,
	hideButton *ui.TappableIcon,
	sm *SlideshowManager,
) context.CancelFunc {
	heartOutlineIcon := fyne.NewStaticResource("heart_outline.svg", ui.MustLoadFile(ui.GetIconPath("heart_outline.svg")))
	heartIcon := fyne.NewStaticResource("heart.svg", ui.MustLoadFile(ui.GetIconPath("heart.svg")))
	thumbDownOutlineIcon := fyne.NewStaticResource("thumb_down_outline.svg", ui.MustLoadFile(ui.GetIconPath("thumb_down_outline.svg")))
	thumbDownIcon := fyne.NewStaticResource("thumb_down.svg", ui.MustLoadFile(ui.GetIconPath("thumb_down.svg")))

	var currentPath string

	// UI update goroutine
	go func() {
		defer log.Println("Home view UI update goroutine terminated.")
		var lastLoadedPath string
		var lastLoadedRes fyne.Resource

		// Create a buffered channel for loading tasks
		type loadTask struct {
			state SlideshowState
		}
		tasks := make(chan loadTask, 10)

		// Start a single worker goroutine to load images sequentially
		go func() {
			for {
				select {
				case <-sm.ctx.Done():
					return
				case task := <-tasks:
					s := task.state
					var loadedRes fyne.Resource
					if s.ImagePath != lastLoadedPath || lastLoadedRes == nil {
						log.Printf("TRACE: Image Loader Goroutine: Calling LoadImageSafely for %s.", s.ImagePath)
						loadedRes = photomanager.LoadImageSafely(s.ImagePath)
						log.Printf("TRACE: Image Loader Goroutine: Finished LoadImageSafely for %s.", s.ImagePath)
						if loadedRes != nil && loadedRes.Content() != nil {
							lastLoadedPath = s.ImagePath
							lastLoadedRes = loadedRes
						} else {
							log.Printf("Warning: Failed to load image safely for %s. Using placeholder.", s.ImagePath)
							loadedRes = fyne.NewStaticResource("placeholder.png", []byte{})
						}
					} else {
						loadedRes = lastLoadedRes
					}

					fyne.Do(func() {
						currentPath = s.ImagePath
						label.Hide()
						img.Show()
						heartButton.Show()
						hideButton.Show()

						if img.Resource != loadedRes {
							img.Resource = loadedRes
							img.Refresh()
						}

						if s.IsFavorite {
							heartButton.SetResource(heartIcon)
						} else {
							heartButton.SetResource(heartOutlineIcon)
						}

						if s.IsHidden {
							hideButton.SetResource(thumbDownIcon)
						} else {
							hideButton.SetResource(thumbDownOutlineIcon)
						}
					})
				}
			}
		}()

		for {
			select {
			case <-sm.ctx.Done(): // Listen for cancellation from SlideshowManager
				lastLoadedPath = "" // Clear cached path on exit
				lastLoadedRes = nil // Clear cached resource on exit
				return
			case state := <-sm.StateChan:
				// Drain tasks channel to skip outdated states if we got behind
				for len(tasks) > 0 {
					<-tasks
				}
				tasks <- loadTask{state: state}
			case <-sm.NoPhotosChan:
				fyne.Do(func() {
					photoDir := sm.cfg.Directory
					if photoDir == "" {
						photoDir = "photos/"
					}
					label.SetText(fmt.Sprintf("No photos found in '%s'.", photoDir))
					label.Show()
					img.Hide()
					heartButton.Hide()
					hideButton.Hide()
				})
			}
		}
	}()

	// Bind UI actions to SlideshowManager methods
	heartButton.OnTapped = func() {
		if currentPath == "" {
			return
		}
		// Optimistic update
		if heartButton.Resource.Name() == "heart_outline.svg" {
			heartButton.SetResource(heartIcon)
		} else {
			heartButton.SetResource(heartOutlineIcon)
		}
		// Pass currentImagePath to manager for accurate state management
		go func() {
			sm.ToggleFavorite(currentPath)
		}()
	}
	hideButton.OnTapped = func() {
		if currentPath == "" {
			return
		}
		// Optimistic update
		if hideButton.Resource.Name() == "thumb_down_outline.svg" {
			hideButton.SetResource(thumbDownIcon)
		} else {
			hideButton.SetResource(thumbDownOutlineIcon)
		}
		// Pass currentImagePath to manager for accurate state management
		go func() {
			sm.ToggleHidden(currentPath)
		}()
	}

	return sm.Stop // Return the stop function of the manager
}
