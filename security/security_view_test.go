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

package security

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"

	"github.com/liquidgecka/homehub/config"
)

func createTestJPEGBytes() []byte {
	img := image.NewRGBA(image.Rect(0, 0, 10, 10))
	for x := 0; x < 10; x++ {
		for y := 0; y < 10; y++ {
			img.Set(x, y, color.RGBA{R: 255, G: 0, B: 0, A: 255})
		}
	}
	var buf bytes.Buffer
	_ = jpeg.Encode(&buf, img, nil)
	return buf.Bytes()
}

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
					t.Errorf(
						"Expected error message to contain '%s', but got '%s'",
						tt.expectedError, err.Error(),
					)
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error, but got: %v", err)
				}
				if cam.token != tt.expectedToken {
					t.Errorf(
						"Expected token to be '%s', but got '%s'",
						tt.expectedToken, cam.token,
					)
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

func TestSecurityView_NewAndFetchImage(t *testing.T) {
	test.NewApp()
	win := test.NewWindow(widget.NewLabel("Security Test"))
	defer win.Close()

	// 1. No cameras configured
	config.SetMockConfig(config.Config{
		Security: config.SecurityConfig{
			Camera: []config.CameraConfig{},
		},
	})

	secViewEmpty := New(win)
	if secViewEmpty == nil || secViewEmpty.GetContent() == nil {
		t.Fatal("expected non-nil security view for empty config")
	}

	// 2. Camera configured with mock server
	jpegData := createTestJPEGBytes()
	server := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.HasSuffix(r.URL.Path, "/api/login") {
				http.SetCookie(
					w, &http.Cookie{Name: "frigate_token", Value: "tok"},
				)
				w.WriteHeader(http.StatusOK)
				return
			}
			w.Header().Set("Content-Type", "image/jpeg")
			_, _ = w.Write(jpegData)
		}),
	)
	defer server.Close()

	config.SetMockConfig(config.Config{
		Security: config.SecurityConfig{
			Camera: []config.CameraConfig{
				{
					Name:     "Front Door",
					Type:     "frigate",
					URL:      server.URL,
					Refresh:  "50ms",
					Username: "user",
					Password: "pass",
				},
				{
					Name:     "Backyard",
					Type:     "generic",
					URL:      server.URL,
					Refresh:  "invalid-duration",
					Username: "user",
					Password: "pass",
				},
			},
		},
	})

	secView := New(win)
	if secView == nil || secView.GetContent() == nil {
		t.Fatal("expected non-nil security view with cameras")
	}

	// Wait briefly for fetchImage to run
	time.Sleep(100 * time.Millisecond)
	secView.Stop()
}

func TestTappableAndMin(t *testing.T) {
	if min(5*time.Second, 10*time.Second) != 5*time.Second {
		t.Errorf("min failed for 5s < 10s")
	}
	if min(15*time.Second, 10*time.Second) != 10*time.Second {
		t.Errorf("min failed for 15s > 10s")
	}

	tappedCalled := false
	tap := &tappable{
		onTap: func() {
			tappedCalled = true
		},
	}
	r := tap.CreateRenderer()
	r.Layout(fyne.NewSize(100, 100))
	r.Refresh()
	if r.MinSize().Width != 0 || r.MinSize().Height != 0 {
		t.Errorf("expected min size 0,0")
	}
	if r.Objects() != nil {
		t.Errorf("expected nil objects")
	}
	r.Destroy()

	tap.Tapped(&fyne.PointEvent{})
	if !tappedCalled {
		t.Errorf("expected onTap to be called")
	}
	tap.TappedSecondary(&fyne.PointEvent{})
	if tap.Cursor() != desktop.PointerCursor {
		t.Errorf("expected PointerCursor, got %v", tap.Cursor())
	}
}
