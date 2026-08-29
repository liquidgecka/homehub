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
	"crypto/sha256"
	"errors"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"fyne.io/fyne/v2"

	"github.com/disintegration/imageorient"
	"github.com/nfnt/resize"
)

// supportedImageExtensions is a map of file extensions that the app will recognize as images.
var supportedImageExtensions = map[string]bool{
	".jpg":  true,
	".jpeg": true,
	".png":  true,
}

// ErrDuplicatePhoto is returned when an uploaded photo is identical to an existing one.
var ErrDuplicatePhoto = errors.New("duplicate photo already exists")

// NewPhotoDownloadedChan is a channel used to signal when a new photo is added or deleted.
var NewPhotoDownloadedChan = make(chan bool, 1)

// NotifyNewPhotoDownloaded signals listeners (such as the slideshow) that photos have changed.
func NotifyNewPhotoDownloaded() {
	select {
	case NewPhotoDownloadedChan <- true:
	default:
	}
}

// ListLocalPhotos scans a directory and returns a slice of paths to supported image files.
var ListLocalPhotos = func(dir string) ([]string, error) {
	var imagePaths []string

	if _, err := os.Stat(dir); os.IsNotExist(err) {
		// Create the directory if it doesn't exist.
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("photo directory '%s' does not exist and could not be created: %w", dir, err)
		}
		return imagePaths, nil // Return empty slice if dir was just created.
	}

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			ext := strings.ToLower(filepath.Ext(path))
			if supportedImageExtensions[ext] {
				imagePaths = append(imagePaths, path)
			}
		}
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("error walking photo directory '%s': %w", dir, err)
	}

	return imagePaths, nil
}

// IsDuplicatePhoto checks if a photo with identical content (SHA-256 hash) exists in localPhotosDir.
var IsDuplicatePhoto = func(data []byte, localPhotosDir string) (bool, string, error) {
	if len(data) == 0 {
		return false, "", nil
	}
	incomingHash := sha256.Sum256(data)
	incomingSize := int64(len(data))

	existingPhotos, err := ListLocalPhotos(localPhotosDir)
	if err != nil {
		return false, "", err
	}

	for _, path := range existingPhotos {
		info, err := os.Stat(path)
		if err != nil {
			continue
		}
		if info.Size() == incomingSize {
			existingData, err := os.ReadFile(path)
			if err != nil {
				continue
			}
			if sha256.Sum256(existingData) == incomingHash {
				return true, path, nil
			}
		}
	}
	return false, "", nil
}

// DeletePhoto removes a photo file from the local filesystem and deletes its associated metadata.
var DeletePhoto = func(filename string, localPhotosDir string) error {
	localPath := filepath.Join(localPhotosDir, filename)

	// Delete the file from the filesystem
	if err := os.Remove(localPath); err != nil {
		return fmt.Errorf("failed to delete photo file '%s': %w", localPath, err)
	}
	log.Printf("Successfully deleted photo file: %s", localPath)

	// Delete associated metadata (favorite and hidden status)
	if err := SetPhotoFavorite(filename, false); err != nil {
		log.Printf("Warning: Failed to remove favorite status for '%s': %v", filename, err)
	}
	if err := SetPhotoHidden(filename, false); err != nil {
		log.Printf("Warning: Failed to remove hidden status for '%s': %v", filename, err)
	}

	// Trigger playlist refresh
	select {
	case NewPhotoDownloadedChan <- true:
	default:
	}

	return nil
}

// AddPhoto saves a new photo to the local filesystem with automatic deduplication.
// If an identical photo is found, it returns ErrDuplicatePhoto.
// If a different photo exists with the same filename, it chooses a unique filename.
var AddPhoto = func(filename string, data []byte, localPhotosDir string) error {
	if err := os.MkdirAll(localPhotosDir, 0755); err != nil {
		return fmt.Errorf("failed to create photo directory '%s': %w", localPhotosDir, err)
	}

	isDup, existingPath, err := IsDuplicatePhoto(data, localPhotosDir)
	if err != nil {
		log.Printf("Warning: error checking for duplicate photo: %v", err)
	}
	if isDup {
		log.Printf("Photo '%s' is a duplicate of '%s', skipping save.", filename, existingPath)
		return ErrDuplicatePhoto
	}

	// Check if filename exists with different content; if so, assign a unique name
	targetPath := filepath.Join(localPhotosDir, filename)
	if _, err := os.Stat(targetPath); err == nil {
		ext := filepath.Ext(filename)
		base := strings.TrimSuffix(filename, ext)
		counter := 1
		for {
			uniqueFilename := fmt.Sprintf("%s_%d%s", base, counter, ext)
			targetPath = filepath.Join(localPhotosDir, uniqueFilename)
			if _, err := os.Stat(targetPath); os.IsNotExist(err) {
				filename = uniqueFilename
				break
			}
			counter++
		}
	}

	if err := os.WriteFile(targetPath, data, 0644); err != nil {
		return fmt.Errorf("failed to save photo file '%s': %w", targetPath, err)
	}
	log.Printf("Successfully saved photo file: %s", targetPath)

	// Trigger playlist refresh
	select {
	case NewPhotoDownloadedChan <- true:
	default:
	}

	return nil
}

// GenerateThumbnail creates a thumbnail for a given image file.
// It resizes the image to the specified width, maintaining aspect ratio, and returns
// the JPEG encoded bytes. Generated thumbnails are cached on disk for fast retrieval.
var GenerateThumbnail = func(imagePath string, width uint) ([]byte, error) {
	srcInfo, err := os.Stat(imagePath)
	if err != nil {
		return nil, fmt.Errorf("failed to stat image file: %w", err)
	}

	cacheDir := filepath.Join(os.Getenv("HOME"), ".cache", "homehub", "thumbnails")
	cacheKey := fmt.Sprintf("%x_%d.jpg", sha256.Sum256([]byte(imagePath)), width)
	cachePath := filepath.Join(cacheDir, cacheKey)

	if cacheInfo, err := os.Stat(cachePath); err == nil {
		if cacheInfo.ModTime().After(srcInfo.ModTime()) {
			cachedData, err := os.ReadFile(cachePath)
			if err == nil && len(cachedData) > 0 {
				return cachedData, nil
			}
		}
	}

	file, err := os.Open(imagePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open image file: %w", err)
	}
	defer file.Close()

	// Decode the image
	img, _, err := image.Decode(file)
	if err != nil {
		return nil, fmt.Errorf("failed to decode image: %w", err)
	}

	// Resize the image
	thumbnail := resize.Resize(width, 0, img, resize.Lanczos3)

	// Encode the thumbnail as JPEG
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, thumbnail, nil); err != nil {
		return nil, fmt.Errorf("failed to encode thumbnail to JPEG: %w", err)
	}

	data := buf.Bytes()
	if err := os.MkdirAll(cacheDir, 0755); err == nil {
		_ = os.WriteFile(cachePath, data, 0644)
	}

	return data, nil
}

// CleanupHiddenPhotos checks for photos that have been hidden for more than 30 days
// and deletes them from the local filesystem, also removing their hidden status.
func CleanupHiddenPhotos(localPhotosDir string) {
	hiddenPhotos, err := ListAllHiddenPhotos()
	if err != nil {
		log.Printf("Error listing all hidden photos for cleanup: %v", err)
		return
	}

	for _, filename := range hiddenPhotos {
		hiddenTime, err := GetPhotoHiddenTime(filename)
		if err != nil {
			log.Printf("Error getting hidden time for %s: %v", filename, err)
			continue
		}

		// If hidden for more than 30 days
		if time.Since(hiddenTime) > 30*24*time.Hour {
			localPath := filepath.Join(localPhotosDir, filename)
			if err := os.Remove(localPath); err != nil {
				log.Printf("Error deleting old hidden photo %s: %v", localPath, err)
			} else {
				log.Printf("Successfully deleted old hidden photo %s.", localPath)
				// Also remove the hidden status from the database
				if err := SetPhotoHidden(filename, false); err != nil {
					log.Printf("Error removing hidden status for %s after deletion: %v", filename, err)
				}
			}
		}
	}
}

// LoadDecodedImage attempts to load an image file, applies EXIF orientation if present,
// and returns the decoded image.Image.
// If the image dimensions are excessively large (> 2560px), it downscales the image
// to optimize memory usage and GPU texture upload performance.
var LoadDecodedImage = func(path string) (image.Image, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open image file %s: %w", path, err)
	}
	defer file.Close()

	img, _, err := imageorient.Decode(file)
	if err != nil {
		return nil, fmt.Errorf("failed to decode image with orientation for %s: %w", path, err)
	}

	bounds := img.Bounds()
	const maxDim = 1920
	if bounds.Dx() > maxDim || bounds.Dy() > maxDim {
		img = resize.Thumbnail(maxDim, maxDim, img, resize.Bilinear)
	}

	return img, nil
}

// LoadImageSafely attempts to load an image file, applies EXIF orientation if present,
// and returns it as a fyne.Resource.
// If the image is corrupted or cannot be decoded, it returns an empty resource with a log message.
var LoadImageSafely = func(path string) fyne.Resource {
	data, err := os.ReadFile(path)
	if err != nil {
		log.Printf("ERROR: LoadImageSafely - Failed to read image file %s: %v. Using empty resource.", path, err)
		return fyne.NewStaticResource("placeholder.png", []byte{})
	}

	// Use imageorient.Decode to automatically handle EXIF orientation
	img, formatName, err := imageorient.Decode(bytes.NewReader(data))
	if err != nil {
		log.Printf("ERROR: LoadImageSafely - Failed to decode image with orientation for %s: %v. Using empty resource.", path, err)
		return fyne.NewStaticResource("placeholder.png", []byte{})
	}

	// Re-encode the image to a bytes.Buffer
	var buf bytes.Buffer
	switch formatName {
	case "jpeg":
		err = jpeg.Encode(&buf, img, nil)
	case "png":
		err = png.Encode(&buf, img)
	default:
		log.Printf("ERROR: LoadImageSafely - Unsupported image format %s for %s. Using original data.", formatName, path)
		return fyne.NewStaticResource(filepath.Base(path), data)
	}

	if err != nil {
		log.Printf("ERROR: LoadImageSafely - Failed to re-encode image for %s: %v. Using original data.", path, err)
		return fyne.NewStaticResource(filepath.Base(path), data)
	}

	return fyne.NewStaticResource(filepath.Base(path), buf.Bytes())
}
