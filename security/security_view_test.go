//   Copyright 2026 - Brady Catherman
//
//   Licensed under the Apache License, Version 2.0 (the "License");
//   you may not use this file except in compliance with the License.
//   You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
//   Unless required by applicable law or agreed to in writing, software
//   distributed under the License is distributed on an "AS IS" BASIS,
//   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//   See the License for the specific language governing permissions and
//   limitations under the License.

package security

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/liquidgecka/homehub/config"
)

func TestFrigateCamera_Login(t *testing.T) {
	tests := []struct {
		name          string
		handler       http.HandlerFunc
		expectError   bool
		expectedToken string
		expectedError string
	}{
		{
			name: "Success",
			handler: func(w http.ResponseWriter, r *http.Request) {
				cookie := http.Cookie{
					Name:  "frigate_token",
					Value: "test-token",
				}
				http.SetCookie(w, &cookie)
				w.WriteHeader(http.StatusOK)
			},
			expectError:   false,
			expectedToken: "test-token",
		},
		{
			name: "API Error",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusUnauthorized)
			},
			expectError:   true,
			expectedError: "login failed with status code 401",
		},
		{
			name: "Missing Cookie",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			},
			expectError:   true,
			expectedError: "frigate_token cookie not found in login response",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(tt.handler)
			defer server.Close()

			cam := &frigateCamera{
				CameraConfig: &config.CameraConfig{
					URL:      server.URL,
					Username: "test-user",
					Password: "test-password",
				},
			}

			err := cam.login()

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected an error, but got nil")
				}
				if !strings.Contains(err.Error(), tt.expectedError) {
					t.Errorf("Expected error message to contain '%s', but got '%s'", tt.expectedError, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error, but got: %v", err)
				}
				if cam.token != tt.expectedToken {
					t.Errorf("Expected token to be '%s', but got '%s'", tt.expectedToken, cam.token)
				}
			}
		})
	}
}

func TestSecurityView_Stop(t *testing.T) {
	s := &securityView{
		stops: []chan struct{}{
			make(chan struct{}),
			make(chan struct{}),
		},
	}

	s.Stop()

	for i, ch := range s.stops {
		select {
		case <-ch:
			// Channel is closed, as expected
		case <-time.After(1 * time.Second):
			t.Errorf("Channel %d was not closed", i)
		}
	}
}
