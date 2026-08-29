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
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/liquidgecka/homehub/database"
)

const favoritePhotoPrefix = "favorite_photo_"
const hiddenPhotoPrefix = "hidden_photo_"

// IsPhotoFavorite checks if a photo is marked as a favorite in the database.
var IsPhotoFavorite = func(filename string) bool {
	key := fmt.Sprintf("%s%s", favoritePhotoPrefix, filename)
	val, err := database.GetStorageValue(key)
	return err == nil && val == "true"
}

// SetPhotoFavorite marks a photo as a favorite or not in the database.
var SetPhotoFavorite = func(filename string, isFavorite bool) error {
	key := fmt.Sprintf("%s%s", favoritePhotoPrefix, filename)
	if isFavorite {
		return database.SetStorageValue(key, "true")
	}
	return database.DeleteStorageValue(key)
}

// IsPhotoHidden checks if a photo is marked as hidden in the database.
var IsPhotoHidden = func(filename string) bool {
	t, err := GetPhotoHiddenTime(filename)
	return err == nil && !t.IsZero()
}

// SetPhotoHidden marks a photo as hidden or not in the database.
var SetPhotoHidden = func(filename string, isHidden bool) error {
	key := fmt.Sprintf("%s%s", hiddenPhotoPrefix, filename)
	if isHidden {
		// Store the timestamp when the photo was hidden
		return database.SetStorageValue(key, time.Now().Format(time.RFC3339))
	}
	// If no longer hidden, delete the key
	return database.DeleteStorageValue(key)
}

// GetPhotoHiddenTime retrieves the timestamp when a photo was marked hidden.
// Returns time.Time{} and an error if not found or cannot be parsed.
var GetPhotoHiddenTime = func(filename string) (time.Time, error) {
	key := fmt.Sprintf("%s%s", hiddenPhotoPrefix, filename)
	val, err := database.GetStorageValue(key)
	if err != nil {
		return time.Time{}, fmt.Errorf(
			"photo not found or error retrieving hidden time: %w", err,
		)
	}

	// Handle legacy "true" value
	if val == "true" {
		// This is old data. Treat as if hidden now.
		now := time.Now()
		// Update record to the new format.
		if err := database.SetStorageValue(
			key, now.Format(time.RFC3339),
		); err != nil {
			log.Printf(
				"Failed to update legacy hidden time for %s: %v",
				filename, err,
			)
		}
		return now, nil
	}

	t, err := time.Parse(time.RFC3339, val)
	if err != nil {
		return time.Time{}, fmt.Errorf(
			"failed to parse hidden time for %s: %w", filename, err,
		)
	}
	return t, nil
}

// ListAllHiddenPhotos retrieves a list of all filenames that are currently
// marked as hidden.
var ListAllHiddenPhotos = func() ([]string, error) {
	keys, err := database.ListStorageKeysWithPrefix(hiddenPhotoPrefix)
	if err != nil {
		return nil, fmt.Errorf("failed to list hidden photo keys: %w", err)
	}

	var filenames []string
	for _, key := range keys {
		// Remove the prefix to get the actual filename
		filename := strings.TrimPrefix(key, hiddenPhotoPrefix)
		filenames = append(filenames, filename)
	}
	return filenames, nil
}

// ListAllFavoritePhotos retrieves a list of all filenames that are currently
// marked as favorite.
var ListAllFavoritePhotos = func() ([]string, error) {
	keys, err := database.ListStorageKeysWithPrefix(favoritePhotoPrefix)
	if err != nil {
		return nil, fmt.Errorf("failed to list favorite photo keys: %w", err)
	}

	var filenames []string
	for _, key := range keys {
		// Remove the prefix to get the actual filename
		filename := strings.TrimPrefix(key, favoritePhotoPrefix)
		filenames = append(filenames, filename)
	}
	return filenames, nil
}
