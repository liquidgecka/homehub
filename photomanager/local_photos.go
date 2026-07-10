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
	"fmt"
	"image"
	"image/jpeg"
	"image/png" // Explicitly import for encoding
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"fyne.io/fyne/v2"

	"github.com/disintegration/imageorient" // Import the imageorient library
	"github.com/nfnt/resize"                // Import for image resizing
)

// supportedImageExtensions is a map of file extensions that the app will recognize as images.
var supportedImageExtensions = map[string]bool{
	".jpg":  true,
	".jpeg": true,
	".png":  true,
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

// AddPhoto saves a new photo to the local filesystem.
var AddPhoto = func(filename string, data []byte, localPhotosDir string) error {
	localPath := filepath.Join(localPhotosDir, filename)
	if err := os.WriteFile(localPath, data, 0644); err != nil {
		return fmt.Errorf("failed to save photo file '%s': %w", localPath, err)
	}
	log.Printf("Successfully saved photo file: %s", localPath)

	// Trigger playlist refresh
	select {
	case NewPhotoDownloadedChan <- true:
	default:
	}

	return nil
}

// GenerateThumbnail creates a thumbnail for a given image file.
// It resizes the image to the specified width, maintaining aspect ratio, and returns
// the JPEG encoded bytes.
var GenerateThumbnail = func(imagePath string, width uint) ([]byte, error) {
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

	return buf.Bytes(), nil
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
