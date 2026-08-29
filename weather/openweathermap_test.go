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

package weather

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/liquidgecka/homehub/config"
)

func TestGetCoordinates(t *testing.T) {
	tests := []struct {
		name          string
		mockResponse  string
		mockStatus    int
		expectError   bool
		expectedLat   float64
		expectedLon   float64
		expectedError string
	}{
		{
			name: "Success",
			mockResponse: `[
				{
					"lat": 34.0522,
					"lon": -118.2437
				}
			]`,
			mockStatus:  http.StatusOK,
			expectError: false,
			expectedLat: 34.0522,
			expectedLon: -118.2437,
		},
		{
			name:          "API Error",
			mockResponse:  `{"cod": "401", "message": "Invalid API key"}`,
			mockStatus:    http.StatusUnauthorized,
			expectError:   true,
			expectedError: "geocoding API request failed with status 401",
		},
		{
			name:          "No coordinates found",
			mockResponse:  `[]`,
			mockStatus:    http.StatusOK,
			expectError:   true,
			expectedError: "no coordinates found for city: TestCity",
		},
		{
			name:          "Invalid JSON",
			mockResponse:  `[`,
			mockStatus:    http.StatusOK,
			expectError:   true,
			expectedError: "failed to unmarshal geocoding response",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(
				http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(tt.mockStatus)
					fmt.Fprintln(w, tt.mockResponse)
				}),
			)
			defer server.Close()

			geocodingAPIURL = server.URL

			lat, lon, err := GetCoordinates("TestCity", "test-api-key")

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected an error, but got nil")
				}
				if !strings.Contains(err.Error(), tt.expectedError) {
					t.Errorf(
						"Expected error to contain '%s', got '%s'",
						tt.expectedError, err.Error(),
					)
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error, but got: %v", err)
				}
				if lat != tt.expectedLat || lon != tt.expectedLon {
					t.Errorf(
						"Expected lat, lon %f, %f, got %f, %f",
						tt.expectedLat, tt.expectedLon, lat, lon,
					)
				}
			}
		})
	}
}

func TestDownloadImage(t *testing.T) {
	// Setup a mock server to serve a dummy image
	server := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("dummy-image-data"))
		}),
	)
	defer server.Close()

	// Create a temporary directory for the downloaded file
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "test.png")

	// Call downloadImage
	err := downloadImage(server.URL, filePath)
	if err != nil {
		t.Fatalf("Expected no error from downloadImage, but got: %v", err)
	}

	// Verify the file was created and contains the correct data
	data, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("Failed to read downloaded file: %v", err)
	}
	if string(data) != "dummy-image-data" {
		t.Errorf(
			"Expected file content 'dummy-image-data', got '%s'",
			string(data),
		)
	}

	// Test case for HTTP error
	t.Run("HTTP Error", func(t *testing.T) {
		server := httptest.NewServer(
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
			}),
		)
		defer server.Close()

		filePath := filepath.Join(tempDir, "test_error.png")
		err := downloadImage(server.URL, filePath)
		if err == nil {
			t.Fatal("Expected an error from downloadImage, but got nil")
		}
		if !strings.Contains(err.Error(), "status: 500") {
			t.Errorf(
				"Expected error to contain 'status: 500', got '%s'",
				err.Error(),
			)
		}
		if _, err := os.Stat(filePath); !os.IsNotExist(err) {
			t.Errorf("Expected file not to be created on HTTP error, but it was")
		}
	})
}

func TestGetWeather_Cache(t *testing.T) {
	weatherCache.mutex.Lock()
	weatherCache.data = &OpenWeather{Lat: 123}
	weatherCache.lastFetched = time.Now()
	weatherCache.mutex.Unlock()

	cfg := &config.Config{
		OpenWeather: config.OpenWeatherConfig{
			RefreshMinutes: 10,
		},
	}

	// Should return from cache
	data, err := GetWeather(cfg)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if data.Lat != 123 {
		t.Errorf("Expected to get cached data")
	}

	// Expire cache
	weatherCache.mutex.Lock()
	weatherCache.lastFetched = time.Now().Add(-15 * time.Minute)
	weatherCache.mutex.Unlock()

	// Mock GetCoordinates to check if it's called
	getCoordinatesCalled := false
	originalGetCoordinates := GetCoordinates
	GetCoordinates = func(
		cityName string, apiKey string,
	) (float64, float64, error) {
		getCoordinatesCalled = true
		return 1, 2, nil
	}
	defer func() { GetCoordinates = originalGetCoordinates }()

	// This should trigger a fetch
	_, _ = GetWeather(cfg)

	if !getCoordinatesCalled {
		t.Error("Expected GetCoordinates to be called when cache is expired")
	}
}

func TestGetWeather_E2E(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		mockGeoResponse := `[{"lat": 1.23, "lon": 4.56}]`
		mockWeatherResponse := `{"lat": 1.23, "lon": 4.56, ` +
			`"current": {"temp": 70}, ` +
			`"daily": [{"weather": [{"icon": "01d"}]}]}`

		server := httptest.NewServer(
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if strings.Contains(r.URL.Path, "/direct") {
					w.WriteHeader(http.StatusOK)
					w.Write([]byte(mockGeoResponse))
				} else if strings.Contains(r.URL.Path, "/onecall") {
					w.WriteHeader(http.StatusOK)
					w.Write([]byte(mockWeatherResponse))
				}
			}),
		)
		defer server.Close()

		originalGeoURL, originalWeatherURL := geocodingAPIURL, openWeatherMapAPIURL
		geocodingAPIURL, openWeatherMapAPIURL = server.URL, server.URL
		defer func() {
			geocodingAPIURL = originalGeoURL
			openWeatherMapAPIURL = originalWeatherURL
		}()

		weatherCache.mutex.Lock()
		weatherCache.data = nil
		weatherCache.mutex.Unlock()

		cfg := &config.Config{
			OpenWeather: config.OpenWeatherConfig{
				Location: "Test", APIKey: "key",
			},
		}
		originalDownload := downloadImageFunc
		var downloaded bool
		downloadImageFunc = func(url, path string) error {
			downloaded = true
			return nil
		}
		defer func() { downloadImageFunc = originalDownload }()

		data, err := GetWeather(cfg)
		if err != nil {
			t.Fatalf("E2E Success: Expected no error, but got: %v", err)
		}
		if data.Current.Temp != 70 {
			t.Errorf("E2E Success: Expected temp 70, got %f", data.Current.Temp)
		}
		if !downloaded {
			t.Error("E2E Success: Expected downloadImageFunc to be called")
		}
	})

	t.Run("Geocoding Fails", func(t *testing.T) {
		server := httptest.NewServer(
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
			}),
		)
		defer server.Close()

		originalGeoURL := geocodingAPIURL
		geocodingAPIURL = server.URL
		defer func() { geocodingAPIURL = originalGeoURL }()

		weatherCache.mutex.Lock()
		weatherCache.data = nil
		weatherCache.mutex.Unlock()

		cfg := &config.Config{
			OpenWeather: config.OpenWeatherConfig{
				Location: "Test", APIKey: "key",
			},
		}
		_, err := GetWeather(cfg)
		if err == nil {
			t.Fatal("E2E Geocoding Fails: Expected an error, but got nil")
		}
	})

	t.Run("Weather API Fails", func(t *testing.T) {
		server := httptest.NewServer(
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if strings.Contains(r.URL.Path, "/direct") {
					w.WriteHeader(http.StatusOK)
					w.Write([]byte(`[{"lat": 1.23, "lon": 4.56}]`))
				} else if strings.Contains(r.URL.Path, "/onecall") {
					w.WriteHeader(http.StatusInternalServerError)
				}
			}),
		)
		defer server.Close()

		originalGeoURL, originalWeatherURL := geocodingAPIURL, openWeatherMapAPIURL
		geocodingAPIURL, openWeatherMapAPIURL = server.URL, server.URL
		defer func() {
			geocodingAPIURL = originalGeoURL
			openWeatherMapAPIURL = originalWeatherURL
		}()

		weatherCache.mutex.Lock()
		weatherCache.data = nil
		weatherCache.mutex.Unlock()

		cfg := &config.Config{
			OpenWeather: config.OpenWeatherConfig{
				Location: "Test", APIKey: "key",
			},
		}
		_, err := GetWeather(cfg)
		if err == nil {
			t.Fatal("E2E Weather API Fails: Expected an error, but got nil")
		}
	})

	t.Run("Invalid Weather JSON", func(t *testing.T) {
		server := httptest.NewServer(
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if strings.Contains(r.URL.Path, "/direct") {
					w.WriteHeader(http.StatusOK)
					w.Write([]byte(`[{"lat": 1.23, "lon": 4.56}]`))
				} else if strings.Contains(r.URL.Path, "/onecall") {
					w.WriteHeader(http.StatusOK)
					w.Write([]byte(`{"invalid-json`))
				}
			}),
		)
		defer server.Close()

		originalGeoURL, originalWeatherURL := geocodingAPIURL, openWeatherMapAPIURL
		geocodingAPIURL, openWeatherMapAPIURL = server.URL, server.URL
		defer func() {
			geocodingAPIURL = originalGeoURL
			openWeatherMapAPIURL = originalWeatherURL
		}()

		weatherCache.mutex.Lock()
		weatherCache.data = nil
		weatherCache.mutex.Unlock()

		cfg := &config.Config{
			OpenWeather: config.OpenWeatherConfig{
				Location: "Test", APIKey: "key",
			},
		}
		_, err := GetWeather(cfg)
		if err == nil {
			t.Fatal("E2E Invalid Weather JSON: Expected error, but got nil")
		}
	})
}
