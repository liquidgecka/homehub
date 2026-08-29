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

package database

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/liquidgecka/homehub/config"
)

// BackupInfo contains metadata about a database backup file.
type BackupInfo struct {
	Filename           string    `json:"filename"`
	Path               string    `json:"path"`
	Size               int64     `json:"size"`
	SizeFormatted      string    `json:"size_formatted"`
	CreatedAt          time.Time `json:"created_at"`
	CreatedAtFormatted string    `json:"created_at_formatted"`
}

// ResolveBackupDirectory returns the resolved backup directory path,
// expanding any leading `~` or defaulting to ~/.local/homehub/backups.
func ResolveBackupDirectory(configuredDir string) (string, error) {
	if configuredDir == "" {
		usrHome, err := osUserHomeDir()
		if err != nil {
			return "", fmt.Errorf("unable to get home directory: %w", err)
		}
		return filepath.Join(usrHome, ".local", "homehub", "backups"), nil
	}

	if strings.HasPrefix(configuredDir, "~/") || configuredDir == "~" {
		usrHome, err := osUserHomeDir()
		if err != nil {
			return "", fmt.Errorf("unable to get home directory: %w", err)
		}
		if configuredDir == "~" {
			return usrHome, nil
		}
		return filepath.Join(usrHome, configuredDir[2:]), nil
	}

	return filepath.Clean(configuredDir), nil
}

// FormatBytes converts a byte count into a human-readable string.
func FormatBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}

// CreateBackup creates a new zip backup of the SQLite database in backupDir.
func CreateBackup(backupDir string) (*BackupInfo, error) {
	resolvedDir, err := ResolveBackupDirectory(backupDir)
	if err != nil {
		return nil, err
	}

	if err := os.MkdirAll(resolvedDir, 0700); err != nil {
		return nil, fmt.Errorf("unable to create backup directory: %w", err)
	}

	dbPath, err := GetDBPath()
	if err != nil {
		return nil, err
	}

	now := time.Now()
	timestampStr := now.Format("20060102_150405")
	backupFilename := fmt.Sprintf("homehub_backup_%s.zip", timestampStr)
	finalBackupPath := filepath.Join(resolvedDir, backupFilename)
	tempBackupPath := filepath.Join(
		resolvedDir,
		fmt.Sprintf(".tmp_%s_%d.zip", timestampStr, now.UnixNano()),
	)

	// Obtain database file content
	tempVacuumDB := filepath.Join(
		resolvedDir,
		fmt.Sprintf(".tmp_vacuum_%s_%d.db", timestampStr, now.UnixNano()),
	)
	var sourceDBPath string
	vacuumSucceeded := false

	if db != nil {
		// Use SQLite VACUUM INTO for a consistent snapshot if DB is active
		_, vErr := db.Exec(fmt.Sprintf("VACUUM INTO '%s'", tempVacuumDB))
		if vErr == nil {
			sourceDBPath = tempVacuumDB
			vacuumSucceeded = true
			defer os.Remove(tempVacuumDB)
		}
	}

	if !vacuumSucceeded {
		// Fall back to reading production db file directly
		if _, statErr := os.Stat(dbPath); os.IsNotExist(statErr) {
			return nil, fmt.Errorf("database file does not exist at %s", dbPath)
		}
		sourceDBPath = dbPath
	}

	srcFile, err := os.Open(sourceDBPath)
	if err != nil {
		return nil, fmt.Errorf("unable to read source database: %w", err)
	}
	defer srcFile.Close()

	srcStat, err := srcFile.Stat()
	if err != nil {
		return nil, fmt.Errorf("unable to stat source database: %w", err)
	}

	// Create zip archive
	zipFile, err := os.OpenFile(
		tempBackupPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600,
	)
	if err != nil {
		return nil, fmt.Errorf("unable to create temp backup file: %w", err)
	}

	zipWriter := zip.NewWriter(zipFile)
	header, err := zip.FileInfoHeader(srcStat)
	if err != nil {
		zipFile.Close()
		os.Remove(tempBackupPath)
		return nil, fmt.Errorf("failed to create zip header: %w", err)
	}
	header.Name = "homehub.db"
	header.Method = zip.Deflate

	writer, err := zipWriter.CreateHeader(header)
	if err != nil {
		zipFile.Close()
		os.Remove(tempBackupPath)
		return nil, fmt.Errorf("failed to create zip entry: %w", err)
	}

	if _, err := io.Copy(writer, srcFile); err != nil {
		zipFile.Close()
		os.Remove(tempBackupPath)
		return nil, fmt.Errorf("failed to write zip content: %w", err)
	}

	if err := zipWriter.Close(); err != nil {
		zipFile.Close()
		os.Remove(tempBackupPath)
		return nil, fmt.Errorf("failed to close zip writer: %w", err)
	}
	if err := zipFile.Close(); err != nil {
		os.Remove(tempBackupPath)
		return nil, fmt.Errorf("failed to close zip file: %w", err)
	}

	// Rename temp zip to final destination
	if err := os.Rename(tempBackupPath, finalBackupPath); err != nil {
		os.Remove(tempBackupPath)
		return nil, fmt.Errorf("failed to finalize backup file: %w", err)
	}

	fi, err := os.Stat(finalBackupPath)
	if err != nil {
		return nil, fmt.Errorf("failed to stat final backup: %w", err)
	}

	info := &BackupInfo{
		Filename:           backupFilename,
		Path:               finalBackupPath,
		Size:               fi.Size(),
		SizeFormatted:      FormatBytes(fi.Size()),
		CreatedAt:          now,
		CreatedAtFormatted: now.Format("Jan 02, 2006 3:04 PM"),
	}

	log.Printf(
		"Database backup created successfully: %s (%s)",
		backupFilename, info.SizeFormatted,
	)
	return info, nil
}

// ListBackups scans the backup directory for all .zip files.
// Backups are detected purely by the presence of a zip file in the directory.
func ListBackups(backupDir string) ([]BackupInfo, error) {
	resolvedDir, err := ResolveBackupDirectory(backupDir)
	if err != nil {
		return nil, err
	}

	if _, err := os.Stat(resolvedDir); os.IsNotExist(err) {
		return []BackupInfo{}, nil
	}

	entries, err := os.ReadDir(resolvedDir)
	if err != nil {
		return nil, fmt.Errorf("unable to read backup directory: %w", err)
	}

	var backups []BackupInfo
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(strings.ToLower(name), ".zip") {
			continue
		}
		if strings.HasPrefix(name, ".") {
			continue // Skip hidden/temp files
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		createdAt := info.ModTime()
		// Try parsing timestamp from standard filename format
		// e.g. homehub_backup_20060102_150405.zip
		cleanName := strings.TrimSuffix(name, filepath.Ext(name))
		parts := strings.Split(cleanName, "_")
		if len(parts) >= 3 {
			datePart := parts[len(parts)-2]
			timePart := parts[len(parts)-1]
			if parsed, pErr := time.ParseInLocation(
				"20060102_150405", datePart+"_"+timePart, time.Local,
			); pErr == nil {
				createdAt = parsed
			}
		}

		backups = append(backups, BackupInfo{
			Filename:           name,
			Path:               filepath.Join(resolvedDir, name),
			Size:               info.Size(),
			SizeFormatted:      FormatBytes(info.Size()),
			CreatedAt:          createdAt,
			CreatedAtFormatted: createdAt.Format("Jan 02, 2006 3:04 PM"),
		})
	}

	// Sort backups descending by creation time (most recent first)
	sort.Slice(backups, func(i, j int) bool {
		return backups[i].CreatedAt.After(backups[j].CreatedAt)
	})

	return backups, nil
}

// PruneOldBackups removes backup zip files older than retentionDays.
func PruneOldBackups(backupDir string, retentionDays int) (int, error) {
	if retentionDays <= 0 {
		retentionDays = 30
	}

	backups, err := ListBackups(backupDir)
	if err != nil {
		return 0, err
	}

	cutoff := time.Now().AddDate(0, 0, -retentionDays)
	deletedCount := 0

	for _, b := range backups {
		if b.CreatedAt.Before(cutoff) {
			if err := os.Remove(b.Path); err != nil {
				log.Printf(
					"Warning: Failed to prune backup %s: %v", b.Path, err,
				)
			} else {
				deletedCount++
				log.Printf("Pruned old backup: %s", b.Filename)
			}
		}
	}

	return deletedCount, nil
}

// RestoreBackup restores the SQLite database from a zip backup archive.
// It does NOT require the current database connection to be functional.
func RestoreBackup(backupPath string) error {
	zipReader, err := zip.OpenReader(backupPath)
	if err != nil {
		return fmt.Errorf("unable to open backup zip archive: %w", err)
	}
	defer zipReader.Close()

	var dbFileEntry *zip.File
	for _, file := range zipReader.File {
		if file.FileInfo().IsDir() {
			continue
		}
		if file.Name == "homehub.db" ||
			strings.HasSuffix(strings.ToLower(file.Name), ".db") {
			dbFileEntry = file
			break
		}
	}

	if dbFileEntry == nil && len(zipReader.File) > 0 {
		for _, file := range zipReader.File {
			if !file.FileInfo().IsDir() {
				dbFileEntry = file
				break
			}
		}
	}

	if dbFileEntry == nil {
		return fmt.Errorf("no database file found inside backup archive")
	}

	dbPath, err := GetDBPath()
	if err != nil {
		return err
	}

	homehubDir := filepath.Dir(dbPath)
	if err := os.MkdirAll(homehubDir, 0700); err != nil {
		return fmt.Errorf("unable to create target directory: %w", err)
	}

	tempExtracted := filepath.Join(
		homehubDir, fmt.Sprintf(".restore_%d.tmp", time.Now().UnixNano()),
	)

	rc, err := dbFileEntry.Open()
	if err != nil {
		return fmt.Errorf("unable to open zip entry: %w", err)
	}
	defer rc.Close()

	// Read and validate SQLite header (16 bytes: "SQLite format 3\x00")
	headerBuf := make([]byte, 16)
	n, err := io.ReadFull(rc, headerBuf)
	if err != nil || n < 16 {
		return fmt.Errorf("corrupt database file in backup: invalid header")
	}
	expectedHeader := []byte("SQLite format 3\x00")
	if !bytes.Equal(headerBuf, expectedHeader) {
		return fmt.Errorf("file in backup is not a valid SQLite database")
	}

	out, err := os.OpenFile(
		tempExtracted, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600,
	)
	if err != nil {
		return fmt.Errorf("unable to create temp restore file: %w", err)
	}

	if _, err := out.Write(headerBuf); err != nil {
		out.Close()
		os.Remove(tempExtracted)
		return fmt.Errorf(
			"failed to write header to temp restore file: %w", err,
		)
	}

	if _, err := io.Copy(out, rc); err != nil {
		out.Close()
		os.Remove(tempExtracted)
		return fmt.Errorf("failed to extract database from backup: %w", err)
	}
	out.Close()

	// Safely close the existing DB connection if one is active
	CloseDB()

	// Replace the production database file
	if err := os.Rename(tempExtracted, dbPath); err != nil {
		os.Remove(tempExtracted)
		return fmt.Errorf("failed to replace database file: %w", err)
	}

	// Remove WAL and SHM journal files from previous DB state
	os.Remove(dbPath + "-wal")
	os.Remove(dbPath + "-shm")

	// Reopen the newly restored database
	if err := OpenFileDB(); err != nil {
		return fmt.Errorf("database restored but failed to reopen: %w", err)
	}
	if err := InitDB(); err != nil {
		return fmt.Errorf("database restored but schema init failed: %w", err)
	}

	log.Printf("Database successfully restored from %s", backupPath)
	return nil
}

// ScheduledBackupTask performs a backup and prunes old backups.
func ScheduledBackupTask(cfg *config.Config) error {
	if cfg == nil {
		cfg = config.GetConfig()
	}
	if !cfg.Database.IsBackupEnabled() {
		log.Println("Database backup is disabled in configuration.")
		return nil
	}

	backupDir := cfg.Database.BackupDirectory
	if _, err := CreateBackup(backupDir); err != nil {
		log.Printf("ERROR: Scheduled database backup failed: %v", err)
		return err
	}

	retention := cfg.Database.BackupRetentionDays
	if retention <= 0 {
		retention = 30
	}
	if _, err := PruneOldBackups(backupDir, retention); err != nil {
		log.Printf("ERROR: Database backup pruning failed: %v", err)
		return err
	}

	return nil
}
