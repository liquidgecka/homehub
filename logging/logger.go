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

package logging

import (
	"fmt"
	"io"
	"log"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/liquidgecka/homehub/config"
)

// RotatingWriter implements io.WriteCloser and automatically rotates log files
// when they exceed a configured size, maintaining a specified retention count.
type RotatingWriter struct {
	mu         sync.Mutex
	dir        string
	filename   string
	maxBytes   int64
	maxBackups int
	file       *os.File
	size       int64
}

// NewRotatingWriter creates a new RotatingWriter using maximum size in megabytes.
func NewRotatingWriter(dir, filename string, maxSizeMB, maxBackups int) (*RotatingWriter, error) {
	if maxSizeMB <= 0 {
		maxSizeMB = 10
	}
	return NewRotatingWriterWithBytes(dir, filename, int64(maxSizeMB)*1024*1024, maxBackups)
}

// NewRotatingWriterWithBytes creates a new RotatingWriter with an exact byte threshold.
func NewRotatingWriterWithBytes(dir, filename string, maxBytes int64, maxBackups int) (*RotatingWriter, error) {
	expandedDir := ExpandPath(dir)
	if err := os.MkdirAll(expandedDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create log directory '%s': %w", expandedDir, err)
	}

	if filename == "" {
		filename = "homehub.log"
	}
	if maxBytes <= 0 {
		maxBytes = 10 * 1024 * 1024
	}
	if maxBackups <= 0 {
		maxBackups = 10
	}

	rw := &RotatingWriter{
		dir:        expandedDir,
		filename:   filename,
		maxBytes:   maxBytes,
		maxBackups: maxBackups,
	}

	return rw, nil
}

// Write writes log data to the active log file, rotating if the file exceeds maxBytes.
func (w *RotatingWriter) Write(p []byte) (n int, err error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	writeLen := int64(len(p))

	if w.file == nil {
		if err := w.openFile(); err != nil {
			return 0, err
		}
	}

	if w.size+writeLen > w.maxBytes {
		if err := w.rotate(); err != nil {
			return 0, err
		}
	}

	n, err = w.file.Write(p)
	w.size += int64(n)
	return n, err
}

// Close closes the currently open log file.
func (w *RotatingWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.file != nil {
		err := w.file.Close()
		w.file = nil
		return err
	}
	return nil
}

// openFile opens the active log file in append mode and records its current size.
func (w *RotatingWriter) openFile() error {
	fullPath := filepath.Join(w.dir, w.filename)
	info, err := os.Stat(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			w.size = 0
		} else {
			return err
		}
	} else {
		w.size = info.Size()
	}

	f, err := os.OpenFile(fullPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("failed to open log file '%s': %w", fullPath, err)
	}
	w.file = f
	return nil
}

// rotate closes the current log file, shifts existing backup files, and opens a fresh log file.
func (w *RotatingWriter) rotate() error {
	if w.file != nil {
		_ = w.file.Close()
		w.file = nil
	}

	fullPath := filepath.Join(w.dir, w.filename)

	// Remove excess backups and shift existing backups down
	for i := w.maxBackups; i >= 1; i-- {
		src := fmt.Sprintf("%s.%d", fullPath, i)
		dst := fmt.Sprintf("%s.%d", fullPath, i+1)
		if i == w.maxBackups {
			_ = os.Remove(src)
		}
		if _, err := os.Stat(src); err == nil {
			_ = os.Rename(src, dst)
		}
	}

	// Rename current log file to .1
	dst1 := fmt.Sprintf("%s.1", fullPath)
	_ = os.Rename(fullPath, dst1)

	// Open a new active file
	return w.openFile()
}

// ExpandPath expands leading ~ with the current user's home directory.
func ExpandPath(path string) string {
	if path == "" {
		return ""
	}
	if strings.HasPrefix(path, "~/") || path == "~" {
		usr, err := user.Current()
		if err == nil && usr.HomeDir != "" {
			if path == "~" {
				return usr.HomeDir
			}
			return filepath.Join(usr.HomeDir, path[2:])
		}
		home := os.Getenv("HOME")
		if home != "" {
			if path == "~" {
				return home
			}
			return filepath.Join(home, path[2:])
		}
	}
	return filepath.Clean(path)
}

// ParseSizeToMB parses size strings like "10M", "10MB", "500K", "1G", "10" into integer megabytes (minimum 1).
func ParseSizeToMB(s string) int {
	s = strings.TrimSpace(strings.ToUpper(s))
	if s == "" {
		return 10
	}
	s = strings.TrimSuffix(s, "IB")
	s = strings.TrimSuffix(s, "B")

	var multiplier float64 = 1.0
	if strings.HasSuffix(s, "G") {
		multiplier = 1024.0
		s = strings.TrimSuffix(s, "G")
	} else if strings.HasSuffix(s, "M") {
		multiplier = 1.0
		s = strings.TrimSuffix(s, "M")
	} else if strings.HasSuffix(s, "K") {
		multiplier = 1.0 / 1024.0
		s = strings.TrimSuffix(s, "K")
	}

	val, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil || val <= 0 {
		return 10
	}

	mb := int(val * multiplier)
	if mb < 1 {
		mb = 1
	}
	return mb
}

// InitLogger initializes the global logger with a RotatingWriter configured from the given Config.
func InitLogger(cfg *config.Config) (*RotatingWriter, error) {
	logCfg := cfg.Logging

	dir := logCfg.Directory
	if dir == "" {
		dir = logCfg.Location
	}
	if dir == "" {
		usr, err := user.Current()
		if err == nil && usr.HomeDir != "" {
			dir = filepath.Join(usr.HomeDir, ".local", "homehub", "logs")
		} else {
			dir = filepath.Join(os.Getenv("HOME"), ".local", "homehub", "logs")
		}
	}

	maxMB := logCfg.RotationSizeMB
	if logCfg.RotationInterval != "" {
		maxMB = ParseSizeToMB(logCfg.RotationInterval)
	}
	if maxMB <= 0 {
		maxMB = 10
	}

	retention := logCfg.RetentionCount
	if retention <= 0 && logCfg.MaxBackups > 0 {
		retention = logCfg.MaxBackups
	}
	if retention <= 0 {
		retention = 10
	}

	filename := logCfg.Filename
	if filename == "" {
		filename = "homehub.log"
	}

	writer, err := NewRotatingWriter(dir, filename, maxMB, retention)
	if err != nil {
		return nil, err
	}

	multi := io.MultiWriter(os.Stdout, writer)
	log.SetOutput(multi)
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)

	log.Printf("Logging initialized: directory=%s, filename=%s, max_size=%dMB, retention=%d",
		writer.dir, writer.filename, maxMB, retention)

	return writer, nil
}
