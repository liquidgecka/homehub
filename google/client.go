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
	"fmt"
	"net/http"
	"os"
	"sync"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/calendar/v3"
	"google.golang.org/api/tasks/v1"

	"github.com/liquidgecka/homehub/config"
)

var (
	GoogleHTTPClient     *http.Client
	GoogleClientInitOnce sync.Once
	GoogleClientInitErr  error
)

// GetGoogleHTTPClient initializes and returns a single, unified, authenticated http.Client for all Google services
// using a service account. It uses a singleton pattern to ensure the client is initialized only once.
func GetGoogleHTTPClient() (*http.Client, error) {
	GoogleClientInitOnce.Do(func() {
		cfg := config.GetConfig()
		if cfg.Google.ServiceAccountKeyFile == "" {
			GoogleClientInitErr = fmt.Errorf("google.service_account_key_file must be set in config.toml")
			return
		}

		keyFile, err := os.ReadFile(cfg.Google.ServiceAccountKeyFile)
		if err != nil {
			GoogleClientInitErr = fmt.Errorf("unable to read service account key file: %w", err)
			return
		}

		creds, err := google.CredentialsFromJSON(context.Background(), keyFile, calendar.CalendarScope, tasks.TasksScope)
		if err != nil {
			GoogleClientInitErr = fmt.Errorf("failed to create credentials from service account key: %w", err)
			return
		}

		GoogleHTTPClient = &http.Client{
			Transport: &oauth2.Transport{
				Source: creds.TokenSource,
			},
		}
	})

	return GoogleHTTPClient, GoogleClientInitErr
}

// NewTasksService creates a new Google Tasks service client.
func NewTasksService() (*tasks.Service, error) {
	client, err := GetGoogleHTTPClient()
	if err != nil {
		return nil, fmt.Errorf("unable to get unified Google client for Tasks: %w", err)
	}
	srv, err := tasks.New(client)
	if err != nil {
		return nil, fmt.Errorf("unable to retrieve Tasks client: %w", err)
	}
	return srv, nil
}
