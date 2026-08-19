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
	"image"
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
	var isFavorite bool
	var isHidden bool

	type loadTask struct {
		ImagePath  string
		IsFavorite bool
		IsHidden   bool
	}
	tasks := make(chan loadTask, 10)

	// Start a single worker goroutine to load and decode images sequentially off the UI thread
	go func() {
		defer log.Println("Home view image loader goroutine terminated.")
		var lastDecodedPath string
		var lastDecodedImg image.Image

		for {
			select {
			case <-sm.ctx.Done():
				return
			case task := <-tasks:
				var decodedImg image.Image
				if task.ImagePath == lastDecodedPath && lastDecodedImg != nil {
					decodedImg = lastDecodedImg
				} else {
					var err error
					decodedImg, err = photomanager.LoadDecodedImage(task.ImagePath)
					if err != nil {
						log.Printf("Warning: Failed to decode image for %s: %v. Falling back to LoadImageSafely.", task.ImagePath, err)
						loadedRes := photomanager.LoadImageSafely(task.ImagePath)
						fyne.Do(func() {
							currentPath = task.ImagePath
							isFavorite = task.IsFavorite
							isHidden = task.IsHidden
							label.Hide()
							img.Image = nil
							img.Resource = loadedRes
							img.Show()
							heartButton.Show()
							hideButton.Show()

							if isFavorite {
								heartButton.SetResource(heartIcon)
							} else {
								heartButton.SetResource(heartOutlineIcon)
							}
							if isHidden {
								hideButton.SetResource(thumbDownIcon)
							} else {
								hideButton.SetResource(thumbDownOutlineIcon)
							}
							img.Refresh()
						})
						continue
					}
					lastDecodedPath = task.ImagePath
					lastDecodedImg = decodedImg
				}

				fyne.Do(func() {
					currentPath = task.ImagePath
					isFavorite = task.IsFavorite
					isHidden = task.IsHidden
					label.Hide()
					img.Resource = nil
					img.Image = decodedImg
					img.Show()
					heartButton.Show()
					hideButton.Show()

					if isFavorite {
						heartButton.SetResource(heartIcon)
					} else {
						heartButton.SetResource(heartOutlineIcon)
					}
					if isHidden {
						hideButton.SetResource(thumbDownIcon)
					} else {
						hideButton.SetResource(thumbDownOutlineIcon)
					}
					img.Refresh()
				})
			}
		}
	}()

	// UI listener goroutine for slideshow manager updates
	go func() {
		defer log.Println("Home view UI update goroutine terminated.")
		var lastDispatchedPath string

		for {
			select {
			case <-sm.ctx.Done():
				return
			case state := <-sm.StateChan:
				// Update favorite / hidden UI state immediately without waiting for image loading
				fyne.Do(func() {
					if state.ImagePath == currentPath || currentPath == "" {
						isFavorite = state.IsFavorite
						isHidden = state.IsHidden
						if isFavorite {
							heartButton.SetResource(heartIcon)
						} else {
							heartButton.SetResource(heartOutlineIcon)
						}
						if isHidden {
							hideButton.SetResource(thumbDownIcon)
						} else {
							hideButton.SetResource(thumbDownOutlineIcon)
						}
						heartButton.Show()
						hideButton.Show()
					}
				})

				// If this is a new image path, dispatch to the image loader worker
				if state.ImagePath != lastDispatchedPath {
					lastDispatchedPath = state.ImagePath
					// Drain older pending tasks to skip outdated states if behind
					for len(tasks) > 0 {
						<-tasks
					}
					tasks <- loadTask{
						ImagePath:  state.ImagePath,
						IsFavorite: state.IsFavorite,
						IsHidden:   state.IsHidden,
					}
				}
			case <-sm.NoPhotosChan:
				fyne.Do(func() {
					currentPath = ""
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
		targetPath := currentPath
		// Immediate optimistic update
		isFavorite = !isFavorite
		if isFavorite {
			heartButton.SetResource(heartIcon)
		} else {
			heartButton.SetResource(heartOutlineIcon)
		}
		go func() {
			sm.ToggleFavorite(targetPath)
		}()
	}

	hideButton.OnTapped = func() {
		if currentPath == "" {
			return
		}
		targetPath := currentPath
		// Immediate optimistic update
		isHidden = !isHidden
		if isHidden {
			hideButton.SetResource(thumbDownIcon)
		} else {
			hideButton.SetResource(thumbDownOutlineIcon)
		}
		go func() {
			sm.ToggleHidden(targetPath)
		}()
	}

	return sm.Stop // Return the stop function of the manager
}
