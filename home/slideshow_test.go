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
	"errors"
	"path/filepath"
	"reflect"
	"sort"
	"testing"
	"time"

	"fyne.io/fyne/v2"
	"github.com/liquidgecka/homehub/photomanager"
)

// Mock Fyne Resource for testing LoadImageSafely
type mockFyneResource struct {
	content []byte
	name    string
}

func (m *mockFyneResource) Content() []byte { return m.content }
func (m *mockFyneResource) Name() string    { return m.name }

// --- Mocks for photomanager functions ---
// original photomanager functions to restore after tests
var (
	originalListLocalPhotos        = photomanager.ListLocalPhotos
	originalIsPhotoHidden          = photomanager.IsPhotoHidden
	originalIsPhotoFavorite        = photomanager.IsPhotoFavorite
	originalSetPhotoFavorite       = photomanager.SetPhotoFavorite
	originalSetPhotoHidden         = photomanager.SetPhotoHidden
	originalLoadImageSafely        = photomanager.LoadImageSafely
	originalNewPhotoDownloadedChan = photomanager.NewPhotoDownloadedChan // Store original channel

	// For mocking SlideshowManager channels in tests
	testStateChan     chan SlideshowState
	testNoPhotosChan  chan bool
	testNextRotation  chan time.Duration
	testForceRotation chan bool
)

// setupMocks sets up mock functions for photomanager dependencies
func setupMocks() {
	photomanager.ListLocalPhotos = func(dir string) ([]string, error) { return []string{}, nil }
	photomanager.IsPhotoHidden = func(filename string) bool { return false }
	photomanager.IsPhotoFavorite = func(filename string) bool { return false }
	photomanager.SetPhotoFavorite = func(filename string, isFavorite bool) error { return nil }
	photomanager.SetPhotoHidden = func(filename string, isHidden bool) error { return nil }
	photomanager.LoadImageSafely = func(path string) fyne.Resource { return &mockFyneResource{name: filepath.Base(path)} } // Dummy resource
	photomanager.ListAllHiddenPhotos = func() ([]string, error) { return []string{}, nil }
	photomanager.ListAllFavoritePhotos = func() ([]string, error) { return []string{}, nil }

	// Mock NewPhotoDownloadedChan with a buffered channel for testing
	photomanager.NewPhotoDownloadedChan = make(chan bool, 10)

	// Create test channels for SlideshowManager
	testStateChan = make(chan SlideshowState, 10)
	testNoPhotosChan = make(chan bool, 10)
	testNextRotation = make(chan time.Duration, 10)
	testForceRotation = make(chan bool, 10)
}

// restoreMocks restores original photomanager functions
func restoreMocks() {
	photomanager.ListLocalPhotos = originalListLocalPhotos
	photomanager.IsPhotoHidden = originalIsPhotoHidden
	photomanager.IsPhotoFavorite = originalIsPhotoFavorite
	photomanager.SetPhotoFavorite = originalSetPhotoFavorite
	photomanager.SetPhotoHidden = originalSetPhotoHidden
	photomanager.LoadImageSafely = originalLoadImageSafely

	// Restore original channel
	photomanager.NewPhotoDownloadedChan = originalNewPhotoDownloadedChan
}

// newTestSlideshowManager for tests uses mocked channels
func newTestSlideshowManager(parentCtx context.Context, cfg SlideshowConfig) *SlideshowManager {
	ctx, cancel := context.WithCancel(parentCtx)

	sm := &SlideshowManager{
		cfg:                cfg,
		StateChan:          testStateChan,
		NoPhotosChan:       testNoPhotosChan,
		forceRotation:      testForceRotation,
		playlistUpdateChan: make(chan struct{}, 1), // Initialize playlistUpdateChan for testing
		ctx:                ctx,
		cancel:             cancel,
		done:               make(chan struct{}),
		ready:              make(chan struct{}, 1),
	}

	return sm
}

/*
func TestSlideshowManager_Stop(t *testing.T) {
	setupMocks()
	defer restoreMocks()

	cfg := SlideshowConfig{
		Directory:               "/test/photos",
		RotationIntervalSeconds: 1,
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	sm := newTestSlideshowManager(ctx, cfg)

	sm.Start()
	<-sm.ready

	sm.Stop()

	// If we reach here, Stop() completed without deadlocking or panicking.
	// Further checks can be added if specific post-stop state is required.
}
*/

func TestSlideshowManager_buildPlaylist(t *testing.T) {
	setupMocks()
	defer restoreMocks()

	cfg := SlideshowConfig{Directory: "/test/photos"}
	sm := newTestSlideshowManager(context.Background(), cfg)
	defer sm.Stop()

	// Mock photos
	photomanager.ListLocalPhotos = func(dir string) ([]string, error) {
		return []string{"/test/photos/1.jpg", "/test/photos/2.png", "/test/photos/3.jpg"}, nil
	}
	photomanager.ListAllHiddenPhotos = func() ([]string, error) {
		return []string{"2.png"}, nil // Mock 2.png as hidden
	}
	photomanager.ListAllFavoritePhotos = func() ([]string, error) {
		return []string{"1.jpg"}, nil // Mock 1.jpg as favorite
	}

	sm.buildPlaylist()

	expectedPlaylist := []string{"/test/photos/1.jpg", "/test/photos/1.jpg", "/test/photos/3.jpg"} // 1.jpg twice, 2.png hidden

	// Sort to compare as shuffle makes order non-deterministic
	sort.Strings(sm.playlist)
	sort.Strings(expectedPlaylist)

	if !reflect.DeepEqual(sm.playlist, expectedPlaylist) {
		t.Errorf("Expected playlist %v, got %v", expectedPlaylist, sm.playlist)
	}

	// Test case: Error listing photos
	photomanager.ListLocalPhotos = func(dir string) ([]string, error) {
		return nil, errors.New("cannot list photos")
	}
	sm.buildPlaylist() // Should not crash
	if len(sm.playlist) != 0 {
		t.Errorf("Expected empty playlist on error, got %v", sm.playlist)
	}
}

func TestSlideshowManager_showNextPhoto(t *testing.T) {
	setupMocks()
	defer restoreMocks()

	cfg := SlideshowConfig{Directory: "/test/photos"}
	sm := newTestSlideshowManager(context.Background(), cfg)
	defer sm.Stop()

	photomanager.ListLocalPhotos = func(dir string) ([]string, error) {
		return []string{"/test/photos/p1.jpg", "/test/photos/p2.png"}, nil
	}
	sm.buildPlaylist()
	sort.Strings(sm.playlist)
	sm.currentIndex = -1 // Simulate initial state

	// Test first photo
	sm.showNextPhoto()
	state := <-sm.StateChan
	if state.ImagePath != "/test/photos/p1.jpg" {
		t.Errorf("Expected first photo to be p1.jpg, got %s", state.ImagePath)
	}
	if sm.currentImagePath != "/test/photos/p1.jpg" {
		t.Errorf("Expected currentImagePath to be p1.jpg, got %s", sm.currentImagePath)
	}

	// Test second photo (circular)
	sm.showNextPhoto()
	state = <-sm.StateChan
	if state.ImagePath != "/test/photos/p2.png" {
		t.Errorf("Expected second photo to be p2.png, got %s", state.ImagePath)
	}

	// Test wrap around
	sm.showNextPhoto()
	state = <-sm.StateChan
	if state.ImagePath != "/test/photos/p1.jpg" {
		t.Errorf("Expected wrapped photo to be p1.jpg, got %s", state.ImagePath)
	}

	// Test no photos
	sm.playlist = []string{}
	sm.showNextPhoto()
	select {
	case <-sm.NoPhotosChan:
		// Expected
	case <-time.After(50 * time.Millisecond):
		t.Error("Expected NoPhotosChan signal")
	}
}
