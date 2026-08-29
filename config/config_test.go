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

package config

import (
	"os"
	"strings"
	"testing"
)

func TestLoadConfig_Success(t *testing.T) {
	content := `
[app]
idle_timeout_minutes = 10
web_server_port = 8080
web_server_listen_address = "localhost"
weather_units = "imperial"

[local_photos]
directory = "/tmp/photos"
rotation_interval_seconds = 30

[google]
  [google.calendar]
  calendar_ids = ["cal1", "cal2"]
  calendar_refresh_minutes = 15
  time_format = "3:04 PM"

[openweathermap]
api_key = "test_owm_key"
location = "TestCity,US"
refresh_minutes = 30

[shopping]
  [[shopping.store]]
  name = "StoreA"
  icon = "iconA"

  [[shopping.store]]
  name = "StoreB"
  icon = "iconB"

[finance]
currency_unit = "USD"

[dpms]
on_periods = [["07:00", "22:00"]]
on_command = ["xset", "-display", ":0", "dpms", "force", "on"]
off_command = ["xset", "-display", ":0", "dpms", "force", "off"]

[logging]
directory = "/var/log/homehub"
rotation_interval = "20M"
retention_count = 15
filename = "custom.log"
`
	configPath, cleanup := createTempFile(t, "config_test.toml", content)
	defer cleanup()

	err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	cfg := GetConfig()

	// Validate AppConfig
	if cfg.App.IdleTimeoutMinutes != 10 ||
		cfg.App.WebServerPort != 8080 ||
		cfg.App.WebServerListenAddress != "localhost" ||
		cfg.App.WeatherUnits != "imperial" {
		t.Errorf("AppConfig mismatch: got %+v", cfg.App)
	}

	// Validate LocalPhotosConfig
	if cfg.LocalPhotos.Directory != "/tmp/photos" ||
		cfg.LocalPhotos.RotationIntervalSeconds != 30 {
		t.Errorf("LocalPhotosConfig mismatch: got %+v", cfg.LocalPhotos)
	}

	// Validate GoogleConfig
	if len(cfg.Google.Calendar.CalendarIDs) != 2 ||
		cfg.Google.Calendar.CalendarIDs[0] != "cal1" ||
		cfg.Google.Calendar.TimeFormat != "3:04 PM" {
		t.Errorf("GoogleCalendarConfig mismatch: got %+v", cfg.Google.Calendar)
	}

	// Validate OpenWeatherConfig
	if cfg.OpenWeather.APIKey != "test_owm_key" ||
		cfg.OpenWeather.Location != "TestCity,US" ||
		cfg.OpenWeather.RefreshMinutes != 30 {
		t.Errorf("OpenWeatherConfig mismatch: got %+v", cfg.OpenWeather)
	}

	// Validate ShoppingConfig
	if len(cfg.Shopping.Store) != 2 ||
		cfg.Shopping.Store[0].Name != "StoreA" ||
		cfg.Shopping.Store[1].Icon != "iconB" {
		t.Errorf("ShoppingConfig mismatch: got %+v", cfg.Shopping)
	}

	if cfg.Finance.CurrencyUnit != "USD" {
		t.Errorf(
			"FinanceConfig CurrencyUnit mismatch: got %s",
			cfg.Finance.CurrencyUnit,
		)
	}

	if len(cfg.DPMS.OnPeriods) != 1 ||
		cfg.DPMS.OnPeriods[0][0] != "07:00" ||
		cfg.DPMS.OnPeriods[0][1] != "22:00" {
		t.Errorf("DPMSConfig OnPeriods mismatch: got %+v", cfg.DPMS.OnPeriods)
	}
	if len(cfg.DPMS.OnCommand) != 6 || cfg.DPMS.OnCommand[5] != "on" {
		t.Errorf("DPMSConfig OnCommand mismatch: got %+v", cfg.DPMS.OnCommand)
	}

	// Validate LoggingConfig
	if cfg.Logging.Directory != "/var/log/homehub" ||
		cfg.Logging.RotationInterval != "20M" ||
		cfg.Logging.RetentionCount != 15 ||
		cfg.Logging.Filename != "custom.log" {
		t.Errorf("LoggingConfig mismatch: got %+v", cfg.Logging)
	}
}

func TestLoadConfig_FileNotFound(t *testing.T) {
	err := LoadConfig("non_existent_config.toml")
	if err == nil {
		t.Error("LoadConfig should have failed for a non-existent file.")
	}
	if !os.IsNotExist(err) &&
		!strings.Contains(err.Error(), "no such file or directory") {
		t.Errorf("Expected 'file not found' error, got: %v", err)
	}
}

func TestLoadConfig_InvalidTOML(t *testing.T) {
	content := `
[app
invalid = "toml"
`
	configPath, cleanup := createTempFile(t, "invalid_config.toml", content)
	defer cleanup()

	err := LoadConfig(configPath)
	if err == nil {
		t.Error("LoadConfig should have failed for invalid TOML, but it didn't.")
	}
	if err == nil || err.Error() == "" { // Check if there's any error message
		t.Error("Expected an error message for invalid TOML, got empty.")
	}
}

func TestLoadConfig_AfterLoad(t *testing.T) {
	content := `
[app]
idle_timeout_minutes = 5
web_server_listen_address = "127.0.0.1"
`
	configPath, cleanup := createTempFile(t, "simple_config.toml", content)
	defer cleanup()

	_ = LoadConfig(configPath) // Load a config
	cfg := GetConfig()

	if cfg == nil {
		t.Fatal("GetConfig returned nil after loading.")
	}
	if cfg.App.IdleTimeoutMinutes != 5 ||
		cfg.App.WebServerListenAddress != "127.0.0.1" {
		t.Errorf(
			"GetConfig returned incorrect value: got idle=%d, addr=%s",
			cfg.App.IdleTimeoutMinutes, cfg.App.WebServerListenAddress,
		)
	}
}

// Test for GetConfig without prior LoadConfig (should return zero-value struct)
func TestGetConfig_NoLoad(t *testing.T) {
	// Reset _typedCfg to ensure no previous state interferes
	cleanup := SetMockConfig(Config{})
	defer cleanup()

	cfg := GetConfig()
	if cfg == nil {
		t.Fatal("GetConfig returned nil before loading.")
	}
	// Check a default value, assuming zero-value for int is 0
	if cfg.App.IdleTimeoutMinutes != 0 ||
		cfg.App.WebServerListenAddress != "" {
		t.Errorf(
			"GetConfig returned non-zero value: got idle=%d, addr=%s",
			cfg.App.IdleTimeoutMinutes, cfg.App.WebServerListenAddress,
		)
	}
}

func TestValidateConfig_CalendarRefreshMinutesZero(t *testing.T) {
	content := `
[app]
idle_timeout_minutes = 10

[local_photos]
directory = "/tmp/photos"

[google]
  [google.calendar]
  calendar_refresh_minutes = 0

`
	configPath, cleanup := createTempFile(
		t, "config_test_cal_zero.toml", content,
	)
	defer cleanup()

	err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	err = ValidateConfig()
	if err == nil {
		t.Error(
			"Expected ValidateConfig to fail for zero refresh, got nil",
		)
	}
	expectedErr := "google.calendar.calendar_refresh_minutes cannot be 0"
	if !strings.Contains(err.Error(), expectedErr) {
		t.Errorf("Expected error about zero CalendarRefreshMinutes, got: %v", err)
	}
}

func TestValidateConfig_Success(t *testing.T) {
	cfg := DefaultConfig()
	cfg.LocalPhotos.Directory = "/tmp/photos"
	cfg.Google.Calendar.CalendarIDs = []string{"test_cal"}
	cfg.OpenWeather.APIKey = "test_key"
	cfg.Security.Camera = []CameraConfig{{Name: "cam1", Type: "frigate"}}
	SetMockConfig(cfg)

	if err := ValidateConfig(); err != nil {
		t.Errorf("ValidateConfig failed for a valid config: %v", err)
	}
}

func TestValidateConfig_Failures(t *testing.T) {
	tests := []struct {
		name          string
		mutator       func(*Config)
		expectedError string
	}{
		{
			name:          "IdleTimeoutMinutes is zero",
			mutator:       func(c *Config) { c.App.IdleTimeoutMinutes = 0 },
			expectedError: "app.idle_timeout_minutes cannot be 0",
		},
		{
			name:          "LocalPhotos.Directory is empty",
			mutator:       func(c *Config) { c.LocalPhotos.Directory = "" },
			expectedError: "local_photos.directory must be configured",
		},
		{
			name: "CalendarIDs is empty",
			mutator: func(c *Config) {
				c.Google.Calendar.CalendarIDs = []string{}
			},
			expectedError: "google.calendar.calendar_ids must be specified",
		},
		{
			name: "OpenWeather.APIKey is placeholder",
			mutator: func(c *Config) {
				c.OpenWeather.APIKey = "YOUR_OPENWEATHERMAP_API_KEY"
			},
			expectedError: "OpenWeatherMap API Key is not configured",
		},
		{
			name: "OpenWeather.APIKey is empty",
			mutator: func(c *Config) {
				c.OpenWeather.APIKey = ""
			},
			expectedError: "OpenWeatherMap API Key is not configured",
		},
		{
			name: "Camera.Type is empty",
			mutator: func(c *Config) {
				c.Security.Camera = []CameraConfig{{Name: "cam1", Type: ""}}
			},
			expectedError: "camera 'cam1' must have a type specified",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Start with a valid config
			cfg := DefaultConfig()
			cfg.LocalPhotos.Directory = "/tmp/photos"
			cfg.Google.Calendar.CalendarIDs = []string{"test_cal"}
			cfg.OpenWeather.APIKey = "test_key"
			cfg.Security.Camera = []CameraConfig{
				{Name: "cam1", Type: "frigate"},
			}

			// Apply the mutation to make it invalid
			tt.mutator(&cfg)
			SetMockConfig(cfg)

			err := ValidateConfig()
			if err == nil {
				t.Fatalf("Expected ValidateConfig to fail, but it passed")
			}
			if !strings.Contains(err.Error(), tt.expectedError) {
				t.Errorf(
					"Expected error containing '%s', got '%s'",
					tt.expectedError, err.Error(),
				)
			}
		})
	}
}

func TestLoadConfig_UnknownField(t *testing.T) {
	content := `
[app]
idle_timeout_minutes = 10
unknown_field = "some_value"
`
	configPath, cleanup := createTempFile(
		t, "config_test_unknown_field.toml", content,
	)
	defer cleanup()

	err := LoadConfig(configPath)
	if err == nil {
		t.Error("Expected LoadConfig to fail for unknown field, got nil")
	}
	if !strings.Contains(err.Error(), "unknown_field") {
		t.Errorf("Expected error about unknown field, got: %v", err)
	}
}

func TestSaveConfig(t *testing.T) {
	t.Run("Success In Memory", func(t *testing.T) {
		// Load an initial config.
		configPath, cleanup := createTempFile(
			t, "save_test.toml", `[app]
idle_timeout_minutes = 1`,
		)
		defer cleanup()
		if err := LoadConfig(configPath); err != nil {
			t.Fatalf("Failed to load initial config: %v", err)
		}

		// Get the config, modify it, and save it.
		cfg := GetConfig()
		cfg.App.IdleTimeoutMinutes = 99

		if err := SaveConfig(cfg); err != nil {
			t.Fatalf("SaveConfig failed: %v", err)
		}

		// Get the config again and check if updated.
		newCfg := GetConfig()
		if newCfg.App.IdleTimeoutMinutes != 99 {
			t.Errorf(
				"Expected config updated. Got %d, want 99",
				newCfg.App.IdleTimeoutMinutes,
			)
		}
	})

	t.Run("No Active Path", func(t *testing.T) {
		// Reset activeConfigPath
		activeConfigPath = ""
		cfg := DefaultConfig()
		err := SaveConfig(&cfg)
		if err == nil {
			t.Fatal("Expected error when saving with no active path")
		}
		if !strings.Contains(err.Error(), "config file path is not set") {
			t.Errorf("Expected error about config path not set, got: %v", err)
		}
	})
}

func TestGetDefaultConfigPath(t *testing.T) {
	path := GetDefaultConfigPath()
	if path == "" {
		t.Error("Expected a default path, but got an empty string")
	}
	// A simple check to ensure it's generating a reasonable-looking path.
	if !strings.Contains(path, ".local/homehub/config.toml") {
		t.Errorf(
			"Expected path to contain '.local/homehub/config.toml', got '%s'",
			path,
		)
	}
}

// createTempFile is a helper function to create a temporary file with content
// for testing.
// It returns the file path and a cleanup function to remove the file.
func createTempFile(t *testing.T, filename, content string) (string, func()) {
	t.Helper()

	tmpFile, err := os.CreateTemp("", filename)
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}

	if _, err := tmpFile.Write([]byte(content)); err != nil {
		t.Fatalf("Failed to write to temp file: %v", err)
	}
	if err := tmpFile.Close(); err != nil {
		t.Fatalf("Failed to close temp file: %v", err)
	}

	return tmpFile.Name(), func() { os.Remove(tmpFile.Name()) }
}
