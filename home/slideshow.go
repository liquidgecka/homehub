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
	"log"
	"math/rand"
	"path/filepath"
	"sync"
	"time"

	"github.com/liquidgecka/homehub/photomanager"
)

// SlideshowConfig holds configuration for the slideshow.
type SlideshowConfig struct {
	Directory               string
	RotationIntervalSeconds int
}

// SlideshowState represents the current state of the slideshow for UI consumption.
type SlideshowState struct {
	ImagePath  string
	IsFavorite bool
	IsHidden   bool
}

// SlideshowManager manages the photo playlist and state for the home view.
type SlideshowManager struct {
	cfg SlideshowConfig

	playlist         []string
	currentImagePath string
	currentIndex     int
	playlistMutex    sync.Mutex

	// Channels for communicating updates to the UI controller
	StateChan    chan SlideshowState
	NoPhotosChan chan bool

	// Used internally to manage the slideshow timing.
	forceRotation      chan bool
	playlistUpdateChan chan struct{}
	resendStateChan    chan bool

	// External context for cancellation
	ctx                      context.Context
	cancel                   context.CancelFunc
	done                     chan struct{}
	nextExpectedRotationTime time.Time
	wg                       sync.WaitGroup
	ready                    chan struct{}
}

// NewSlideshowManager creates and initializes a new SlideshowManager.
func NewSlideshowManager(parentCtx context.Context, cfg SlideshowConfig) *SlideshowManager {
	ctx, cancel := context.WithCancel(parentCtx)
	sm := &SlideshowManager{
		cfg:                      cfg,
		StateChan:                make(chan SlideshowState, 1),
		NoPhotosChan:             make(chan bool, 1),
		forceRotation:            make(chan bool, 1),
		playlistUpdateChan:       make(chan struct{}, 1),
		resendStateChan:          make(chan bool, 1),
		ctx:                      ctx,
		cancel:                   cancel,
		done:                     make(chan struct{}),
		ready:                    make(chan struct{}, 1),
		nextExpectedRotationTime: time.Now().Add(time.Duration(cfg.RotationIntervalSeconds) * time.Second),
	}

	return sm
}

// Start starts the slideshow manager's goroutine.
func (sm *SlideshowManager) Start() {
	sm.wg.Add(1)
	go sm.run()
}

// Stop terminates the slideshow manager's goroutine.
func (sm *SlideshowManager) Stop() {
	sm.cancel()
	sm.wg.Wait()
}

// PriorityRotate forces an immediate rotation to the next photo.
func (sm *SlideshowManager) PriorityRotate() {
	sm.forceRotation <- true
}

// ResendState forces the manager to resend its current state to listeners.
func (sm *SlideshowManager) ResendState() {
	select {
	case sm.resendStateChan <- true:
	default:
	}
}

func (sm *SlideshowManager) run() {
	defer sm.wg.Done()
	defer close(sm.done)
	defer log.Println("SlideshowManager goroutine terminated.")

	// Launch a goroutine to build the initial playlist in the background.
	sm.wg.Add(1)
	go func() {
		defer sm.wg.Done()
		sm.buildPlaylist()
		select {
		case sm.playlistUpdateChan <- struct{}{}:
		case <-sm.ctx.Done():
			return
		}
	}()

	ticker := time.NewTicker(sm.getRotationInterval())
	defer ticker.Stop()

	for {
		select {
		case <-sm.ctx.Done():
			return
		case <-sm.resendStateChan:
			sm.sendStateUpdate()
		case <-sm.playlistUpdateChan:
			log.Println("Playlist updated signal received.")
			sm.playlistMutex.Lock()
			if len(sm.playlist) > 0 {
				if sm.currentImagePath == "" {
					sm.currentIndex = 0
					sm.currentImagePath = sm.playlist[sm.currentIndex]
					sm.sendStateUpdate()
					ticker.Reset(sm.getRotationInterval())
				} else {
					// Check if current image is in new playlist to preserve position
					found := false
					for idx, p := range sm.playlist {
						if p == sm.currentImagePath {
							sm.currentIndex = idx
							found = true
							break
						}
					}
					if !found {
						sm.currentIndex = 0
						sm.currentImagePath = sm.playlist[sm.currentIndex]
						sm.sendStateUpdate()
						ticker.Reset(sm.getRotationInterval())
					}
				}
			} else {
				sm.sendNoPhotosSignal()
				sm.currentImagePath = ""
			}
			sm.playlistMutex.Unlock()
		case <-sm.forceRotation:
			log.Println("Force rotation triggered.")
			sm.showNextPhoto()
			ticker.Reset(sm.getRotationInterval())
		case <-photomanager.NewPhotoDownloadedChan:
			log.Println("New photo downloaded, refreshing slideshow playlist.")
			sm.wg.Add(1)
			go func() {
				defer sm.wg.Done()
				sm.buildPlaylist()
				select {
				case sm.playlistUpdateChan <- struct{}{}:
				case <-sm.ctx.Done():
					return
				}
			}()
		case <-ticker.C:
			sm.showNextPhoto()
		}
	}
}

func (sm *SlideshowManager) buildPlaylist() {
	newPlaylist := sm.generatePlaylist()
	sm.playlistMutex.Lock()
	defer sm.playlistMutex.Unlock()
	sm.playlist = newPlaylist
}

func (sm *SlideshowManager) generatePlaylist() []string {
	allImagePaths, err := photomanager.ListLocalPhotos(sm.cfg.Directory)
	if err != nil {
		log.Printf("Error listing local photos during playlist build: %v", err)
		return []string{}
	}

	// Fetch all hidden photo filenames into a map for fast lookup
	hiddenFilenames, err := photomanager.ListAllHiddenPhotos()
	if err != nil {
		// Continue with empty hidden map rather than failing playlist generation
	}
	isHiddenMap := make(map[string]bool)
	for _, f := range hiddenFilenames {
		isHiddenMap[f] = true
	}

	// Fetch all favorite photo filenames into a map for fast lookup
	favoriteFilenames, err := photomanager.ListAllFavoritePhotos()
	if err != nil {
		// Continue with empty favorite map
	}
	isFavoriteMap := make(map[string]bool)
	for _, f := range favoriteFilenames {
		isFavoriteMap[f] = true
	}

	var visiblePhotos []string
	for _, path := range allImagePaths {
		baseFilename := filepath.Base(path)
		if !isHiddenMap[baseFilename] { // Use map for fast lookup
			visiblePhotos = append(visiblePhotos, path)
		}
	}

	var playlist []string
	for _, path := range visiblePhotos {
		playlist = append(playlist, path)
		baseFilename := filepath.Base(path)
		if isFavoriteMap[baseFilename] { // Use map for fast lookup
			playlist = append(playlist, path) // Add favorites twice
		}
	}

	// Only shuffle if there are photos
	if len(playlist) > 0 {
		rand.Shuffle(len(playlist), func(i, j int) { playlist[i], playlist[j] = playlist[j], playlist[i] })
	}
	return playlist
}

func (sm *SlideshowManager) showNextPhoto() {
	sm.playlistMutex.Lock()
	defer sm.playlistMutex.Unlock()
	if len(sm.playlist) == 0 {
		sm.sendNoPhotosSignal()
		return
	}
	sm.currentIndex = (sm.currentIndex + 1) % len(sm.playlist)
	sm.currentImagePath = sm.playlist[sm.currentIndex]
	sm.sendStateUpdate() // Communicate current image and its status to UI
}

func (sm *SlideshowManager) sendNoPhotosSignal() {
	select {
	case sm.NoPhotosChan <- true:
	default:
		log.Println("NoPhotosChan is full, skipping signal.")
	}
}

func (sm *SlideshowManager) sendStateUpdate() {
	if sm.currentImagePath == "" {
		return // No photo to display yet
	}
	baseFilename := filepath.Base(sm.currentImagePath)
	state := SlideshowState{
		ImagePath:  sm.currentImagePath,
		IsFavorite: photomanager.IsPhotoFavorite(baseFilename),
		IsHidden:   photomanager.IsPhotoHidden(baseFilename),
	}
	select {
	case sm.StateChan <- state:
	default:
		// Drain stale state if channel is full and send the freshest state
		select {
		case <-sm.StateChan:
		default:
		}
		select {
		case sm.StateChan <- state:
		default:
		}
	}
}

// ToggleFavorite toggles the favorite status of the specified photo.
func (sm *SlideshowManager) ToggleFavorite(imagePath string) {
	if imagePath == "" {
		return
	}

	baseFilename := filepath.Base(imagePath)
	if photomanager.IsPhotoHidden(baseFilename) {
		log.Printf("Cannot favorite hidden photo %s.", baseFilename)
		return
	}

	isFav := photomanager.IsPhotoFavorite(baseFilename)
	if err := photomanager.SetPhotoFavorite(baseFilename, !isFav); err != nil {
		log.Printf("Error toggling favorite for %s: %v", baseFilename, err)
		return
	}

	sm.playlistMutex.Lock()
	defer sm.playlistMutex.Unlock()
	if imagePath == sm.currentImagePath {
		sm.sendStateUpdate() // Update UI after change if this is the active photo
	}
}

// ToggleHidden toggles the hidden status of the specified photo.
func (sm *SlideshowManager) ToggleHidden(imagePath string) {
	if imagePath == "" {
		return
	}

	baseFilename := filepath.Base(imagePath)
	isHidden := photomanager.IsPhotoHidden(baseFilename)

	if err := photomanager.SetPhotoHidden(baseFilename, !isHidden); err != nil {
		log.Printf("Error toggling hidden for %s: %v", baseFilename, err)
		return
	}

	sm.playlistMutex.Lock()
	isCurrent := (imagePath == sm.currentImagePath)
	sm.playlistMutex.Unlock()

	// If a photo was hidden, rebuild the playlist and rotate if it's currently showing
	sm.buildPlaylist()
	if isCurrent && !isHidden {
		sm.PriorityRotate()
	}
}

func (sm *SlideshowManager) getRotationInterval() time.Duration {
	if sm.cfg.RotationIntervalSeconds <= 0 {
		return 10 * time.Second
	}
	return time.Duration(sm.cfg.RotationIntervalSeconds) * time.Second
}
