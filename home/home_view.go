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
	"image/color"
	"log"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"

	"github.com/liquidgecka/homehub/celebrations"
	"github.com/liquidgecka/homehub/photomanager"
	"github.com/liquidgecka/homehub/reminders"
	"github.com/liquidgecka/homehub/ui"
)

// CreateView initializes the Fyne UI elements for the home view.
// It returns main view object and references to dynamic UI elements.
func CreateView() (
	fyne.CanvasObject,
	*canvas.Image,
	*widget.Label,
	*ui.TappableIcon,
	*ui.TappableIcon,
) {
	img := canvas.NewImageFromFile("")
	img.FillMode = canvas.ImageFillContain
	img.Hide()

	label := widget.NewLabel("Loading photos...")
	label.Alignment = fyne.TextAlignCenter

	heartOutlineIcon := fyne.NewStaticResource(
		"heart_outline.svg",
		ui.MustLoadFile(ui.GetIconPath("heart_outline.svg")),
	)
	heartButton := ui.NewTappableIcon(heartOutlineIcon, nil)
	heartButton.Hide()

	thumbDownOutlineIcon := fyne.NewStaticResource(
		"thumb_down_outline.svg",
		ui.MustLoadFile(ui.GetIconPath("thumb_down_outline.svg")),
	)
	hideButton := ui.NewTappableIcon(thumbDownOutlineIcon, nil)
	hideButton.Hide()

	bottomButtonContainer := container.NewHBox(
		container.NewPadded(hideButton),
		layout.NewSpacer(),
		container.NewPadded(heartButton),
	)

	celebrationsOverlay := celebrations.CreatePhotoOverlayView()
	remindersOverlay := reminders.CreatePhotoOverlayView()
	anniversaryBanner := createAnniversaryBanner()

	view := container.NewMax(
		img,
		container.NewCenter(label),
		container.NewBorder(nil, bottomButtonContainer, nil, nil, nil),
		container.NewBorder(
			container.NewPadded(container.NewCenter(anniversaryBanner)),
			nil, nil, nil, nil,
		),
		celebrationsOverlay,
		remindersOverlay,
	)

	return view, img, label, heartButton, hideButton
}

// StartSlideshowAndPhotoListener initializes and manages the slideshow
// logic and UI updates.
func StartSlideshowAndPhotoListener(
	img *canvas.Image,
	label *widget.Label,
	heartButton *ui.TappableIcon,
	hideButton *ui.TappableIcon,
	sm *SlideshowManager,
) context.CancelFunc {
	heartOutlineIcon := fyne.NewStaticResource(
		"heart_outline.svg",
		ui.MustLoadFile(ui.GetIconPath("heart_outline.svg")),
	)
	heartIcon := fyne.NewStaticResource(
		"heart.svg",
		ui.MustLoadFile(ui.GetIconPath("heart.svg")),
	)
	thumbDownOutlineIcon := fyne.NewStaticResource(
		"thumb_down_outline.svg",
		ui.MustLoadFile(ui.GetIconPath("thumb_down_outline.svg")),
	)
	thumbDownIcon := fyne.NewStaticResource(
		"thumb_down.svg",
		ui.MustLoadFile(ui.GetIconPath("thumb_down.svg")),
	)

	var (
		stateMu     sync.RWMutex
		currentPath string
		isFavorite  bool
		isHidden    bool
	)

	type loadTask struct {
		ImagePath  string
		IsFavorite bool
		IsHidden   bool
	}
	tasks := make(chan loadTask, 10)

	// Start a single worker goroutine to load and decode images sequentially
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
					decodedImg, err = photomanager.LoadDecodedImage(
						task.ImagePath,
					)
					if err != nil {
						log.Printf(
							"Warning: Failed to decode image for %s: %v.",
							task.ImagePath, err,
						)
						loadedRes := photomanager.LoadImageSafely(
							task.ImagePath,
						)
						anniversaryText := getAnniversaryText(task.ImagePath)
						fyne.Do(func() {
							stateMu.Lock()
							currentPath = task.ImagePath
							isFavorite = task.IsFavorite
							isHidden = task.IsHidden
							fav := isFavorite
							hid := isHidden
							stateMu.Unlock()

							label.Hide()
							img.Image = nil
							img.Resource = loadedRes
							img.Show()
							heartButton.Show()
							hideButton.Show()

							if fav {
								heartButton.SetResource(heartIcon)
							} else {
								heartButton.SetResource(heartOutlineIcon)
							}
							if hid {
								hideButton.SetResource(thumbDownIcon)
							} else {
								hideButton.SetResource(thumbDownOutlineIcon)
							}
							updateAnniversaryBanner(anniversaryText)
							img.Refresh()
						})
						continue
					}
					lastDecodedPath = task.ImagePath
					lastDecodedImg = decodedImg
				}

				anniversaryText := getAnniversaryText(task.ImagePath)
				fyne.Do(func() {
					stateMu.Lock()
					currentPath = task.ImagePath
					isFavorite = task.IsFavorite
					isHidden = task.IsHidden
					fav := isFavorite
					hid := isHidden
					stateMu.Unlock()

					label.Hide()
					img.Resource = nil
					img.Image = decodedImg
					img.Show()
					heartButton.Show()
					hideButton.Show()

					if fav {
						heartButton.SetResource(heartIcon)
					} else {
						heartButton.SetResource(heartOutlineIcon)
					}
					if hid {
						hideButton.SetResource(thumbDownIcon)
					} else {
						hideButton.SetResource(thumbDownOutlineIcon)
					}
					updateAnniversaryBanner(anniversaryText)
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
				// Update favorite / hidden UI state immediately
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
					stateMu.Lock()
					currentPath = ""
					stateMu.Unlock()

					photoDir := sm.cfg.Directory
					if photoDir == "" {
						photoDir = "photos/"
					}
					label.SetText(fmt.Sprintf("No photos found in '%s'.", photoDir))
					label.Show()
					img.Hide()
					heartButton.Hide()
					hideButton.Hide()
					updateAnniversaryBanner("")
				})
			}
		}
	}()

	// Bind UI actions to SlideshowManager methods
	heartButton.OnTapped = func() {
		stateMu.Lock()
		if currentPath == "" {
			stateMu.Unlock()
			return
		}
		targetPath := currentPath
		isFavorite = !isFavorite
		fav := isFavorite
		stateMu.Unlock()

		if fav {
			heartButton.SetResource(heartIcon)
		} else {
			heartButton.SetResource(heartOutlineIcon)
		}
		go func() {
			sm.ToggleFavorite(targetPath)
		}()
	}

	hideButton.OnTapped = func() {
		stateMu.Lock()
		if currentPath == "" {
			stateMu.Unlock()
			return
		}
		targetPath := currentPath
		isHidden = !isHidden
		hid := isHidden
		stateMu.Unlock()

		if hid {
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

var (
	anniversaryBannerContainer *fyne.Container
	anniversaryBannerLabel     *canvas.Text
	anniversaryBannerMu        sync.Mutex
	timeNow                    = time.Now
)

// formatAnniversaryText computes the celebratory banner text if takenDate
// occurred on the same month and day as nowDate. Returns empty string if not
// an anniversary or if years < 0.
func formatAnniversaryText(takenDate, nowDate time.Time) string {
	if takenDate.IsZero() || nowDate.IsZero() {
		return ""
	}
	if takenDate.Month() != nowDate.Month() || takenDate.Day() != nowDate.Day() {
		return ""
	}
	years := nowDate.Year() - takenDate.Year()
	switch {
	case years == 1:
		return "1 year ago today"
	case years > 1:
		return fmt.Sprintf("%d years ago today", years)
	default:
		return ""
	}
}

// getAnniversaryText inspects the EXIF DateTimeOriginal of filePath and
// returns the banner message if it matches today's date.
func getAnniversaryText(filePath string) string {
	if filePath == "" {
		return ""
	}
	exifDate, err := photomanager.GetEXIFDate(filePath)
	if err != nil {
		return ""
	}
	return formatAnniversaryText(exifDate, timeNow())
}

// createAnniversaryBanner builds the UI container for the anniversary banner.
func createAnniversaryBanner() fyne.CanvasObject {
	anniversaryBannerMu.Lock()
	defer anniversaryBannerMu.Unlock()

	label := canvas.NewText("", color.White)
	label.TextSize = 18
	label.TextStyle.Bold = true
	label.Alignment = fyne.TextAlignCenter
	anniversaryBannerLabel = label

	sparkleRes := fyne.NewStaticResource(
		"sparkle.svg",
		ui.MustLoadFile(ui.GetIconPath("sparkle.svg")),
	)
	sparkleLeft := canvas.NewImageFromResource(sparkleRes)
	sparkleLeft.FillMode = canvas.ImageFillContain
	sparkleLeft.SetMinSize(fyne.NewSize(24, 24))

	sparkleRight := canvas.NewImageFromResource(sparkleRes)
	sparkleRight.FillMode = canvas.ImageFillContain
	sparkleRight.SetMinSize(fyne.NewSize(24, 24))

	content := container.NewHBox(
		container.NewPadded(sparkleLeft),
		container.NewPadded(label),
		container.NewPadded(sparkleRight),
	)

	bg := canvas.NewRectangle(color.NRGBA{R: 20, G: 25, B: 40, A: 220})
	bg.CornerRadius = 12
	bg.StrokeColor = color.NRGBA{R: 251, G: 191, B: 36, A: 200} // Amber glow
	bg.StrokeWidth = 2

	banner := container.New(
		layout.NewMaxLayout(),
		bg,
		container.NewPadded(content),
	)
	anniversaryBannerContainer = banner
	banner.Hide()

	return banner
}

// updateAnniversaryBanner shows or hides the banner according to text.
func updateAnniversaryBanner(text string) {
	anniversaryBannerMu.Lock()
	defer anniversaryBannerMu.Unlock()

	if anniversaryBannerContainer == nil || anniversaryBannerLabel == nil {
		return
	}

	if text == "" {
		anniversaryBannerContainer.Hide()
	} else {
		anniversaryBannerLabel.Text = text
		anniversaryBannerLabel.Refresh()
		anniversaryBannerContainer.Show()
		anniversaryBannerContainer.Refresh()
	}
}
