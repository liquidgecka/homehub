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

package google

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"sync"
	"testing"

	"github.com/liquidgecka/homehub/config"
	"golang.org/x/oauth2"
	"google.golang.org/api/calendar/v3"
	"google.golang.org/api/drive/v3"
	"google.golang.org/api/option"
)

// Helper function to create a dummy service account key file
func createDummyServiceAccountKey(t *testing.T) (string, func()) {
	t.Helper()
	tempFile, err := ioutil.TempFile("", "service-account-key-*.json")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer tempFile.Close()

	// Minimal valid service account key structure
	key := map[string]string{
		"type":                        "service_account",
		"project_id":                  "test-project",
		"private_key_id":              "123",
		"private_key":                 "-----BEGIN PRIVATE KEY-----\n...\n-----END PRIVATE KEY-----\n",
		"client_email":                "test@test-project.iam.gserviceaccount.com",
		"client_id":                   "test-client-id",
		"auth_uri":                    "https://accounts.google.com/o/oauth2/auth",
		"token_uri":                   "https://oauth2.googleapis.com/token",
		"auth_provider_x509_cert_url": "https://www.googleapis.com/oauth2/v1/certs",
		"client_x509_cert_url":        "https://www.googleapis.com/robot/v1/metadata/x509/test%40test-project.iam.gserviceaccount.com",
		"universe_domain":             "googleapis.com",
	}

	encoder := json.NewEncoder(tempFile)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(key); err != nil {
		t.Fatalf("Failed to write dummy key to file: %v", err)
	}

	return tempFile.Name(), func() { os.Remove(tempFile.Name()) }
}

// resetGoogleClientGlobals resets the global variables used by GetGoogleHTTPClient
// to ensure a clean state for each test.
func resetGoogleClientGlobals() {
	GoogleClientInitOnce = sync.Once{}
	GoogleHTTPClient = nil
	GoogleClientInitErr = nil
}

func TestGetGoogleHTTPClient_ServiceAccount(t *testing.T) {
	resetGoogleClientGlobals()
	// Create a dummy service account key file
	keyFilePath, cleanupKeyFile := createDummyServiceAccountKey(t)
	defer cleanupKeyFile()

	// Set up a mock config
	mockConfig := config.DefaultConfig()
	mockConfig.Google.ServiceAccountKeyFile = keyFilePath
	cleanupConfig := config.SetMockConfig(mockConfig)
	defer cleanupConfig()

	client, err := GetGoogleHTTPClient()
	if err != nil {
		t.Fatalf("GetGoogleHTTPClient failed: %v", err)
	}
	if client == nil {
		t.Fatal("GetGoogleHTTPClient returned a nil client")
	}

	if _, ok := client.Transport.(*oauth2.Transport); !ok {
		t.Errorf("Expected client transport to be *oauth2.Transport, got %T", client.Transport)
	}
}

// Test for GetGoogleHTTPClient with missing service account key file
func TestGetGoogleHTTPClient_MissingKeyFile(t *testing.T) {
	resetGoogleClientGlobals()
	// Set up a mock config with a non-existent key file
	mockConfig := config.DefaultConfig()
	mockConfig.Google.ServiceAccountKeyFile = "/path/to/nonexistent/key.json"
	cleanupConfig := config.SetMockConfig(mockConfig)
	defer cleanupConfig()

	client, err := GetGoogleHTTPClient()
	if err == nil {
		t.Fatal("GetGoogleHTTPClient unexpectedly succeeded with a missing key file")
	}
	if client != nil {
		t.Error("GetGoogleHTTPClient returned a non-nil client with a missing key file")
	}
}

// Test for GetGoogleHTTPClient with empty service account key file path
func TestGetGoogleHTTPClient_EmptyKeyFilePath(t *testing.T) {
	resetGoogleClientGlobals()
	// Set up a mock config with an empty key file path
	mockConfig := config.DefaultConfig()
	mockConfig.Google.ServiceAccountKeyFile = ""
	cleanupConfig := config.SetMockConfig(mockConfig)
	defer cleanupConfig()

	client, err := GetGoogleHTTPClient()
	if err == nil {
		t.Fatal("GetGoogleHTTPClient unexpectedly succeeded with an empty key file path")
	}
	if client != nil {
		t.Error("GetGoogleHTTPClient returned a non-nil client with an empty key file path")
	}
}

// Test to ensure GetGoogleHTTPClient is a singleton
func TestGetGoogleHTTPClient_Singleton(t *testing.T) {
	resetGoogleClientGlobals()
	keyFilePath, cleanupKeyFile := createDummyServiceAccountKey(t)
	defer cleanupKeyFile()

	mockConfig := config.DefaultConfig()
	mockConfig.Google.ServiceAccountKeyFile = keyFilePath
	cleanupConfig := config.SetMockConfig(mockConfig)
	defer cleanupConfig()

	client1, err := GetGoogleHTTPClient()
	if err != nil {
		t.Fatalf("First call to GetGoogleHTTPClient failed: %v", err)
	}

	// Change the mock config's key file path after the first call
	// This change should NOT affect the client returned by subsequent calls if it's a singleton
	mockConfig.Google.ServiceAccountKeyFile = "/path/to/another/nonexistent/key.json"
	config.SetMockConfig(mockConfig) // Update the mock config

	client2, err := GetGoogleHTTPClient()
	if err != nil {
		t.Fatalf("Second call to GetGoogleHTTPClient failed: %v", err)
	}

	if client1 != client2 {
		t.Error("GetGoogleHTTPClient did not return the same client instance on subsequent calls (singleton pattern failed)")
	}
}

// Test for Calendar service creation
func TestNewCalendarService(t *testing.T) {
	resetGoogleClientGlobals()
	keyFilePath, cleanupKeyFile := createDummyServiceAccountKey(t)
	defer cleanupKeyFile()

	mockConfig := config.DefaultConfig()
	mockConfig.Google.ServiceAccountKeyFile = keyFilePath
	cleanupConfig := config.SetMockConfig(mockConfig)
	defer cleanupConfig()

	srv, err := NewCalendarService(context.Background())
	if err != nil {
		t.Fatalf("NewCalendarService failed: %v", err)
	}
	if srv == nil {
		t.Fatal("NewCalendarService returned a nil service")
	}
}

// Test for Drive service creation
func TestNewDriveService(t *testing.T) {
	resetGoogleClientGlobals()
	keyFilePath, cleanupKeyFile := createDummyServiceAccountKey(t)
	defer cleanupKeyFile()

	mockConfig := config.DefaultConfig()
	mockConfig.Google.ServiceAccountKeyFile = keyFilePath
	cleanupConfig := config.SetMockConfig(mockConfig)
	defer cleanupConfig()

	srv, err := NewDriveService(context.Background())
	if err != nil {
		t.Fatalf("NewDriveService failed: %v", err)
	}
	if srv == nil {
		t.Fatal("NewDriveService returned a nil service")
	}
}

// NewCalendarService creates a new Google Calendar service client.
func NewCalendarService(ctx context.Context) (*calendar.Service, error) {
	httpClient, err := GetGoogleHTTPClient()
	if err != nil {
		return nil, fmt.Errorf("failed to get Google HTTP client: %w", err)
	}
	srv, err := calendar.NewService(ctx, option.WithHTTPClient(httpClient))
	if err != nil {
		return nil, fmt.Errorf("failed to create Google Calendar service: %w", err)
	}
	return srv, nil
}

// NewDriveService creates a new Google Drive service client.
func NewDriveService(ctx context.Context) (*drive.Service, error) {
	httpClient, err := GetGoogleHTTPClient()
	if err != nil {
		return nil, fmt.Errorf("failed to get Google HTTP client: %w", err)
	}
	srv, err := drive.NewService(ctx, option.WithHTTPClient(httpClient))
	if err != nil {
		return nil, fmt.Errorf("failed to create Google Drive service: %w", err)
	}
	return srv, nil
}
