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

package weather

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"github.com/liquidgecka/homehub/config"
)

func TestCreateView(t *testing.T) {
	// Test case with mock weather data
	t.Run("With WeatherData", func(t *testing.T) {
		// Mock the GetWeather function
		originalGetWeather := GetWeather
		defer func() { GetWeather = originalGetWeather }()
		GetWeather = func(cfg *config.Config) (*OpenWeather, error) {
			return &OpenWeather{
				Current: CurrentWeather{
					Temp:      75,
					FeelsLike: 78,
					Humidity:  60,
					WindSpeed: 10,
					Weather:   []WeatherDescription{{Main: "Clear", Description: "clear sky", Icon: "01d"}},
				},
				Daily: []DailyWeather{
					{Dt: time.Now().Unix(), Temp: DailyTemp{Max: 80, Min: 60}, Weather: []WeatherDescription{{Description: "clear sky", Icon: "01d"}}},
					{Dt: time.Now().Add(24 * time.Hour).Unix(), Temp: DailyTemp{Max: 82, Min: 62}, Weather: []WeatherDescription{{Description: "few clouds", Icon: "02d"}}},
				},
			}, nil
		}

		cfg := &config.Config{
			App:         config.AppConfig{WeatherUnits: "imperial"},
			OpenWeather: config.OpenWeatherConfig{Location: "Testville"},
		}

		view := CreateView(cfg)
		if view == nil {
			t.Fatal("CreateView returned nil with valid weatherData")
		}
		// Basic check to ensure it's a container
		if _, ok := view.(*fyne.Container); !ok {
			t.Errorf("Expected a container for valid weatherData, got %T", view)
		}
	})
}

func TestGetWeatherIconResource(t *testing.T) {
	tempDir := t.TempDir()
	cfg := &config.Config{
		OpenWeather: config.OpenWeatherConfig{
			ImageCacheDir: tempDir,
		},
	}
	iconCode := "01d"
	iconPath := filepath.Join(tempDir, "01d@4x.png")

	// Test case: Icon exists in cache
	t.Run("Icon Exists", func(t *testing.T) {
		// Create a dummy icon file
		if err := os.WriteFile(iconPath, []byte("dummy-data"), 0644); err != nil {
			t.Fatalf("Failed to create dummy icon file: %v", err)
		}

		obj := getWeatherIconResource(cfg, iconCode, 64)
		if _, ok := obj.(*canvas.Image); !ok {
			t.Errorf("Expected a canvas.Image when icon exists, got %T", obj)
		}
	})

	// Test case: Icon does not exist in cache
	t.Run("Icon Not Exists", func(t *testing.T) {
		// Ensure the file does not exist
		os.Remove(iconPath)

		obj := getWeatherIconResource(cfg, "unknown-icon", 64)
		if _, ok := obj.(*canvas.Circle); !ok {
			t.Errorf("Expected a canvas.Circle when icon does not exist, got %T", obj)
		}
	})
}
