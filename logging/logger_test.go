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
	"log"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/liquidgecka/homehub/config"
)

func TestRotatingWriter_BasicWrite(t *testing.T) {
	tempDir := t.TempDir()
	rw, err := NewRotatingWriterWithBytes(tempDir, "test.log", 1024, 3)
	if err != nil {
		t.Fatalf("NewRotatingWriterWithBytes failed: %v", err)
	}
	defer rw.Close()

	msg := []byte("Hello, world!\n")
	n, err := rw.Write(msg)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}
	if n != len(msg) {
		t.Errorf("Expected to write %d bytes, wrote %d", len(msg), n)
	}

	content, err := os.ReadFile(filepath.Join(tempDir, "test.log"))
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}
	if string(content) != string(msg) {
		t.Errorf("Expected file content %q, got %q", string(msg), string(content))
	}
}

func TestRotatingWriter_RotationAndRetention(t *testing.T) {
	tempDir := t.TempDir()
	// Set threshold to 50 bytes and retain max 3 backups (.1, .2, .3)
	maxBytes := int64(50)
	maxBackups := 3
	rw, err := NewRotatingWriterWithBytes(tempDir, "app.log", maxBytes, maxBackups)
	if err != nil {
		t.Fatalf("NewRotatingWriterWithBytes failed: %v", err)
	}
	defer rw.Close()

	// Write 5 chunks of 30 bytes each.
	// Chunk 1: 30 bytes -> fits in app.log (size = 30)
	// Chunk 2: 30 bytes -> exceeds 50! Rotates: app.log -> app.log.1, new app.log gets chunk 2 (size = 30)
	// Chunk 3: 30 bytes -> exceeds 50! Rotates: app.log.1 -> app.log.2, app.log -> app.log.1, new app.log gets chunk 3 (size = 30)
	// Chunk 4: 30 bytes -> exceeds 50! Rotates: app.log.2 -> app.log.3, app.log.1 -> app.log.2, app.log -> app.log.1, new app.log gets chunk 4 (size = 30)
	// Chunk 5: 30 bytes -> exceeds 50! Rotates: app.log.3 deleted, .2 -> .3, .1 -> .2, app.log -> .1, new app.log gets chunk 5 (size = 30)
	for i := 1; i <= 5; i++ {
		chunk := fmt.Sprintf("Log Entry %02d - padding text\n", i)
		if _, err := rw.Write([]byte(chunk)); err != nil {
			t.Fatalf("Failed to write chunk %d: %v", i, err)
		}
	}

	// Verify active file exists
	if _, err := os.Stat(filepath.Join(tempDir, "app.log")); err != nil {
		t.Errorf("Active app.log does not exist: %v", err)
	}

	// Verify backups .1, .2, .3 exist
	for i := 1; i <= maxBackups; i++ {
		backupPath := filepath.Join(tempDir, fmt.Sprintf("app.log.%d", i))
		if _, err := os.Stat(backupPath); err != nil {
			t.Errorf("Expected backup %s to exist, err: %v", backupPath, err)
		}
	}

	// Verify backup .4 was deleted and does not exist
	backup4 := filepath.Join(tempDir, "app.log.4")
	if _, err := os.Stat(backup4); !os.IsNotExist(err) {
		t.Errorf("Expected backup %s to NOT exist, but it was found", backup4)
	}

	// Verify active app.log contains Chunk 5
	activeContent, _ := os.ReadFile(filepath.Join(tempDir, "app.log"))
	if string(activeContent) != "Log Entry 05 - padding text\n" {
		t.Errorf("Unexpected active content: %q", string(activeContent))
	}
}

func TestRotatingWriter_ConcurrentWrites(t *testing.T) {
	tempDir := t.TempDir()
	rw, err := NewRotatingWriterWithBytes(tempDir, "concurrent.log", 200, 5)
	if err != nil {
		t.Fatalf("NewRotatingWriterWithBytes failed: %v", err)
	}
	defer rw.Close()

	var wg sync.WaitGroup
	workers := 10
	iterations := 20

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				msg := fmt.Sprintf("Worker %d message %d\n", workerID, j)
				if _, err := rw.Write([]byte(msg)); err != nil {
					t.Errorf("Worker write failed: %v", err)
				}
			}
		}(i)
	}

	wg.Wait()

	// Verify the active log file is readable and non-empty
	info, err := os.Stat(filepath.Join(tempDir, "concurrent.log"))
	if err != nil {
		t.Fatalf("Stat concurrent.log failed: %v", err)
	}
	if info.Size() == 0 {
		t.Errorf("Expected non-empty log file")
	}
}

func TestParseSizeToMB(t *testing.T) {
	tests := []struct {
		input    string
		expected int
	}{
		{"10M", 10},
		{"10MB", 10},
		{"10MiB", 10},
		{"5M", 5},
		{"1G", 1024},
		{"2GB", 2048},
		{"500K", 1},  // rounds to minimum 1 MB
		{"1000K", 1}, // rounds to minimum 1 MB
		{"2048K", 2}, // 2 MB
		{"15", 15},   // plain number
		{"", 10},     // default
		{"invalid", 10},
		{"-5M", 10},
	}

	for _, tc := range tests {
		result := ParseSizeToMB(tc.input)
		if result != tc.expected {
			t.Errorf("ParseSizeToMB(%q) = %d; want %d", tc.input, result, tc.expected)
		}
	}
}

func TestExpandPath(t *testing.T) {
	if ExpandPath("") != "" {
		t.Errorf("ExpandPath(\"\") should be empty")
	}

	absPath := "/var/log/homehub"
	if ExpandPath(absPath) != absPath {
		t.Errorf("ExpandPath(%q) = %q; want %q", absPath, ExpandPath(absPath), absPath)
	}

	tildePath := "~/test/logs"
	expanded := ExpandPath(tildePath)
	if expanded == tildePath || expanded == "" {
		t.Errorf("ExpandPath(%q) did not expand ~: %q", tildePath, expanded)
	}
}

func TestInitLogger(t *testing.T) {
	tempDir := t.TempDir()
	origOut := log.Writer()
	origFlags := log.Flags()
	defer func() {
		log.SetOutput(origOut)
		log.SetFlags(origFlags)
	}()

	cfg := &config.Config{
		Logging: config.LoggingConfig{
			Directory:        tempDir,
			RotationInterval: "5M",
			RetentionCount:   5,
			Filename:         "test_init.log",
		},
	}

	writer, err := InitLogger(cfg)
	if err != nil {
		t.Fatalf("InitLogger failed: %v", err)
	}
	defer writer.Close()

	log.Println("Testing InitLogger output")

	content, err := os.ReadFile(filepath.Join(tempDir, "test_init.log"))
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}
	if len(content) == 0 {
		t.Errorf("Expected non-empty log file after log.Println")
	}
}

func TestNewRotatingWriter_Defaults(t *testing.T) {
	tempDir := t.TempDir()
	rw, err := NewRotatingWriter(tempDir, "", 0, 0)
	if err != nil {
		t.Fatalf("NewRotatingWriter failed: %v", err)
	}
	defer rw.Close()

	if rw.filename != "homehub.log" {
		t.Errorf("Expected default filename 'homehub.log', got %q", rw.filename)
	}
	if rw.maxBytes != 10*1024*1024 {
		t.Errorf("Expected default maxBytes 10MB, got %d", rw.maxBytes)
	}
	if rw.maxBackups != 10 {
		t.Errorf("Expected default maxBackups 10, got %d", rw.maxBackups)
	}
}
