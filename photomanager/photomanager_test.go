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
	"bytes"
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
	database.GetRefreshToken = func(serviceName string) (string, error) {
		return "", nil
	}
	database.StoreRefreshToken = func(
		serviceName, refreshToken string,
	) error {
		return nil
	}
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
		t.Errorf(
			"SetStorageValue called with wrong arguments: %s, %s",
			setKey, setValue,
		)
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
		t.Errorf(
			"Expected updated value to be a valid timestamp, got %s",
			updatedValue,
		)
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

func TestLoadDecodedImage(t *testing.T) {
	tempDir := t.TempDir()
	testImagePath := filepath.Join(tempDir, "test.jpg")

	img := image.NewRGBA(image.Rect(0, 0, 20, 20))
	for y := 0; y < 20; y++ {
		for x := 0; x < 20; x++ {
			img.Set(x, y, color.RGBA{R: uint8(x * 10), G: uint8(y * 10), B: 128, A: 255})
		}
	}

	file, err := os.Create(testImagePath)
	if err != nil {
		t.Fatalf("Failed to create test image file: %v", err)
	}
	defer file.Close()
	jpeg.Encode(file, img, nil)

	decoded, err := LoadDecodedImage(testImagePath)
	if err != nil {
		t.Fatalf("LoadDecodedImage failed: %v", err)
	}
	if decoded == nil {
		t.Fatal("LoadDecodedImage returned nil image")
	}
	if decoded.Bounds().Dx() != 20 || decoded.Bounds().Dy() != 20 {
		t.Errorf(
			"Expected 20x20 image, got %dx%d",
			decoded.Bounds().Dx(), decoded.Bounds().Dy(),
		)
	}
}

func TestGenerateThumbnail(t *testing.T) {
	tempDir := t.TempDir()
	testImagePath := filepath.Join(tempDir, "test.jpg")

	img := image.NewRGBA(image.Rect(0, 0, 100, 100))
	file, err := os.Create(testImagePath)
	if err != nil {
		t.Fatalf("Failed to create test image file: %v", err)
	}
	defer file.Close()
	if err := jpeg.Encode(file, img, nil); err != nil {
		t.Fatalf("Failed to encode test image: %v", err)
	}

	// First call: generate thumbnail
	data1, err := GenerateThumbnail(testImagePath, 50)
	if err != nil {
		t.Fatalf("GenerateThumbnail failed on first call: %v", err)
	}
	if len(data1) == 0 {
		t.Fatal("GenerateThumbnail returned empty slice on first call")
	}

	// Second call: should hit disk cache
	data2, err := GenerateThumbnail(testImagePath, 50)
	if err != nil {
		t.Fatalf("GenerateThumbnail failed on second call: %v", err)
	}
	if !reflect.DeepEqual(data1, data2) {
		t.Errorf("Expected cached thumbnail data to match original data")
	}
}

func TestGenerateThumbnailOrientation(t *testing.T) {
	tempDir := t.TempDir()
	testImagePath := filepath.Join(tempDir, "oriented.jpg")

	// Create a 100x50 image (wider than tall)
	img := image.NewRGBA(image.Rect(0, 0, 100, 50))
	for y := 0; y < 50; y++ {
		for x := 0; x < 100; x++ {
			img.Set(x, y, color.RGBA{R: 200, G: 100, B: 50, A: 255})
		}
	}

	var rawJpeg bytes.Buffer
	if err := jpeg.Encode(&rawJpeg, img, nil); err != nil {
		t.Fatalf("Failed to encode base JPEG: %v", err)
	}

	// EXIF Orientation 6 (Rotate 90 CW) header
	exifHeader := []byte{
		0xFF, 0xE1, // APP1 marker
		0x00, 0x1E, // Length: 30 bytes
		0x45, 0x78, 0x69, 0x66, 0x00, 0x00, // "Exif\x00\x00"
		0x49, 0x49, 0x2A, 0x00, // TIFF Little Endian
		0x08, 0x00, 0x00, 0x00, // Offset to 1st IFD
		0x01, 0x00, // Entry count: 1
		0x12, 0x01, // Tag 0x0112 (Orientation)
		0x03, 0x00, // Type 3 (SHORT)
		0x01, 0x00, 0x00, 0x00, // Count: 1
		0x06, 0x00, 0x00, 0x00, // Value: 6 (Rotate 90 CW)
		0x00, 0x00, 0x00, 0x00, // Next IFD offset: 0
	}

	// Insert EXIF header right after SOI (\xFF\xD8)
	rawBytes := rawJpeg.Bytes()
	var orientedJpeg bytes.Buffer
	orientedJpeg.Write(rawBytes[:2])
	orientedJpeg.Write(exifHeader)
	orientedJpeg.Write(rawBytes[2:])

	if err := os.WriteFile(testImagePath, orientedJpeg.Bytes(), 0644); err != nil {
		t.Fatalf("Failed to write oriented test image: %v", err)
	}

	// Generate thumbnail with width 50
	thumbBytes, err := GenerateThumbnail(testImagePath, 50)
	if err != nil {
		t.Fatalf("GenerateThumbnail failed: %v", err)
	}

	// Decode generated thumbnail to verify dimensions
	thumbImg, err := jpeg.Decode(bytes.NewReader(thumbBytes))
	if err != nil {
		t.Fatalf("Failed to decode generated thumbnail: %v", err)
	}

	// Since the original was 100x50 with orientation 6 (90 deg CW), its
	// oriented dimensions are 50x100 (taller than wide). When resized to
	// width 50, height should be 100.
	bounds := thumbImg.Bounds()
	if bounds.Dx() != 50 || bounds.Dy() != 100 {
		t.Errorf(
			"Expected thumbnail dimensions 50x100 (oriented), got %dx%d",
			bounds.Dx(), bounds.Dy(),
		)
	}
}

func TestAddPhotoAndDeduplication(t *testing.T) {
	tempDir := t.TempDir()

	photo1 := []byte("image-data-sample-one")
	photo2 := []byte("image-data-sample-two")

	// 1. Add first photo
	err := AddPhoto("pic1.jpg", photo1, tempDir)
	if err != nil {
		t.Fatalf("AddPhoto failed for pic1.jpg: %v", err)
	}

	// Verify file exists on disk
	if _, err := os.Stat(filepath.Join(tempDir, "pic1.jpg")); os.IsNotExist(err) {
		t.Errorf("Expected pic1.jpg to exist on disk")
	}

	// 2. Add duplicate photo with same filename
	err = AddPhoto("pic1.jpg", photo1, tempDir)
	if !errors.Is(err, ErrDuplicatePhoto) {
		t.Errorf("Expected ErrDuplicatePhoto for identical photo, got: %v", err)
	}

	// 3. Add duplicate photo with different filename
	err = AddPhoto("pic1_copy.jpg", photo1, tempDir)
	if !errors.Is(err, ErrDuplicatePhoto) {
		t.Errorf(
			"Expected ErrDuplicatePhoto for copy, got: %v",
			err,
		)
	}

	// 4. Add different photo with same filename as existing (unique name)
	err = AddPhoto("pic1.jpg", photo2, tempDir)
	if err != nil {
		t.Fatalf("AddPhoto failed for pic1.jpg with different data: %v", err)
	}

	// Verify unique filename was generated
	savedTarget := filepath.Join(tempDir, "pic1_1.jpg")
	if _, err := os.Stat(savedTarget); os.IsNotExist(err) {
		t.Errorf("Expected pic1_1.jpg to exist on disk for conflicting name")
	}
}

func TestNotifyNewPhotoDownloaded(t *testing.T) {
	// Drain channel
	select {
	case <-NewPhotoDownloadedChan:
	default:
	}

	NotifyNewPhotoDownloaded()

	select {
	case val := <-NewPhotoDownloadedChan:
		if !val {
			t.Errorf("Expected true from NewPhotoDownloadedChan, got false")
		}
	default:
		t.Errorf("Expected message on NewPhotoDownloadedChan")
	}

	// Multiple calls should not block
	NotifyNewPhotoDownloaded()
	NotifyNewPhotoDownloaded()
}

func TestPhotoPathSanitization(t *testing.T) {
	tempDir := t.TempDir()

	// Test DeletePhoto rejecting path traversal
	err := DeletePhoto("../../etc/passwd", tempDir)
	if err == nil {
		t.Errorf("Expected error for DeletePhoto with traversal, got nil")
	}

	err = DeletePhoto("", tempDir)
	if err == nil {
		t.Errorf("Expected error for DeletePhoto with empty filename, got nil")
	}

	// Test AddPhoto sanitizing path traversal filename
	data := []byte("test-content")
	err = AddPhoto("../../test.jpg", data, tempDir)
	if err != nil {
		t.Fatalf("AddPhoto failed: %v", err)
	}

	// Verify it was saved as test.jpg inside tempDir (not escaped)
	if _, err := os.Stat(filepath.Join(tempDir, "test.jpg")); err != nil {
		t.Errorf("Expected test.jpg inside tempDir: %v", err)
	}
}
