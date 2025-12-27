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
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"google.golang.org/api/drive/v3"

	"github.com/liquidgecka/homehub/config"
	"github.com/liquidgecka/homehub/database"
	"github.com/liquidgecka/homehub/google"
)

// StartDrivePhotoSync initializes and runs a goroutine for periodic Google Drive photo synchronization.
func StartDrivePhotoSync(parentCtx context.Context) context.CancelFunc {
	ctx, cancel := context.WithCancel(parentCtx)
	cfg := config.GetConfig()
	interval := time.Duration(cfg.Google.Drive.CheckIntervalMinutes) * time.Minute

	// Run once immediately in a goroutine to avoid blocking startup
	go func() {
		defer log.Println("Initial Google Drive photo check goroutine terminated.")
		select {
		case <-ctx.Done():
			return
		default:
			log.Println("Performing initial Google Drive photo check.")
			if err := ProcessDrivePhotos(); err != nil {
				log.Printf("Error during initial Google Drive photo check: %v", err)
			}
		}
	}()

	// Start periodic checks
	ticker := time.NewTicker(interval)
	go func() {
		defer log.Println("Periodic Google Drive photo check goroutine terminated.")
		for {
			select {
			case <-ctx.Done():
				ticker.Stop()
				return
			case <-ticker.C:
				log.Printf("Performing periodic Google Drive photo check (every %d minutes).", cfg.Google.Drive.CheckIntervalMinutes)
				if err := ProcessDrivePhotos(); err != nil {
					log.Printf("Error during periodic Google Drive photo check: %v", err)
				}
			}
		}
	}()
	return cancel
}

var driveService *drive.Service

// NewPhotoDownloadedChan is a channel to signal when a new photo has been downloaded.
var NewPhotoDownloadedChan = make(chan bool, 1) // Buffered channel to avoid blocking sender

// NotifyNewPhotoDownloaded sends a signal on the channel that a new photo has been downloaded.
func NotifyNewPhotoDownloaded() {
	select {
	case NewPhotoDownloadedChan <- true:
		// Sent successfully
	default:
		// Channel is full, meaning the receiver hasn't processed the previous signal yet.
		// This is fine, we just want to signal that an update is needed.
	}
}

// InitGoogleDriveService initializes the Google Drive API client.
func InitGoogleDriveService() error {
	client, err := google.GetGoogleHTTPClient()
	if err != nil {
		return fmt.Errorf("unable to get unified Google client for Drive: %w", err)
	}

	srv, err := drive.New(client)
	if err != nil {
		return fmt.Errorf("unable to retrieve Drive client: %w", err)
	}
	driveService = srv
	return nil
}

// getDriveFolderID resolves a folder name or ID into a unique folder ID.
// If the input is already a valid-looking ID, it's returned.
// If it's a name, it searches for a folder with that name.
func getDriveFolderID(nameOrID string) (string, error) {
	// Simple check to see if it looks like a Google Drive ID.
	if len(nameOrID) > 20 && !strings.Contains(nameOrID, " ") {
		return nameOrID, nil
	}

	// It's likely a name, so search for it.
	query := fmt.Sprintf("mimeType='application/vnd.google-apps.folder' and name='%s' and trashed=false", nameOrID)
	r, err := driveService.Files.List().Q(query).Fields("files(id, name)").Do()
	if err != nil {
		return "", fmt.Errorf("unable to search for folder named '%s': %w", nameOrID, err)
	}

	if len(r.Files) == 0 {
		return "", fmt.Errorf("no folder named '%s' found in Google Drive. Please use the folder ID from the URL", nameOrID)
	}
	if len(r.Files) > 1 {
		return "", fmt.Errorf("multiple folders named '%s' found. Please use the unique folder ID from the URL instead of the name", nameOrID)
	}

	return r.Files[0].Id, nil
}

// ProcessDrivePhotos checks configured Google Drive folders for new photos,
// downloads them to a local directory.
func ProcessDrivePhotos() error {
	driveConfig := config.GetConfig().Google.Drive
	localDir := config.GetConfig().LocalPhotos.Directory

	if driveService == nil {
		return fmt.Errorf("drive service not initialized")
	}
	if len(driveConfig.SourceFolderIDs) == 0 {
		return fmt.Errorf("Google Drive Source Folder IDs are not configured. Please set at least one value in config.toml to enable photo processing.")
	}

	var allErrors []error

	for _, sourceFolderID := range driveConfig.SourceFolderIDs {
		// 1. Resolve the folder name/ID to a unique ID.
		folderID, err := getDriveFolderID(sourceFolderID)
		if err != nil {
			allErrors = append(allErrors, fmt.Errorf("error resolving folder ID '%s': %w", sourceFolderID, err))
			continue // Skip to next folder
		}
		log.Printf("Processing Google Drive folder: %s (ID: %s)", sourceFolderID, folderID)

		// 2. Find files in the specified folder, handling pagination.
		var allFiles []*drive.File
		pageToken := ""
		for {
			query := fmt.Sprintf("'%s' in parents and trashed = false", folderID)
			req := driveService.Files.List().Q(query).
				Fields("nextPageToken, files(id, name, mimeType)")
			if pageToken != "" {
				req.PageToken(pageToken)
			}
			r, err := req.Do()
			if err != nil {
				allErrors = append(allErrors, fmt.Errorf("unable to retrieve files from Drive folder ID '%s': %w", folderID, err))
				break // Break from pagination loop on error
			}

			allFiles = append(allFiles, r.Files...)

			if r.NextPageToken == "" {
				break // No more pages
			}
			pageToken = r.NextPageToken
		}

		if err != nil {
			continue // Already logged error, so just move to the next folder
		}

		if len(allFiles) == 0 {
			log.Printf("No new photos found in Google Drive folder '%s'.", sourceFolderID)
			continue
		}

		log.Printf("Found %d files in Google Drive folder '%s'.", len(allFiles), sourceFolderID)

		for _, f := range allFiles {
			// Check if file is already downloaded and tracked in DB
			isDownloaded, err := database.IsDrivePhotoDownloaded(f.Id)
			if err != nil {
				allErrors = append(allErrors, fmt.Errorf("error checking download status for Drive file %s: %w", f.Name, err))
				continue
			}
			if isDownloaded {
				log.Printf("Skipping already downloaded file: %s from folder '%s'.", f.Name, sourceFolderID)
				continue
			}

			// 3. Check if it's an image.
			if !strings.HasPrefix(f.MimeType, "image/") {
				log.Printf("Skipping non-image file in folder '%s': %s (%s)", sourceFolderID, f.Name, f.MimeType)
				continue
			}

			// 4. Download the file.
			log.Printf("Downloading %s from folder '%s'...", f.Name, sourceFolderID)
			resp, err := driveService.Files.Get(f.Id).Download()
			if err != nil {
				allErrors = append(allErrors, fmt.Errorf("error downloading file %s from folder '%s': %w", f.Name, sourceFolderID, err))
				continue // Skip to the next file
			}
			defer resp.Body.Close()

			// 5. Save the file locally.
			localPath := filepath.Join(localDir, f.Name)
			out, err := os.Create(localPath)
			if err != nil {
				allErrors = append(allErrors, fmt.Errorf("error creating local file %s for photo from '%s': %w", localPath, sourceFolderID, err))
				continue
			}
			defer out.Close()

			_, err = io.Copy(out, resp.Body)
			if err != nil {
				allErrors = append(allErrors, fmt.Errorf("error saving file %s for photo from '%s': %w", localPath, sourceFolderID, err))
				continue
			}
			log.Printf("Successfully saved %s from folder '%s'.", localPath, sourceFolderID)
			NotifyNewPhotoDownloaded() // Signal that a new photo has been downloaded.

			// Mark photo as downloaded in DB
			if err := database.SetDrivePhotoDownloaded(f.Id, true); err != nil {
				allErrors = append(allErrors, fmt.Errorf("error marking Drive file %s as downloaded: %w", f.Name, err))
				// Continue processing, but log the error
			}

			// 6. Delete the file from Google Drive.
			// NOTE: Per user request, photos are NOT deleted from Drive from shared folders.
			// err = driveService.Files.Delete(f.Id).Do()
			// if err != nil {
			// 	log.Printf("Error deleting file %s from Drive: %v", f.Name, err)
			// } else {
			// 	log.Printf("Successfully deleted %s from Google Drive.", f.Name)
		}
		// Introduce a small delay after processing each folder to yield CPU
		time.Sleep(50 * time.Millisecond)
	}

	// Clean up old hidden photos once after processing all folders
	CleanupHiddenPhotos(localDir)

	if len(allErrors) > 0 {
		return fmt.Errorf("encountered errors during Google Drive photo processing: %v", allErrors)
	}
	return nil
}
