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
	"database/sql"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"
	"time"

	"github.com/liquidgecka/homehub/database"
)

// setup and teardown for database mocks
func setupDBMocks() {
	database.GetRefreshToken = func(serviceName string) (string, error) { return "", nil }
	database.StoreRefreshToken = func(serviceName, refreshToken string) error { return nil }
}

func TestIsPhotoFavorite(t *testing.T) {
	setupDBMocks()
	originalGet := database.GetStorageValue
	defer func() { database.GetStorageValue = originalGet }()

	// Test case: Photo is a favorite
	database.GetStorageValue = func(key string) (string, error) {
		if key == "favorite_photo_test.jpg" {
			return "true", nil
		}
		return "", sql.ErrNoRows
	}
	if !IsPhotoFavorite("test.jpg") {
		t.Error("Expected IsPhotoFavorite to be true, but it was false")
	}

	// Test case: Photo is not a favorite
	database.GetStorageValue = func(key string) (string, error) {
		return "", sql.ErrNoRows
	}
	if IsPhotoFavorite("another.jpg") {
		t.Error("Expected IsPhotoFavorite to be false, but it was true")
	}
}

func TestSetPhotoFavorite(t *testing.T) {
	setupDBMocks()
	originalSet := database.SetStorageValue
	originalDel := database.DeleteStorageValue
	defer func() {
		database.SetStorageValue = originalSet
		database.DeleteStorageValue = originalDel
	}()

	// Test setting a photo as favorite
	var setKey, setValue string
	database.SetStorageValue = func(key, value string) error {
		setKey = key
		setValue = value
		return nil
	}
	err := SetPhotoFavorite("test.jpg", true)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if setKey != "favorite_photo_test.jpg" || setValue != "true" {
		t.Errorf("SetStorageValue called with wrong arguments: %s, %s", setKey, setValue)
	}

	// Test unsetting a photo as favorite
	var deletedKey string
	database.DeleteStorageValue = func(key string) error {
		deletedKey = key
		return nil
	}
	err = SetPhotoFavorite("test.jpg", false)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if deletedKey != "favorite_photo_test.jpg" {
		t.Errorf("DeleteStorageValue called with wrong key: %s", deletedKey)
	}
}

func TestGetPhotoHiddenTime(t *testing.T) {
	setupDBMocks()
	originalGet := database.GetStorageValue
	originalSet := database.SetStorageValue
	defer func() {
		database.GetStorageValue = originalGet
		database.SetStorageValue = originalSet
	}()

	// Test case: Valid RFC3339 timestamp
	now := time.Now().Truncate(time.Second)
	nowStr := now.Format(time.RFC3339)
	database.GetStorageValue = func(key string) (string, error) {
		return nowStr, nil
	}
	hiddenTime, err := GetPhotoHiddenTime("test.jpg")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if !hiddenTime.Equal(now) {
		t.Errorf("Expected time %v, got %v", now, hiddenTime)
	}

	// Test case: Legacy "true" value
	database.GetStorageValue = func(key string) (string, error) {
		return "true", nil
	}
	var updatedKey, updatedValue string
	database.SetStorageValue = func(key, value string) error {
		updatedKey = key
		updatedValue = value
		return nil
	}
	hiddenTime, err = GetPhotoHiddenTime("legacy.jpg")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if time.Since(hiddenTime) > time.Second {
		t.Error("Expected time to be very recent")
	}
	if updatedKey != "hidden_photo_legacy.jpg" {
		t.Errorf("Expected SetStorageValue to be called to update legacy value")
	}
	if _, err := time.Parse(time.RFC3339, updatedValue); err != nil {
		t.Errorf("Expected updated value to be a valid timestamp, got %s", updatedValue)
	}
}

func TestListLocalPhotos(t *testing.T) {
	// Create a temporary directory for testing
	tempDir := t.TempDir()

	// Create some dummy files
	os.Create(filepath.Join(tempDir, "image1.jpg"))
	os.Create(filepath.Join(tempDir, "image2.png"))
	os.Create(filepath.Join(tempDir, "document.txt"))
	os.Mkdir(filepath.Join(tempDir, "subdir"), 0755)
	os.Create(filepath.Join(tempDir, "subdir", "image3.jpeg"))

	photos, err := ListLocalPhotos(tempDir)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	expected := []string{
		filepath.Join(tempDir, "image1.jpg"),
		filepath.Join(tempDir, "image2.png"),
		filepath.Join(tempDir, "subdir/image3.jpeg"),
	}

	sort.Strings(photos)
	sort.Strings(expected)

	if !reflect.DeepEqual(photos, expected) {
		t.Errorf("Expected %v, got %v", expected, photos)
	}
}

func TestListAllHiddenPhotos(t *testing.T) {
	originalList := database.ListStorageKeysWithPrefix
	defer func() { database.ListStorageKeysWithPrefix = originalList }()

	database.ListStorageKeysWithPrefix = func(prefix string) ([]string, error) {
		if prefix == hiddenPhotoPrefix {
			return []string{"hidden_photo_a.jpg", "hidden_photo_b.png"}, nil
		}
		return nil, errors.New("unexpected prefix")
	}

	hidden, err := ListAllHiddenPhotos()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	expected := []string{"a.jpg", "b.png"}
	sort.Strings(hidden)
	sort.Strings(expected)

	if !reflect.DeepEqual(hidden, expected) {
		t.Errorf("Expected %v, got %v", expected, hidden)
	}
}

func TestLoadImageSafely(t *testing.T) {
	tempDir := t.TempDir()
	testImagePath := filepath.Join(tempDir, "test.jpg")

	// Create a simple JPEG image
	img := image.NewRGBA(image.Rect(0, 0, 10, 10))
	for y := 0; y < 10; y++ {
		for x := 0; x < 10; x++ {
			img.Set(x, y, color.RGBA{R: uint8(x * 25), G: uint8(y * 25), B: 0, A: 255})
		}
	}

	file, err := os.Create(testImagePath)
	if err != nil {
		t.Fatalf("Failed to create test image file: %v", err)
	}
	defer file.Close()
	jpeg.Encode(file, img, nil)

	res := LoadImageSafely(testImagePath)
	if res == nil {
		t.Fatal("LoadImageSafely returned nil resource")
	}
	if len(res.Content()) == 0 {
		t.Error("LoadImageSafely returned empty resource content")
	}
	if res.Name() != "test.jpg" {
		t.Errorf("Expected resource name 'test.jpg', got '%s'", res.Name())
	}
}
